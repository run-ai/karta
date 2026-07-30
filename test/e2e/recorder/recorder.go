// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package recorder runs the Karta end-to-end suite against a real cluster provisioned by hack/e2e/up.sh.
// It drives each workload and records what it does under ../conformance/fixtures, which the offline
// golden there replays. It lives in the test/e2e module so the cluster deps stay out of the library.
package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/run-ai/karta/test/e2e/cases"
	"github.com/run-ai/karta/test/e2e/conformance"
)

var (
	ctx       = context.Background()
	k8sClient client.Client
	dynClient dynamic.Interface
)

// e2eRoot points from this package's dir (test/e2e/recorder, the go test working dir) to the module root
// test/e2e, which the case KartaFile/WorkloadFile paths and the fixtures dir are relative to.
const e2eRoot = ".."

// observeTransitions watches one workload and records every distinct CR until the flow finishes. A scale
// flow (state stays Running while the replica count changes) uses driveByPosition, others driveByState.
func observeTransitions(wc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured, karta *kartav1alpha1.Karta) *recording {
	// One deadline for the whole flow, so a stuck RPC fails at the flow timeout, not the suite budget.
	flowCtx, cancel := context.WithTimeout(ctx, wc.Timeout)
	defer cancel()

	watcher := watchWorkload(flowCtx, obj)
	defer watcher.Stop()

	rec := &recording{}
	if journeyGated(fl.Journey) {
		driveByPosition(flowCtx, wc, fl, obj, karta, watcher, rec)
	} else {
		driveByState(flowCtx, wc, fl, obj, karta, watcher, rec)
	}
	return rec
}

