// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package recorder runs the Karta end-to-end suite against a real cluster provisioned by hack/e2e/up.sh.
// It drives each workload and records the CRs it moves through under ../recordings, which the offline
// tests replay. It judges each state from the workload's own fields and never runs Karta, so the recorder
// stays decoupled from the library it feeds.
package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/cases"
)

var (
	ctx           = context.Background()
	k8sClient     client.Client
	dynClient     dynamic.Interface
	serverVersion string // the cluster's Kubernetes version, set in BeforeSuite
)

// e2eRoot points from this package's dir (test/e2e/recorder, the go test working dir) to the module root
// test/e2e, which the case KartaFile/WorkloadFile paths and the recordings dir are relative to.
const e2eRoot = ".."

// observeTransitions watches one workload and records every distinct CR until the flow finishes, walking
// the journey's checkpoints (see record).
func observeTransitions(wc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured) (*recording, bool) {
	// One deadline for the whole flow, so a stuck RPC fails at the flow timeout, not the suite budget.
	flowCtx, cancel := context.WithTimeout(ctx, wc.Timeout)
	defer cancel()

	watcher := watchWorkload(flowCtx, obj)
	defer watcher.Stop()

	return record(flowCtx, wc, fl, obj, watcher)
}

func watchWorkload(ctx context.Context, obj *unstructured.Unstructured) watch.Interface {
	gvk := obj.GroupVersionKind()
	mapping, err := k8sClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	Expect(err).NotTo(HaveOccurred(), "rest mapping for %s", gvk)

	namespace := obj.GetNamespace()
	Expect(namespace).NotTo(BeEmpty(), "workload has no namespace")

	initialRV := obj.GetResourceVersion()
	if initialRV == "" { // Create was a no-op (object already existed); fetch a current RV to start from
		seed := GVKOnly(obj)
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

// record watches the workload and returns the recording plus whether the flow reached its terminal. It
// walks the journey's checkpoints - the steps that carry an action or an action predicate - popping the
// next one once the workload reaches that step's state and its ActionPredicate holds (a nil predicate
// matches on state alone), firing the step's action then. It keeps every distinct CR, advancing only once
// the status settles; plain states between checkpoints are recorded as they pass. On timeout or a closed
// watch it stashes the reason and returns false, so the run is still recorded.
func record(ctx context.Context, wc cases.WorkloadCase, fl cases.Flow, obj *unstructured.Unstructured, watcher watch.Interface) (*recording, bool) {
	rec := &recording{}
	pending := checkpoints(fl.Journey)
	done := func(state kartav1alpha1.ResourceStatus) bool {
		return state == fl.DesiredFinalStatus() && len(pending) == 0
	}

	// A workload already at its terminal state when Create returns never fires a watch event (the watch
	// replays only newer resourceVersions), so take that first snapshot from the create response.
	if state := cases.Classify(obj, wc.States); statusSettled(obj) && done(state) {
		rec.keep(obj, state)
		return rec, true
	}

	var lastSeen *unstructured.Unstructured
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			rec.failure = fmt.Sprintf("did not reach %q within %s; observed %v\nlast-seen status:\n%s",
				fl.DesiredFinalStatus(), wc.Timeout, rec.order, dumpStatus(lastSeen))
			return rec, false
		case event, open := <-watcher.ResultChan():
			if !open {
				rec.failure = fmt.Sprintf("watch closed before reaching %q; observed %v; last watch error: %v\nlast-seen status:\n%s",
					fl.DesiredFinalStatus(), rec.order, lastErr, dumpStatus(lastSeen))
				return rec, false
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
			if state == "" && statusSettled(u) {
				// A settled CR we cannot classify is a real gap (a state missing from both the case and
				// Karta): record it as Undefined so the order check fails and the run is saved for triage,
				// rather than skipping it silently.
				state = kartav1alpha1.UndefinedStatus
			}
			if state == "" {
				continue // half-written status not in any recognized state yet; wait for the next event
			}
			rec.keep(u, state)
			if !statusSettled(u) {
				continue // do not advance a phase on a half-written status
			}
			// Pop the next checkpoint once its state and predicate hold, firing its action.
			if len(pending) > 0 && state == pending[0].State &&
				(pending[0].ActionPredicate == nil || pending[0].ActionPredicate(u)) {
				if pending[0].Action != nil {
					rec.attachAction(fireAction(ctx, obj, pending[0].Action))
				}
				pending = pending[1:]
			}
			if done(state) {
				return rec, true
			}
		}
	}
}

// fireAction sends an action's merge-patch to the workload and returns the recorded request and response.
func fireAction(ctx context.Context, obj *unstructured.Unstructured, action *cases.Action) *Action {
	target := GVKOnly(obj)
	target.SetName(obj.GetName())
	target.SetNamespace(obj.GetNamespace())
	Expect(k8sClient.Patch(ctx, target, client.RawPatch(types.MergePatchType, action.Patch))).
		To(Succeed(), "action %s on %s/%s", action.Type, obj.GetNamespace(), obj.GetName())

	var request map[string]interface{}
	Expect(json.Unmarshal(action.Patch, &request)).To(Succeed())
	return &Action{Type: action.Type, Request: request, Response: significantCR(target)}
}

// checkpoints are the steps the recorder must reach and fire in order: those carrying an action or an
// action predicate. Plain states between them are recorded as they pass but are not stops.
func checkpoints(journey []cases.Step) []cases.Step {
	var out []cases.Step
	for _, st := range journey {
		if st.Action != nil || st.ActionPredicate != nil {
			out = append(out, st)
		}
	}
	return out
}

// recording is one watched run: the distinct CRs a workload moved through, their states, and any actions.
type recording struct {
	order     []kartav1alpha1.ResourceStatus
	snapshots []capture
	lastSig   map[string]any
	failure   string // why the flow did not reach its terminal, if it did not
}

type capture struct {
	state  kartav1alpha1.ResourceStatus
	raw    *unstructured.Unstructured
	action *Action
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

// attachAction records the action fired at the current step (the last CR kept).
func (r *recording) attachAction(a *Action) {
	if len(r.snapshots) > 0 {
		r.snapshots[len(r.snapshots)-1].action = a
	}
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

// GVKOnly is a GVK-only object, so a merge-patch never sends back a stale spec or status.
func GVKOnly(src *unstructured.Unstructured) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(src.GroupVersionKind())
	return u
}

func journeySteps(steps []cases.Step) []JourneyStep {
	out := make([]JourneyStep, len(steps))
	for i, st := range steps {
		out[i] = JourneyStep{State: st.State, Optional: st.Optional}
	}
	return out
}

// observedOrderErr runs the recorder's observed states through the same check the offline tests use.
func observedOrderErr(fl cases.Flow, order []kartav1alpha1.ResourceStatus) error {
	return ObservedOrderErr(journeySteps(fl.Journey), order, fl.DesiredFinalStatus())
}

// writeRecording writes the run as one <flow>.yaml under recorded_data/: whether it succeeded, and each
// step's own-fields state, its CR (full on the first step, a merge-patch after), and any action fired.
func writeRecording(wc cases.WorkloadCase, fl cases.Flow, rec *recording, succeeded bool) {
	version := operatorVersion(wc.Operator)
	rc := Recording{
		SchemaVersion: SchemaVersion,
		Operator:      wc.Operator,
		Version:       version,
		KartaName:     wc.KartaName,
		Flow:          fl.Name,
		Want:          string(fl.DesiredFinalStatus()),
		Succeeded:     succeeded,
		KartaFile:     strings.TrimPrefix(wc.KartaFile, "../../"),
	}
	var prevCR map[string]any
	for i, snap := range rec.snapshots {
		cur := significantCR(snap.raw)
		st := Step{State: string(snap.state), Action: snap.action}
		if i == 0 {
			st.CR = cur
		} else {
			st.Patch = MergePatch(prevCR, cur)
		}
		rc.Steps = append(rc.Steps, st)
		prevCR = cur
	}

	path := RecordingPath(filepath.Join(e2eRoot, "recorded_data"), rc)
	Expect(WriteRecording(path, rc)).To(Succeed())
	GinkgoWriter.Printf("recorded %s/%s/%s/%s.yaml (%d steps %v)\n", wc.Operator, version, wc.KartaName, fl.Name, len(rc.Steps), rec.order)
}

// operatorVersion is the installed version of op, or the cluster's Kubernetes version for built-in
// workloads no operator provides (batch-job, deployment, ...).
func operatorVersion(op string) string {
	b, err := os.ReadFile(filepath.Join(e2eRoot, "..", "..", "hack", "e2e", "operators", ".installed-versions"))
	if err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == op {
				return strings.TrimSpace(v)
			}
		}
	}
	return serverVersion
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
