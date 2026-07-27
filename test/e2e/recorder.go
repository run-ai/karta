// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package e2e runs the Karta end-to-end suite against a real cluster provisioned by
// hack/e2e/up.sh. It is its own Go module so the cluster deps stay out of the library.
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/conformance"
	"github.com/run-ai/karta/test/e2e/cases"
)

var (
	ctx       = context.Background()
	k8sClient client.Client
	dynClient dynamic.Interface
)

// observeTransitions watches one workload from creation and records every distinct CR it moves through
// until the flow finishes (or the timeout). An ordinary flow uses driveByState; a scale flow, whose
// state stays Running while the replica count changes, uses driveByPosition.
func observeTransitions(tc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured, karta *kartav1alpha1.Karta, timeout time.Duration) *recording {
	watcher := watchWorkload(obj)
	defer watcher.Stop()

	rec := &recording{}
	if journeyGated(fl.Journey) {
		driveByPosition(tc, fl, obj, karta, watcher, timeout, rec)
	} else {
		driveByState(tc, fl, obj, karta, watcher, timeout, rec)
	}
	return rec
}

func watchWorkload(obj *unstructured.Unstructured) watch.Interface {
	gvk := obj.GroupVersionKind()
	mapping, err := k8sClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	Expect(err).NotTo(HaveOccurred(), "rest mapping for %s", gvk)

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	initialRV := obj.GetResourceVersion()
	if initialRV == "" { // Create was a no-op (object already existed); fetch a current RV to start from
		seed := cases.EmptyLike(obj)
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), seed)).To(Succeed())
		initialRV = seed.GetResourceVersion()
	}

	watcher, err := watchtools.NewRetryWatcherWithContext(ctx, initialRV, &cache.ListWatch{
		WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", obj.GetName()).String()
			return dynClient.Resource(mapping.Resource).Namespace(namespace).Watch(ctx, opts)
		},
	})
	Expect(err).NotTo(HaveOccurred())
	return watcher
}

// driveByState follows the states as they appear, firing each journey action when its state is reached,
// until the workload is at the terminal state with every action fired.
func driveByState(tc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured, karta *kartav1alpha1.Karta, watcher watch.Interface, timeout time.Duration, rec *recording) {
	pending := actionSteps(fl.Journey)
	done := func(state kartav1alpha1.ResourceStatus) bool { return state == fl.Want() && len(pending) == 0 }

	// A workload already at its terminal state when Create returns never fires a watch event (the watch
	// replays only newer resourceVersions), so take that first snapshot from the create response.
	if state := cases.Classify(obj, tc.States); statusSettled(obj) && done(state) {
		rec.keep(obj, state)
		return
	}

	recordUntil(tc, fl, karta, watcher, timeout, rec, func(_ *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) bool {
		if len(pending) > 0 && state == pending[0].State {
			Expect(pending[0].Action(ctx, obj)).NotTo(HaveOccurred(), "action at state %q", state)
			pending = pending[1:]
		}
		return done(state)
	})
}

// driveByPosition walks a gated journey step by step: a step is reached only when its state matches and
// its settle gate holds. The scale path - the gate (a replica count), not the state, is what advances.
func driveByPosition(tc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured, karta *kartav1alpha1.Karta, watcher watch.Interface, timeout time.Duration, rec *recording) {
	pos := 0
	recordUntil(tc, fl, karta, watcher, timeout, rec, func(u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) bool {
		// An Optional step is a transient scale dip declared for the order check only, not a drive stop -
		// skip past it. driveByPosition waits only at the real steps.
		for pos < len(fl.Journey) && fl.Journey[pos].Optional {
			pos++
		}
		if pos == len(fl.Journey) {
			return true
		}
		step := fl.Journey[pos]
		if state != step.State || (step.Settle != nil && !step.Settle(u)) {
			return false
		}
		if step.Action != nil {
			Expect(step.Action(ctx, obj)).NotTo(HaveOccurred(), "action at step %d state %q", pos, state)
		}
		pos++
		return pos == len(fl.Journey)
	})
}

// advanceFunc handles one settled event: it fires any action due now and returns whether the flow has
// finished. u is the observed CR (for settle gates); actions patch the workload the closure captured.
type advanceFunc func(u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) bool