func watchWorkload(ctx context.Context, obj *unstructured.Unstructured) watch.Interface {
	gvk := obj.GroupVersionKind()
	mapping, err := k8sClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	Expect(err).NotTo(HaveOccurred(), "rest mapping for %s", gvk)

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	initialRV := obj.GetResourceVersion()
	if initialRV == "" { // Create was a no-op (object already existed); fetch a current RV to start from
		seed := cases.GVKOnly(obj)
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

func driveByState(ctx context.Context, wc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured, karta *kartav1alpha1.Karta, watcher watch.Interface, rec *recording) {
	pending := actionSteps(fl.Journey)
	done := func(state kartav1alpha1.ResourceStatus) bool {
		return state == fl.DesiredFinalStatus() && len(pending) == 0
	}

	// A workload already at its terminal state when Create returns never fires a watch event (the watch
	// replays only newer resourceVersions), so take that first snapshot from the create response.
	if state := cases.Classify(obj, wc.States); statusSettled(obj) && done(state) {
		rec.keep(obj, state)
		return
	}

	recordUntil(ctx, wc, fl, karta, watcher, rec, func(_ *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) bool {
		if len(pending) > 0 && state == pending[0].State {
			Expect(pending[0].Action(ctx, obj)).NotTo(HaveOccurred(), "action at state %q", state)
			pending = pending[1:]
		}
		return done(state)
	})
}

// driveByPosition walks a gated journey step by step: a step is reached only when its state matches and
// its settle gate holds. The scale path - the gate (a replica count), not the state, is what advances.
func driveByPosition(ctx context.Context, wc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured, karta *kartav1alpha1.Karta, watcher watch.Interface, rec *recording) {
	pos := 0
	recordUntil(ctx, wc, fl, karta, watcher, rec, func(u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) bool {
		// Skip Optional steps: they are order-check-only dips, not drive stops.
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

type advanceFunc func(u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) bool

// recordUntil is the watch loop: keep every distinct CR, but call advance only once the status settles.
func recordUntil(ctx context.Context, wc cases.WorkloadCase, fl cases.Flow, karta *kartav1alpha1.Karta, watcher watch.Interface, rec *recording, advance advanceFunc) {
	var lastSeen *unstructured.Unstructured
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			Fail(fmt.Sprintf("workload %s flow %q did not reach %q within %s; recorded %v\nlast-seen status:\n%s",
				wc.Name, fl.Name, fl.DesiredFinalStatus(), wc.Timeout, rec.order, dumpStatus(lastSeen)))
		case event, open := <-watcher.ResultChan():
			if !open {
				Fail(fmt.Sprintf("%s flow %q: watch closed before reaching %q; recorded %v; last watch error: %v\nlast-seen status:\n%s",
					wc.Name, fl.Name, fl.DesiredFinalStatus(), rec.order, lastErr, dumpStatus(lastSeen)))
			}
			if event.Type == watch.Error {
				lastErr = apierrors.FromObject(event.Object)
				continue
			}
			u, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue // a bookmark carries no workload object
			}
			lastSeen = u
			state := cases.Classify(u, wc.States)
			if state == "" {
				// No predicate matched. If Karta still reads a real state the registry is missing a predicate
				// (fail, else it goes unvalidated); if Karta reads nothing either, the CR is outside the case.
				if ks := kartaState(ctx, karta, u); statusSettled(u) && ks != "" {
					Fail(fmt.Sprintf("%s flow %q: Karta reads %q here but no predicate in the registry "+
						"declares it - add one, or the mapping over-reads. full CR:\n%s", wc.Name, fl.Name, ks, dumpCR(u)))
				} else if ks == "" {
					GinkgoWriter.Printf("no Karta state for %s flow %q: CR is outside the registry\n%s", wc.Name, fl.Name, dumpCR(u))
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

func actionSteps(journey []cases.Step) []cases.Step {
	var out []cases.Step
	for _, st := range journey {
		if st.Action != nil {
			out = append(out, st)
		}
	}
	return out
}

func journeyGated(journey []cases.Step) bool {
	for _, st := range journey {
		if st.Settle != nil {
			return true
		}
	}
	return false
}

// recording is one watched run: the distinct CRs a workload moved through and their states.
type recording struct {
	order     []kartav1alpha1.ResourceStatus
	snapshots []capture
	lastSig   map[string]any
}

type capture struct {
	state kartav1alpha1.ResourceStatus
	raw   *unstructured.Unstructured
}

func (r *recording) keep(u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) {
	sig := significantCR(u)
	if r.lastSig != nil && reflect.DeepEqual(r.lastSig, sig) {
		return
	}
	r.lastSig = sig
	r.snapshots = append(r.snapshots, capture{state: state, raw: u.DeepCopy()})
	r.order = append(r.order, state)
}

// significantCR drops the fields the apiserver bumps on every event (resourceVersion, managedFields) so
// keep dedups on real changes.
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

func journeySteps(steps []cases.Step) []conformance.JourneyStep {
	out := make([]conformance.JourneyStep, len(steps))
	for i, st := range steps {
		out[i] = conformance.JourneyStep{State: st.State, Optional: st.Optional}
	}
	return out
}

// observedOrderErr runs the recorder's observed states through the same check the offline golden uses.
func observedOrderErr(fl cases.Flow, order []kartav1alpha1.ResourceStatus) error {
	return conformance.ObservedOrderErr(journeySteps(fl.Journey), order, fl.DesiredFinalStatus())
}

// writeFixture writes the run as one <flow>.yaml under conformance/fixtures/: each step's own-fields state,
// CR, and Karta reading, full on the first step and a merge-patch from the previous after.
func writeFixture(wc cases.WorkloadCase, fl cases.Flow, rec *recording, karta *kartav1alpha1.Karta) {
	version := operatorVersion(wc.Operator)
	rc := conformance.Recording{
		SchemaVersion: conformance.SchemaVersion,
		Operator:      wc.Operator,
		Version:       version,
		KartaName:     wc.KartaName,
		Flow:          fl.Name,
		Want:          string(fl.DesiredFinalStatus()),
		KartaFile:     strings.TrimPrefix(wc.KartaFile, "../../"),
	}
	var prevCR, prevExp map[string]any
	for i, snap := range rec.snapshots {
		cur := snap.raw.Object
		exp, err := conformance.Reading(ctx, karta, snap.raw)
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

	path := conformance.RecordingPath(filepath.Join(e2eRoot, "conformance", "fixtures"), rc)
	Expect(conformance.WriteRecording(path, rc)).To(Succeed())
	GinkgoWriter.Printf("recorded %s/%s/%s/%s.yaml (%d steps %v)\n", wc.Operator, version, wc.KartaName, fl.Name, len(rc.Steps), rec.order)
}

func actionName(fl cases.Flow, state kartav1alpha1.ResourceStatus) string {
	for _, st := range fl.Journey {
		if st.State == state && st.Action != nil {
			// ScaleParallelism returns a closure named ...funcN; walk to the constructor name, not func8.
			full := runtime.FuncForPC(reflect.ValueOf(st.Action).Pointer()).Name()
			for {
				i := strings.LastIndexByte(full, '.')
				base := full[i+1:]
				if i < 0 || len(base) <= 4 || base[:4] != "func" || strings.Trim(base[4:], "0123456789") != "" {
					return base
				}
				full = full[:i]
			}
		}
	}
	return ""
}

func operatorVersion(op string) string {
	b, err := os.ReadFile(filepath.Join(e2eRoot, "..", "..", "hack", "e2e", "operators", ".installed-versions"))
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

// kartaState returns the state Karta reads for this CR, or "" if it matches nothing (Undefined counts
// as no match).
func kartaState(ctx context.Context, karta *kartav1alpha1.Karta, u *unstructured.Unstructured) string {
	reading, err := conformance.Reading(ctx, karta, u)
	if err != nil {
		GinkgoWriter.Printf("kartaState: Karta read failed, undeclared-state guard may under-report: %v\n", err)
		return ""
	}
	matched, _ := reading["matchedStatuses"].([]any)
	for _, s := range matched {
		if str, ok := s.(string); ok && str != string(kartav1alpha1.UndefinedStatus) {
			return str
		}
	}
	return ""
}

func dumpCR(u *unstructured.Unstructured) string {
	c := u.DeepCopy()
	unstructured.RemoveNestedField(c.Object, "metadata", "managedFields")
	b, err := yaml.Marshal(c.Object)
	if err != nil {
		return fmt.Sprintf("(cr marshal error: %v)", err)
	}
	return string(b)
}

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
