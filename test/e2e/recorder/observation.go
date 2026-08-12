// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// observation is one watched run of a flow: the workload being driven, the checkpoints still to act on, and
// the CRs recorded so far. Its methods do the driving and the recording; Flow.observe fills it in.
type observation struct {
	flow     *Flow
	workload *unstructured.Unstructured

	pending   []journeyStep              // checkpoints not yet reached, in order
	snapshots []snapshot                 // the distinct CRs kept, in order
	lastSig   map[string]any             // significant fields of the last kept CR, for dedup
	lastSeen  *unstructured.Unstructured // most recent CR, shown in triage on timeout
	failure   string                     // why the flow did not finish, empty if it did
}

// snapshot is one recorded CR: the state read from its own fields, the object, and any action performed at it.
type snapshot struct {
	state                   kartav1alpha1.ResourceStatus
	cr                      *unstructured.Unstructured
	action                  *RecordedAction
	staleObservedGeneration bool // the controller had not observed the spec yet; recorded, but never judged
}

// follow watches the workload until the flow finishes or fails, recording each CR it sees. It reconnects
// when the apiserver drops a stale watch, and sets failure on timeout or an unrecoverable error.
func (o *observation) follow(ctx context.Context) {
	watcher, err := o.flow.startWatch(ctx, o.workload)
	if err != nil {
		o.failure = err.Error()
		return
	}
	defer watcher.Stop()

	var lastWatchErr error
	for {
		select {
		case <-ctx.Done():
			o.failure = fmt.Sprintf("did not reach %q within %s; observed %v\nlast-seen status:\n%s",
				o.flow.want(), o.flow.rec.timeout, o.states(), dumpStatus(o.lastSeen))
			return
		case event, open := <-watcher.ResultChan():
			switch {
			case !open:
				var stop bool
				if watcher, stop = o.reconnect(ctx, watcher, lastWatchErr); stop {
					return
				}
			case event.Type == watch.Error:
				lastWatchErr = apierrors.FromObject(event.Object)
			default:
				cr, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					continue // a bookmark carries no workload object
				}
				if o.record(ctx, cr) {
					return
				}
			}
		}
	}
}

// startWatch starts a resilient watch of the workload by name that resumes after transient drops.
func (f *Flow) startWatch(ctx context.Context, workload *unstructured.Unstructured) (watch.Interface, error) {
	gvk := workload.GroupVersionKind()
	mapping, err := f.client().RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("rest mapping for %s: %w", gvk, err)
	}
	namespace := workload.GetNamespace()
	if namespace == "" {
		return nil, fmt.Errorf("workload %s has no namespace", workload.GetName())
	}

	startRV := workload.GetResourceVersion()
	if startRV == "" { // Create was a no-op (the object already existed); fetch a current RV to start from
		current := blankWithGVK(workload)
		if err := f.client().Get(ctx, client.ObjectKeyFromObject(workload), current); err != nil {
			return nil, fmt.Errorf("get %s for watch resource version: %w", workload.GetName(), err)
		}
		startRV = current.GetResourceVersion()
	}

	return watchtools.NewRetryWatcherWithContext(ctx, startRV, &cache.ListWatch{
		WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", workload.GetName()).String()
			return f.rec.config.Cluster.Dynamic.Resource(mapping.Resource).Namespace(namespace).Watch(ctx, opts)
		},
	})
}

// record handles one seen CR: it keeps every distinct frame, but acts on checkpoints and the terminal only
// once the controller has observed the current spec.
func (o *observation) record(ctx context.Context, cr *unstructured.Unstructured) (done bool) {
	o.lastSeen = cr
	state := classify(cr, o.flow.rec.states)
	if state == "" {
		// A CR we cannot classify is a real gap: keep it as Undefined so an observed frame fails the order
		// check and the run is saved for triage, rather than skipping it silently.
		state = kartav1alpha1.UndefinedStatus
	}
	observed := isWorkloadObserved(cr)
	o.keep(cr, state, observed)
	if !observed {
		return false
	}
	if o.advanceCheckpoint(ctx, state, cr) {
		return true // the action failed; stop and report it
	}
	return o.hasReachedTerminal(state)
}

// keep appends cr as a new snapshot, unless it duplicates the last kept one (same significant fields).
func (o *observation) keep(cr *unstructured.Unstructured, state kartav1alpha1.ResourceStatus, observed bool) {
	sig := significantFields(cr)
	if o.lastSig != nil && reflect.DeepEqual(o.lastSig, sig) {
		return
	}
	o.lastSig = sig
	o.snapshots = append(o.snapshots, snapshot{state: state, cr: cr.DeepCopy(), staleObservedGeneration: !observed})
}

