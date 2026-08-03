// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package recorder drives a workload through a declared flow and records the CRs it moves through, for the
// offline tests to replay. It judges each state from the workload's own fields and never runs Karta, so it
// stays decoupled from the library it feeds. It only records: installing the Karta definition and running
// the suite belong to the caller.
package recorder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

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
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/cases"
)

// e2eRoot points from a package dir under test/e2e (the go test working dir) to test/e2e, which manifest,
// Karta, and recording paths are relative to.
const e2eRoot = ".."

// Cluster access, bound once by the suite via Bind.
var (
	k8sClient     client.Client
	dynClient     dynamic.Interface
	serverVersion string
	namespace     string
)

// Bind wires the recorder to the cluster clients, the Kubernetes version, and the throwaway namespace
// workloads are created in. The suite calls it once in BeforeSuite.
func Bind(c client.Client, d dynamic.Interface, version, ns string) {
	k8sClient, dynClient, serverVersion, namespace = c, d, version, ns
}

// Recorder records the flows of one workload type. Bind it to the type's Karta definition and state
// registry, then build and Run a Flow per case.
type Recorder struct {
	operator  string
	kartaName string
	kartaFile string
	states    []cases.NamedState
	timeout   time.Duration
}

// New starts a recorder for the given operator key, Karta definition name, and Karta YAML path. The path is
// recording metadata (so the offline golden can load the definition); New does not touch the cluster.
func New(operator, kartaName, kartaFile string) *Recorder {
	return &Recorder{operator: operator, kartaName: kartaName, kartaFile: kartaFile, timeout: 3 * time.Minute}
}

// State registers how to recognise a state from the workload's own fields; declare them least to most
// advanced (Classify keeps the furthest match).
func (r *Recorder) State(name kartav1alpha1.ResourceStatus, match cases.StateCheck) *Recorder {
	r.states = append(r.states, cases.NamedState{Name: name, Match: match})
	return r
}

// Timeout overrides the per-flow deadline (default 3m).
func (r *Recorder) Timeout(d time.Duration) *Recorder { r.timeout = d; return r }

// Flow starts a flow named name, seeded from the workload manifest (a path relative to test/e2e).
func (r *Recorder) Flow(name, manifest string) *Flow {
	return &Flow{rec: r, name: name, manifest: manifest}
}

// Flow is a declared journey: a manifest to apply, then the ordered stops the workload must reach. A stop
// carrying an action predicate and/or an action is a checkpoint the recorder drives to.
type Flow struct {
	rec      *Recorder
	name     string
	manifest string
	journey  []cases.Step
}

// Reaches adds a plain stop: the workload must classify as state here.
func (f *Flow) Reaches(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, cases.Step{State: state})
	return f
}

// Maybe adds an optional stop the workload may skip (a transient dip); the order check tolerates it.
func (f *Flow) Maybe(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, cases.Step{State: state, Optional: true})
	return f
}

// At adds a stop to be gated with When/WaitUntil and/or fired with Do.
func (f *Flow) At(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, cases.Step{State: state})
	return f
}

// When gates the current stop: it is not reached until this predicate over the workload's own fields holds.
func (f *Flow) When(gate cases.StateCheck) *Flow { f.last().ActionPredicate = gate; return f }

// WaitUntil is When for the terminal stop - the flow finishes once it holds.
func (f *Flow) WaitUntil(gate cases.StateCheck) *Flow { f.last().ActionPredicate = gate; return f }

// Do fires an action when the current stop is reached.
func (f *Flow) Do(action *cases.Action) *Flow { f.last().Action = action; return f }

func (f *Flow) last() *cases.Step { return &f.journey[len(f.journey)-1] }

// want is the flow's terminal state: the last stop's state.
func (f *Flow) want() kartav1alpha1.ResourceStatus { return f.journey[len(f.journey)-1].State }

// Run applies the manifest, drives the workload through the journey, writes the recording, and returns it.
// It returns an error if the workload did not reach its terminal state in the declared order; the recording
// is written either way (a failed run is saved with succeeded:false, for triage).
func (f *Flow) Run(ctx context.Context) (*Recording, error) {
	obj := &unstructured.Unstructured{}
	Expect(yaml.Unmarshal(mustRead(f.manifest), obj)).To(Succeed())
	obj.SetNamespace(namespace)
	Expect(k8sClient.Create(ctx, obj)).To(Succeed(), "create workload for %s", f.name)
	defer func() { _ = k8sClient.Delete(context.Background(), obj) }()

	flowCtx, cancel := context.WithTimeout(ctx, f.rec.timeout)
	defer cancel()
	watcher := watchWorkload(flowCtx, obj)
	defer watcher.Stop()

	rec := f.observe(flowCtx, obj, watcher)
	orderErr := observedOrderErr(f.journey, rec.order, f.want())
	out := f.write(rec, rec.failure == "" && orderErr == nil)

	if rec.failure != "" {
		return out, errors.New(rec.failure)
	}
	return out, orderErr
}

