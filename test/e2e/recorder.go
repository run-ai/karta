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

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/conformance"
	"github.com/run-ai/karta/test/e2e/cases"
)

var (
	ctx       = context.Background()
	k8sClient client.Client
	dynClient dynamic.Interface
)

// observeTransitions watches one workload from creation and records every distinct settled CR
// it moves through, firing each journey action once when its state is reached, until the
// terminal state is reached with all actions fired (or the timeout).
func observeTransitions(tc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured, timeout time.Duration) *recording {
	gvk := obj.GroupVersionKind()
	mapping, err := k8sClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	Expect(err).NotTo(HaveOccurred(), "rest mapping for %s", gvk)

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	// Watch from the workload's creation resourceVersion so no early state is missed.
	initialRV := obj.GetResourceVersion()
	if initialRV == "" {
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
	defer watcher.Stop()

	rec := &recording{}

	// A journey that gates any step on the workload's own fields (a scale flow, where the state stays
	// Running while the replica count changes) is driven strictly by position: one CR captured per
	// step, so a repeated state is recorded each time. The default path below dedups on the state.
	if journeyGated(fl.Journey) {
		observeGated(ctx, tc, fl, obj, watcher, timeout, rec)
		return rec
	}

	var pending []cases.Step // journey steps with an action, fired head-first in order
	for _, st := range fl.Journey {
		if st.Action != nil {
			pending = append(pending, st)
		}
	}
	done := func(state kartav1alpha1.ResourceStatus) bool { return state == fl.Want() && len(pending) == 0 }
	var lastSeen *unstructured.Unstructured

	// A workload terminal at creation never fires a watch event; take the create response.
	if statusSettled(obj) && done(cases.Classify(obj, tc.States)) {
		rec.keep(obj, fl.Want())
		return rec
	}

	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			Fail(fmt.Sprintf("workload %s flow %q did not finish within %s; observed %v, want %q, unfired %v\nlast-seen status:\n%s",
				tc.Name, fl.Name, timeout, rec.order, fl.Want(), stepStates(pending), dumpStatus(lastSeen)))
		case event, open := <-watcher.ResultChan():
			if !open {
				Fail("watch closed before terminal state")
			}
			u, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue // a bookmark or error frame carries no workload object
			}
			lastSeen = u
			state := cases.Classify(u, tc.States)
			if !statusSettled(u) || state == "" {
				continue // mid-reconcile or unrecognised: nothing to replay against
			}
			rec.keep(u, state)
			if len(pending) > 0 && state == pending[0].State {
				Expect(pending[0].Action(ctx, obj)).NotTo(HaveOccurred(), "action at state %q", state)
				pending = pending[1:]
			}
			if done(state) {
				return rec
			}
		}
	}
}

// observeGated drives a gated journey strictly by position: it walks the steps in order, and a step
// is reached only when Classify returns its state and its settle gate holds. Each reached step's CR
// is captured (even when the state repeats) and its action fires to drive toward the next step. This
// is the scale path - the workload stays Running while spec.replicas and the ready count change, and
// settle (ReplicasReady) picks out the CR settled at each count.
func observeGated(ctx context.Context, tc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured, watcher watch.Interface, timeout time.Duration, rec *recording) {
	pos := 0
	var lastSeen *unstructured.Unstructured
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			st := fl.Journey[pos]
			Fail(fmt.Sprintf("workload %s flow %q stuck at step %d (want %q + settle); recorded %v\nlast-seen status:\n%s",
				tc.Name, fl.Name, pos, st.State, rec.order, dumpStatus(lastSeen)))
		case event, open := <-watcher.ResultChan():
			if !open {
				Fail("watch closed before the journey finished")
			}
			u, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			lastSeen = u
			state := cases.Classify(u, tc.States)
			if !statusSettled(u) || state == "" {
				continue
			}
			st := fl.Journey[pos]
			if state != st.State || (st.Settle != nil && !st.Settle(u)) {
				continue
			}
			rec.keepForced(u, state)
			if st.Action != nil {
				Expect(st.Action(ctx, obj)).NotTo(HaveOccurred(), "action at step %d state %q", pos, state)
			}
			pos++
			if pos == len(fl.Journey) {
				return
			}
		}
	}
}

// journeyGated reports whether any step carries a settle gate, so the flow needs the position path.
func journeyGated(journey []cases.Step) bool {
	for _, st := range journey {
		if st.Settle != nil {
			return true
		}
	}
	return false
}

// recording is one watched run: the CRs worth freezing and the order the states happened in. A CR
// is kept only when its classified state differs from the previous kept one, so the recording holds
// one CR per state change. A genuine return to an earlier state (A -> B -> A) is kept, so the order
// check can catch a backwards jump. Deduping on the state (read from the workload's own fields)
// needs no sanitize to tell a real change from resourceVersion churn.
type recording struct {
	order     []kartav1alpha1.ResourceStatus
	snapshots []capture
	lastState kartav1alpha1.ResourceStatus
	started   bool
}

type capture struct {
	state kartav1alpha1.ResourceStatus
	raw   *unstructured.Unstructured // the full CR, stored as-is; the recording is not sanitized
}

func (r *recording) keep(u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) {
	if r.started && state == r.lastState {
		return
	}
	r.keepForced(u, state)
}

// keepForced captures a CR at the given state without the state-change dedup, so a gated journey can
// record the same state more than once (a scale flow's Running before and after the scale).
func (r *recording) keepForced(u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) {
	r.started = true
	r.lastState = state
	r.snapshots = append(r.snapshots, capture{state: state, raw: u.DeepCopy()})
	r.order = append(r.order, state)
}

func stepStates(sts []cases.Step) []kartav1alpha1.ResourceStatus {
	out := make([]kartav1alpha1.ResourceStatus, len(sts))
	for i, st := range sts {
		out[i] = st.State
	}
	return out
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

// observedOrderErr checks the observed states are an in-order subsequence of the journey, ending on
// the terminal. A fast workload may skip a declared step (subsequence, not equality), but a state out
// of order or not declared at all is a regression. A workload that genuinely revisits a state - a Job
// or JobSet reads active-not-ready again as its pod terminates, so Running dips back to Initializing -
// declares that revisit as its own step in the journey, so the dip is stated, never waived.
func observedOrderErr(fl cases.Flow, order []kartav1alpha1.ResourceStatus) error {
	seq := slices.Compact(slices.Clone(order))
	if len(seq) == 0 {
		return fmt.Errorf("no states observed")
	}
	declared := stepStates(fl.Journey)
	j := 0
	for _, s := range seq {
		for j < len(declared) && declared[j] != s {
			j++
		}
		if j == len(declared) {
			if slices.Contains(declared, s) {
				return fmt.Errorf("state %q observed out of journey %v", s, declared)
			}
			return fmt.Errorf("observed undeclared state %q; journey is %v", s, declared)
		}
		j++
	}
	if last := seq[len(seq)-1]; last != fl.Want() {
		return fmt.Errorf("terminal must be %q, observed %q", fl.Want(), last)
	}
	return nil
}

// writeFixture builds a Recording from the run and writes it as one <flow>.yaml under
// test/conformance/fixtures/. For each observed state it stores the workload's own-fields state, the
// CR, and what Karta read of it; the CR and the reading are both a full value on the first step and a
// merge-patch from the previous step on every later step, so the file holds only what changed. No
// sanitize - the offline golden rebuilds the exact CR, so Karta reads the exact same bytes back.
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
	var prevCR, prevExp map[string]interface{}
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