// advanceCheckpoint performs the next checkpoint's action if the workload just reached its state and gate, then
// drops it from pending. It reports whether the action failed, which stops the run.
func (o *observation) advanceCheckpoint(ctx context.Context, state kartav1alpha1.ResourceStatus, cr *unstructured.Unstructured) (actionFailed bool) {
	next := o.pending
	reached := len(next) > 0 && state == next[0].State &&
		(next[0].ActionPredicate == nil || next[0].ActionPredicate(cr))
	if !reached {
		return false
	}
	if next[0].Action != nil {
		action, err := o.flow.performAction(ctx, o.workload, next[0].Action)
		if err != nil {
			o.failure = err.Error()
			return true
		}
		o.attachAction(action)
	}
	o.pending = next[1:]
	return false
}

// performAction sends an action's merge-patch to the workload and returns the recorded operation. The result
// object is not captured - the STATE events that follow already show where the operator drives it.
func (f *Flow) performAction(ctx context.Context, workload *unstructured.Unstructured, action *Action) (*RecordedAction, error) {
	target := blankWithGVK(workload)
	target.SetName(workload.GetName())
	target.SetNamespace(workload.GetNamespace())
	if err := f.client().Patch(ctx, target, client.RawPatch(types.MergePatchType, action.Patch)); err != nil {
		return nil, fmt.Errorf("action %s on %s/%s: %w", action.Type, workload.GetNamespace(), workload.GetName(), err)
	}

	var payload map[string]any
	if err := json.Unmarshal(action.Patch, &payload); err != nil {
		return nil, fmt.Errorf("action %s payload: %w", action.Type, err)
	}
	return &RecordedAction{
		Name:      string(action.Type),
		Operation: Operation{Verb: "PATCH", PatchType: "application/merge-patch+json", Payload: payload},
	}, nil
}

// attachAction records the action performed at the snapshot just kept.
func (o *observation) attachAction(action *RecordedAction) {
	if len(o.snapshots) > 0 {
		o.snapshots[len(o.snapshots)-1].action = action
	}
}

// hasReachedTerminal reports whether state is the flow's terminal state and no checkpoints remain.
func (o *observation) hasReachedTerminal(state kartav1alpha1.ResourceStatus) bool {
	return state == o.flow.want() && len(o.pending) == 0
}

// reconnect handles a dropped watch: it re-fetches the workload for a fresh resourceVersion, records that
// state, and opens a new watch. It returns the new watcher, or stop=true when the run must end - failure was
// set, or the re-fetched state already finished the flow.
func (o *observation) reconnect(ctx context.Context, closed watch.Interface, lastWatchErr error) (watch.Interface, bool) {
	// The RetryWatcher gave up, typically "too old resource version" after the object sat idle while etcd
	// compacted its history. If the flow's context is already done, give up too.
	if ctx.Err() != nil {
		o.failure = fmt.Sprintf("watch closed before reaching %q; observed %v; last watch error: %v\nlast-seen status:\n%s",
			o.flow.want(), o.states(), lastWatchErr, dumpStatus(o.lastSeen))
		return nil, true
	}
	closed.Stop()

	current, failure := o.refetch(ctx)
	if failure != "" {
		o.failure = failure
		return nil, true
	}
	o.workload.SetResourceVersion(current.GetResourceVersion())
	if o.record(ctx, current) {
		return nil, true
	}
	fresh, err := o.flow.startWatch(ctx, o.workload)
	if err != nil {
		o.failure = err.Error()
		return nil, true
	}
	return fresh, false
}

// refetch fetches the workload's current object after a watch drop, retrying transient control-plane blips (a
// heavy operator can briefly reject the Get while its authorizer reloads). It returns the object to resume
// from, or a non-empty failure message when it must give up: the workload is gone, or the context ended.
func (o *observation) refetch(ctx context.Context) (*unstructured.Unstructured, string) {
	current := blankWithGVK(o.workload)
	for {
		err := o.flow.client().Get(ctx, client.ObjectKeyFromObject(o.workload), current)
		if err == nil {
			return current, ""
		}
		if apierrors.IsNotFound(err) {
			return nil, fmt.Sprintf("workload gone during flow (re-fetch 404); observed %v", o.states())
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Sprintf("re-fetch after watch closed kept failing: %v; observed %v", err, o.states())
		case <-time.After(2 * time.Second):
		}
	}
}

// states is the ordered sequence of observed-frame states - the walk the order check validates.
func (o *observation) states() []kartav1alpha1.ResourceStatus {
	var out []kartav1alpha1.ResourceStatus
	for _, s := range o.snapshots {
		if !s.staleObservedGeneration {
			out = append(out, s.state)
		}
	}
	return out
}