// recordUntil is the shared watch loop: it keeps every distinct CR (even mid-reconcile) and, once the
// status has settled, calls advance to drive the journey. Recording is unconditional; advancing waits
// for settle. It returns when advance reports done, and fails on the deadline.
func recordUntil(tc cases.WorkloadCase, fl cases.Flow, karta *kartav1alpha1.Karta, watcher watch.Interface, timeout time.Duration, rec *recording, advance advanceFunc) {
	deadline := time.After(timeout)
	var lastSeen *unstructured.Unstructured
	for {
		select {
		case <-deadline:
			Fail(fmt.Sprintf("workload %s flow %q did not reach %q within %s; recorded %v\nlast-seen status:\n%s",
				tc.Name, fl.Name, fl.Want(), timeout, rec.order, dumpStatus(lastSeen)))
		case event, open := <-watcher.ResultChan():
			if !open {
				Fail("watch closed before the flow finished")
			}
			u, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue // a bookmark or error frame carries no workload object
			}
			lastSeen = u
			state := cases.Classify(u, tc.States)
			if state == "" {
				// No predicate matched. If Karta still reads a real state, the registry is missing a
				// predicate - fail with the CR, else that reading goes unvalidated. If Karta reads nothing
				// either (Undefined), the CR is outside the definition; nothing to record.
				if ks := kartaState(karta, u); statusSettled(u) && ks != "" {
					Fail(fmt.Sprintf("%s flow %q: Karta reads %q here but no predicate in the registry "+
						"declares it - add one, or the mapping over-reads. full CR:\n%s", tc.Name, fl.Name, ks, dumpCR(u)))
				}
				continue
			}
			rec.keep(u, state)
			if !statusSettled(u) {
				continue // do not advance a phase on a half-written status
			}
			if advance(u, state) {
				return
			}
		}
	}
}

// actionSteps returns the journey steps that carry an action, in order; driveByState fires them
// head-first as their state appears.
func actionSteps(journey []cases.Step) []cases.Step {
	var out []cases.Step
	for _, st := range journey {
		if st.Action != nil {
			out = append(out, st)
		}
	}
	return out
}

// journeyGated reports whether any step carries a settle gate, so the flow needs driveByPosition.
func journeyGated(journey []cases.Step) bool {
	for _, st := range journey {
		if st.Settle != nil {
			return true
		}
	}
	return false
}

// recording is one watched run: every distinct CR the workload moved through and the order of states.
// keep dedups on the CR bytes, so an intermediate CR at the same state (a scale step) is captured too.
type recording struct {
	order     []kartav1alpha1.ResourceStatus // the state of each snapshot, flat for the order check
	snapshots []capture
	lastRaw   *unstructured.Unstructured
}

type capture struct {
	state kartav1alpha1.ResourceStatus
	raw   *unstructured.Unstructured
}

// keep records the CR when it differs (by sameCR) from the last one kept, storing its classified state
// alongside for the golden's anchor and the order check.
func (r *recording) keep(u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) {
	if r.lastRaw != nil && sameCR(r.lastRaw, u) {
		return
	}
	raw := u.DeepCopy()
	r.lastRaw = raw
	r.snapshots = append(r.snapshots, capture{state: state, raw: raw})
	r.order = append(r.order, state)
}

// sameCR reports whether two CRs are equal ignoring resourceVersion and managedFields, which the
// apiserver bumps on every event with no workload change. Every other field counts.
func sameCR(a, b *unstructured.Unstructured) bool {
	return reflect.DeepEqual(significantCR(a), significantCR(b))
}

