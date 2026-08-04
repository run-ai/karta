// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package recorder drives a workload through a declared flow and records the CRs it passes through, for the
// replay tests to check against Karta. It judges each state from the workload's own fields and never runs
// Karta, and depends on no test framework: failures come back as errors, progress goes to an injected writer.
package recorder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

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
)

// e2eRoot points from a package dir under test/e2e (the test working dir) to test/e2e, which paths are relative to.
const e2eRoot = ".."

var (
	k8sClient     client.Client
	dynClient     dynamic.Interface
	serverVersion string
	namespace     string
	progress      io.Writer = io.Discard
)

// Bind wires the recorder to the cluster clients, Kubernetes version, namespace, and a progress writer (nil
// discards). The suite calls it once in BeforeSuite.
func Bind(c client.Client, d dynamic.Interface, version, ns string, log io.Writer) {
	k8sClient, dynClient, serverVersion, namespace = c, d, version, ns
	if log != nil {
		progress = log
	}
}

// Run applies the manifest, drives the workload through the journey, and writes the recording. On a
// flow-level failure the recording is still written (succeeded:false) for triage.
func (f *Flow) Run(ctx context.Context) (*Recording, error) {
	if len(f.journey) == 0 {
		return nil, fmt.Errorf("flow %s declares no stops", f.name)
	}
	manifest, err := readManifest(f.manifest)
	if err != nil {
		return nil, err
	}
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(manifest, obj); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", f.manifest, err)
	}
	obj.SetNamespace(namespace)
	if err := k8sClient.Create(ctx, obj); err != nil {
		return nil, fmt.Errorf("create workload for %s: %w", f.name, err)
	}
	defer func() {
		delCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := k8sClient.Delete(delCtx, obj); err != nil && !apierrors.IsNotFound(err) {
			fmt.Fprintf(progress, "cleanup: delete %s/%s failed: %v\n", obj.GetNamespace(), obj.GetName(), err)
		}
	}()

	flowCtx, cancel := context.WithTimeout(ctx, f.rec.timeout)
	defer cancel()

	rec := f.observe(flowCtx, obj)
	orderErr := observedOrderErr(f.journey, rec.order, f.want())
	out, err := f.write(rec, rec.failure == "" && orderErr == nil)
	if err != nil {
		return nil, err
	}
	if rec.failure != "" {
		return out, errors.New(rec.failure)
	}
	return out, orderErr
}

// observe watches the workload and records every distinct settled CR, driving the journey's checkpoints
// (stops with an action or predicate) in order and firing their actions. Any failure sets rec.failure.
func (f *Flow) observe(ctx context.Context, obj *unstructured.Unstructured) *recording {
	rec := &recording{}
	pending := checkpoints(f.journey)
	done := func(state kartav1alpha1.ResourceStatus) bool {
		return state == f.want() && len(pending) == 0
	}

	var lastSeen *unstructured.Unstructured
	handle := func(u *unstructured.Unstructured) bool {
		lastSeen = u
		if !statusSettled(u) {
			return false // do not advance on a half-written status
		}
		state := Classify(u, f.rec.states)
		if state == "" {
			// A settled CR we cannot classify is a real gap: record it as Undefined so the order check
			// fails and the run is saved for triage, rather than skipping it silently.
			state = kartav1alpha1.UndefinedStatus
		}
		rec.keep(u, state)
		var stop bool
		if pending, stop = fireReachedCheckpoint(ctx, obj, u, state, pending, rec); stop {
			return true
		}
		return done(state)
	}

	// A workload already at its terminal state when Create returns never fires a watch event (the watch
	// replays only newer resourceVersions), so snapshot it from the create response.
	if statusSettled(obj) && done(Classify(obj, f.rec.states)) {
		handle(obj)
		return rec
	}

	watcher, err := watchWorkload(ctx, obj)
	if err != nil {
		rec.failure = err.Error()
		return rec
	}
	defer func() { watcher.Stop() }()
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			rec.failure = fmt.Sprintf("did not reach %q within %s; observed %v\nlast-seen status:\n%s",
				f.want(), f.rec.timeout, rec.order, dumpStatus(lastSeen))
			return rec
		case event, open := <-watcher.ResultChan():
			if !open {
				// The RetryWatcher gave up (typically "too old resource version" after idle compaction).
				// Re-list for a fresh resourceVersion and resume rather than failing a slow workload.
				if ctx.Err() != nil {
					rec.failure = fmt.Sprintf("watch closed before reaching %q; observed %v; last watch error: %v\nlast-seen status:\n%s",
						f.want(), rec.order, lastErr, dumpStatus(lastSeen))
					return rec
				}
				watcher.Stop()
				seed := GVKOnly(obj)
				// Retry the re-list: a control-plane blip under a heavy operator can return a transient
				// error that must not abort a multi-minute flow.
				var gerr error
				for {
					if gerr = k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), seed); gerr == nil {
						break
					}
					if apierrors.IsNotFound(gerr) {
						rec.failure = fmt.Sprintf("workload gone during flow (re-list 404); observed %v", rec.order)
						return rec
					}
					select {
					case <-ctx.Done():
						rec.failure = fmt.Sprintf("re-list after watch closed kept failing: %v; observed %v", gerr, rec.order)
						return rec
					case <-time.After(2 * time.Second):
					}
				}
				obj.SetResourceVersion(seed.GetResourceVersion())
				if handle(seed) {
					return rec
				}
				if watcher, err = watchWorkload(ctx, obj); err != nil {
					rec.failure = err.Error()
					return rec
				}
				continue
			}
			if event.Type == watch.Error {
				lastErr = apierrors.FromObject(event.Object)
				continue
			}
			u, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue // a bookmark carries no workload object
			}
			if handle(u) {
				return rec
			}
		}
	}
}