// observe watches the workload and records every distinct CR until the flow finishes. It walks the
// journey's checkpoints - the stops carrying an action or an action predicate - popping the next once the
// workload reaches its state and its predicate holds (a nil predicate matches on state alone), firing the
// stop's action then. It keeps every distinct CR, advancing only once the status settles; plain states
// between checkpoints are recorded as they pass. On timeout or a closed watch it sets rec.failure, so the
// run is still recorded.
func (f *Flow) observe(ctx context.Context, obj *unstructured.Unstructured, watcher watch.Interface) *recording {
	rec := &recording{}
	pending := checkpoints(f.journey)
	done := func(state kartav1alpha1.ResourceStatus) bool {
		return state == f.want() && len(pending) == 0
	}

	// A workload already at its terminal state when Create returns never fires a watch event (the watch
	// replays only newer resourceVersions), so take that first snapshot from the create response.
	if state := cases.Classify(obj, f.rec.states); statusSettled(obj) && done(state) {
		rec.keep(obj, state)
		return rec
	}

	var lastSeen *unstructured.Unstructured
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			rec.failure = fmt.Sprintf("did not reach %q within %s; observed %v\nlast-seen status:\n%s",
				f.want(), f.rec.timeout, rec.order, dumpStatus(lastSeen))
			return rec
		case event, open := <-watcher.ResultChan():
			if !open {
				rec.failure = fmt.Sprintf("watch closed before reaching %q; observed %v; last watch error: %v\nlast-seen status:\n%s",
					f.want(), rec.order, lastErr, dumpStatus(lastSeen))
				return rec
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
			state := cases.Classify(u, f.rec.states)
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
				return rec
			}
		}
	}
}

// write persists the run as recorded_data/<operator>/<version>/<kartaName>/<flow>.yaml and returns it: its
// success flag, and each step's own-fields state, its CR (full on the first step, a merge-patch after), and
// any action fired.
func (f *Flow) write(rec *recording, succeeded bool) *Recording {
	version := operatorVersion(f.rec.operator)
	rc := Recording{
		SchemaVersion: SchemaVersion,
		Operator:      f.rec.operator,
		Version:       version,
		KartaName:     f.rec.kartaName,
		Flow:          f.name,
		Want:          string(f.want()),
		Succeeded:     succeeded,
		KartaFile:     strings.TrimPrefix(f.rec.kartaFile, "../../"),
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

	rc.Path = RecordingPath(filepath.Join(e2eRoot, "recorded_data"), rc)
	Expect(WriteRecording(rc.Path, rc)).To(Succeed())
	GinkgoWriter.Printf("recorded %s/%s/%s/%s.yaml (%d steps %v)\n", f.rec.operator, version, f.rec.kartaName, f.name, len(rc.Steps), rec.order)
	return &rc
}

func watchWorkload(ctx context.Context, obj *unstructured.Unstructured) watch.Interface {
	gvk := obj.GroupVersionKind()
	mapping, err := k8sClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	Expect(err).NotTo(HaveOccurred(), "rest mapping for %s", gvk)

	ns := obj.GetNamespace()
	Expect(ns).NotTo(BeEmpty(), "workload has no namespace")

	initialRV := obj.GetResourceVersion()
	if initialRV == "" { // Create was a no-op (object already existed); fetch a current RV to start from
		seed := GVKOnly(obj)
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), seed)).To(Succeed())
		initialRV = seed.GetResourceVersion()
	}

	watcher, err := watchtools.NewRetryWatcherWithContext(ctx, initialRV, &cache.ListWatch{
		WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", obj.GetName()).String()
			return dynClient.Resource(mapping.Resource).Namespace(ns).Watch(ctx, opts)
		},
	})
	Expect(err).NotTo(HaveOccurred())
	return watcher
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

// checkpoints are the stops the recorder must reach and fire in order: those carrying an action or an action
// predicate. Plain states between them are recorded as they pass but are not stops.
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

// significantCR drops the fields the apiserver bumps on every event (resourceVersion, managedFields) so keep
// dedups on real changes.
func significantCR(u *unstructured.Unstructured) map[string]any {
	c := u.DeepCopy().Object
	unstructured.RemoveNestedField(c, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(c, "metadata", "managedFields")
	return c
}

// statusSettled reports whether the controller has caught up (observedGeneration >= generation); workloads
// without those fields count as settled.
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
func observedOrderErr(journey []cases.Step, order []kartav1alpha1.ResourceStatus, want kartav1alpha1.ResourceStatus) error {
	return ObservedOrderErr(journeySteps(journey), order, want)
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

func mustRead(path string) []byte {
	b, err := os.ReadFile(filepath.Join(e2eRoot, path))
	Expect(err).NotTo(HaveOccurred(), "read %s", path)
	return b
}