func significantCR(u *unstructured.Unstructured) map[string]any {
	c := u.DeepCopy().Object
	unstructured.RemoveNestedField(c, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(c, "metadata", "managedFields")
	return c
}

// statusSettled reports whether the controller has caught up (observedGeneration >= generation);
// workloads without those fields count as settled.
func statusSettled(u *unstructured.Unstructured) bool {
	gen, hasGen, _ := unstructured.NestedInt64(u.Object, "metadata", "generation")
	obs, hasObs, _ := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
	if !hasGen || !hasObs {
		return true
	}
	return obs >= gen
}

func assertObservedOrder(fl cases.Flow, order []kartav1alpha1.ResourceStatus) {
	Expect(observedOrderErr(fl, order)).To(Succeed())
}

// stepStates is the journey's declared states, in order.
func stepStates(steps []cases.Step) []kartav1alpha1.ResourceStatus {
	out := make([]kartav1alpha1.ResourceStatus, len(steps))
	for i, st := range steps {
		out[i] = st.State
	}
	return out
}

// observedOrderErr checks the observed states are an in-order subsequence of the journey, ending on the
// terminal. A skipped step is fine; an out-of-order or undeclared state is a regression. A genuine
// revisit (a Job dips Running -> Initializing as its pod terminates) is declared as its own step.
func observedOrderErr(fl cases.Flow, order []kartav1alpha1.ResourceStatus) error {
	observed := slices.Compact(slices.Clone(order)) // collapse consecutive repeats
	if len(observed) == 0 {
		return fmt.Errorf("no states observed")
	}
	declared := stepStates(fl.Journey)

	// Walk declared forward, matching each observed state to the next declared one (subsequence check).
	di := 0
	for _, state := range observed {
		for di < len(declared) && declared[di] != state {
			di++
		}
		if di == len(declared) {
			if slices.Contains(declared, state) {
				return fmt.Errorf("state %q observed out of journey %v", state, declared)
			}
			return fmt.Errorf("observed undeclared state %q; journey is %v", state, declared)
		}
		di++
	}
	if last := observed[len(observed)-1]; last != fl.Want() {
		return fmt.Errorf("terminal must be %q, observed %q", fl.Want(), last)
	}
	return nil
}

// writeFixture writes the run as one <flow>.yaml under test/conformance/fixtures/. Each step holds the
// own-fields state, the CR, and what Karta read of it - full on the first step, a merge-patch from the
// previous step after, so the file holds only what changed.
func writeFixture(tc cases.WorkloadCase, fl cases.Flow, rec *recording, karta *kartav1alpha1.Karta) {
	version := operatorVersion(tc.Operator)
	rc := conformance.Recording{
		SchemaVersion: conformance.SchemaVersion,
		Operator:      tc.Operator,
		Version:       version,
		KartaName:     tc.KartaName,
		Flow:          fl.Name,
		Want:          string(fl.Want()),
		KartaFile:     strings.TrimPrefix(tc.KartaFile, "../../"),
	}
	var prevCR, prevExp map[string]any
	for i, snap := range rec.snapshots {
		cur := snap.raw.Object
		exp, err := conformance.Reading(karta, snap.raw)
		Expect(err).NotTo(HaveOccurred(), "read Karta at state %q", snap.state)
		st := conformance.Step{State: string(snap.state), Action: actionName(fl, snap.state)}
		if i == 0 {
			st.CR = cur
			st.Expected = exp
		} else {
			st.Patch = conformance.MergePatch(prevCR, cur)
			st.ExpectedPatch = conformance.MergePatch(prevExp, exp)
		}
		rc.Steps = append(rc.Steps, st)
		prevCR, prevExp = cur, exp
	}

	path := conformance.RecordingPath(filepath.Join("..", "conformance", "fixtures"), rc)
	Expect(conformance.WriteRecording(path, rc)).To(Succeed())
	GinkgoWriter.Printf("recorded %s/%s/%s/%s.yaml (%d steps %v)\n", tc.Operator, version, tc.KartaName, fl.Name, len(rc.Steps), rec.order)
}

// actionName is the name of the action fired when a state is reached, for recording provenance.
func actionName(fl cases.Flow, state kartav1alpha1.ResourceStatus) string {
	for _, st := range fl.Journey {
		if st.State == state && st.Action != nil {
			full := runtime.FuncForPC(reflect.ValueOf(st.Action).Pointer()).Name()
			if i := strings.LastIndexByte(full, '.'); i >= 0 {
				return full[i+1:]
			}
			return full
		}
	}
	return ""
}

// operatorVersion is the version hack/e2e/up.sh installed for op, or "unknown".
func operatorVersion(op string) string {
	b, err := os.ReadFile(filepath.Join("..", "..", "hack", "e2e", "operators", ".installed-versions"))
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == op {
			return strings.TrimSpace(v)
		}
	}
	return "unknown"
}

// kartaState returns the real state Karta reads for this CR, or "" if Karta matches nothing. Karta's
// Undefined sentinel counts as no match, the same "" our predicates return on those CRs.
func kartaState(karta *kartav1alpha1.Karta, u *unstructured.Unstructured) string {
	reading, err := conformance.Reading(karta, u)
	if err != nil {
		return ""
	}
	matched, _ := reading["matchedStatuses"].([]interface{})
	for _, s := range matched {
		if str, ok := s.(string); ok && str != string(kartav1alpha1.UndefinedStatus) {
			return str
		}
	}
	return ""
}

// dumpCR renders the whole CR (minus managedFields) for an undeclared-state failure message.
func dumpCR(u *unstructured.Unstructured) string {
	c := u.DeepCopy()
	unstructured.RemoveNestedField(c.Object, "metadata", "managedFields")
	b, err := yaml.Marshal(c.Object)
	if err != nil {
		return fmt.Sprintf("(cr marshal error: %v)", err)
	}
	return string(b)
}

// dumpStatus renders a CR's status for a timeout failure message.
func dumpStatus(u *unstructured.Unstructured) string {
	if u == nil {
		return "(no object observed)"
	}
	status, _, _ := unstructured.NestedMap(u.Object, "status")
	b, err := json.MarshalIndent(status, "  ", "  ")
	if err != nil {
		return fmt.Sprintf("(status marshal error: %v)", err)
	}
	return "  " + string(b)
}

// recordEnabled gates fixture writing to make record-e2e (KARTA_RECORD=1).
func recordEnabled() bool {
	return os.Getenv("KARTA_RECORD") == "1"
}