// write persists the run under recorded_data/<operator>/<version>/<kartaName>/<flow>.yaml.
func (f *Flow) write(rec *recording, succeeded bool) (*Recording, error) {
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
	for _, snap := range rec.snapshots {
		rc.Events = append(rc.Events, Event{Kind: EventState, State: string(snap.state), Object: significantCR(snap.raw)})
		if snap.action != nil {
			rc.Events = append(rc.Events, Event{Kind: EventAction, Action: snap.action})
		}
	}

	rc.Path = RecordingPath(filepath.Join(e2eRoot, "recorded_data"), rc)
	if err := WriteRecording(rc.Path, rc); err != nil {
		return nil, fmt.Errorf("write recording %s: %w", rc.Path, err)
	}
	fmt.Fprintf(progress, "recorded %s/%s/%s/%s.yaml (%d events %v)\n", f.rec.operator, version, f.rec.kartaName, f.name, len(rc.Events), rec.order)
	return &rc, nil
}

func watchWorkload(ctx context.Context, obj *unstructured.Unstructured) (watch.Interface, error) {
	gvk := obj.GroupVersionKind()
	mapping, err := k8sClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("rest mapping for %s: %w", gvk, err)
	}

	ns := obj.GetNamespace()
	if ns == "" {
		return nil, fmt.Errorf("workload %s has no namespace", obj.GetName())
	}

	initialRV := obj.GetResourceVersion()
	if initialRV == "" { // Create was a no-op (object already existed); fetch a current RV to start from
		seed := GVKOnly(obj)
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), seed); err != nil {
			return nil, fmt.Errorf("get %s for watch resource version: %w", obj.GetName(), err)
		}
		initialRV = seed.GetResourceVersion()
	}

	return watchtools.NewRetryWatcherWithContext(ctx, initialRV, &cache.ListWatch{
		WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", obj.GetName()).String()
			return dynClient.Resource(mapping.Resource).Namespace(ns).Watch(ctx, opts)
		},
	})
}

// fireAction sends an action's merge-patch and returns the recorded operation; the result object is not
// captured, since the STATE events that follow already show where the operator drives it.
func fireAction(ctx context.Context, obj *unstructured.Unstructured, action *Action) (*RecordedAction, error) {
	target := GVKOnly(obj)
	target.SetName(obj.GetName())
	target.SetNamespace(obj.GetNamespace())
	if err := k8sClient.Patch(ctx, target, client.RawPatch(types.MergePatchType, action.Patch)); err != nil {
		return nil, fmt.Errorf("action %s on %s/%s: %w", action.Type, obj.GetNamespace(), obj.GetName(), err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(action.Patch, &payload); err != nil {
		return nil, fmt.Errorf("action %s payload: %w", action.Type, err)
	}
	return &RecordedAction{
		Name:      string(action.Type),
		Operation: Operation{Verb: "PATCH", PatchType: "application/merge-patch+json", Payload: payload},
	}, nil
}

func checkpoints(journey []journeyStep) []journeyStep {
	var out []journeyStep
	for _, st := range journey {
		if st.Action != nil || st.ActionPredicate != nil {
			out = append(out, st)
		}
	}
	return out
}

// fireReachedCheckpoint fires the next checkpoint's action once the workload reaches its state and its gate
// holds, then drops it from pending. It returns the remaining checkpoints and whether to stop the run (a
// failed action stops it).
func fireReachedCheckpoint(ctx context.Context, obj, u *unstructured.Unstructured, state kartav1alpha1.ResourceStatus, pending []journeyStep, rec *recording) ([]journeyStep, bool) {
	if len(pending) == 0 || state != pending[0].State ||
		(pending[0].ActionPredicate != nil && !pending[0].ActionPredicate(u)) {
		return pending, false
	}
	if pending[0].Action != nil {
		ra, err := fireAction(ctx, obj, pending[0].Action)
		if err != nil {
			rec.failure = err.Error()
			return pending, true
		}
		rec.attachAction(ra)
	}
	return pending[1:], false
}

type recording struct {
	order     []kartav1alpha1.ResourceStatus
	snapshots []capture
	lastSig   map[string]any
	failure   string // why the flow did not reach its terminal, if it did not
}

type capture struct {
	state  kartav1alpha1.ResourceStatus
	raw    *unstructured.Unstructured
	action *RecordedAction
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

func (r *recording) attachAction(a *RecordedAction) {
	if len(r.snapshots) > 0 {
		r.snapshots[len(r.snapshots)-1].action = a
	}
}

// significantCR drops the fields the apiserver bumps every event (resourceVersion, managedFields) so keep
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

func journeySteps(steps []journeyStep) []JourneyStep {
	out := make([]JourneyStep, len(steps))
	for i, st := range steps {
		out[i] = JourneyStep{State: st.State, Optional: st.Optional}
	}
	return out
}

func observedOrderErr(journey []journeyStep, order []kartav1alpha1.ResourceStatus, want kartav1alpha1.ResourceStatus) error {
	return ObservedOrderErr(journeySteps(journey), order, want)
}

// operatorVersion is the installed version of op, or the cluster's Kubernetes version for built-ins.
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

func readManifest(path string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(e2eRoot, path))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return b, nil
}
