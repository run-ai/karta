// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"context"
	"fmt"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// observation is one watched run of a flow: the workload being driven, the checkpoints still to fire, and
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

// snapshot is one recorded CR: the state read from its own fields, the object, and any action fired at it.
type snapshot struct {
	state  kartav1alpha1.ResourceStatus
	cr     *unstructured.Unstructured
	action *RecordedAction
}

// follow watches the workload until the flow finishes or fails, recording each CR it sees. It reconnects
// when the apiserver drops a stale watch, and sets failure on timeout or an unrecoverable error.
func (o *observation) follow(ctx context.Context) {
	watcher, err := o.flow.openWatch(ctx, o.workload)
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

// record handles one observed CR: it keeps the CR if it is settled and new, fires the next checkpoint if the
// workload just reached it, and reports whether the flow is finished (or a fired action failed).
func (o *observation) record(ctx context.Context, cr *unstructured.Unstructured) (done bool) {
	o.lastSeen = cr
	if !statusSettled(cr) {
		return false // a half-written status is not a real state yet
	}
	state := Classify(cr, o.flow.rec.states)
	if state == "" {
		// A settled CR we cannot classify is a real gap: keep it as Undefined so the order check fails and
		// the run is saved for triage, rather than skipping it silently.
		state = kartav1alpha1.UndefinedStatus
	}
	o.keep(cr, state)
	if o.fireCheckpoint(ctx, state, cr) {
		return true // the action failed; stop and report it
	}
	return o.reachedTerminal(state)
}

// fireCheckpoint fires the next checkpoint's action if the workload just reached its state and gate, then
// drops it from pending. It reports whether the action failed, which stops the run.
func (o *observation) fireCheckpoint(ctx context.Context, state kartav1alpha1.ResourceStatus, cr *unstructured.Unstructured) (actionFailed bool) {
	next := o.pending
	reached := len(next) > 0 && state == next[0].State &&
		(next[0].ActionPredicate == nil || next[0].ActionPredicate(cr))
	if !reached {
		return false
	}
	if next[0].Action != nil {
		action, err := o.flow.fireAction(ctx, o.workload, next[0].Action)
		if err != nil {
			o.failure = err.Error()
			return true
		}
		o.attach(action)
	}
	o.pending = next[1:]
	return false
}

// reconnect handles a dropped watch: it re-lists the workload for a fresh resourceVersion, records that
// state, and opens a new watch. It returns the new watcher, or stop=true when the run must end - failure was
// set, or the re-listed state already finished the flow.
func (o *observation) reconnect(ctx context.Context, closed watch.Interface, lastWatchErr error) (watch.Interface, bool) {
	// The RetryWatcher gave up, typically "too old resource version" after the object sat idle while etcd
	// compacted its history. If the flow's context is already done, give up too.
	if ctx.Err() != nil {
		o.failure = fmt.Sprintf("watch closed before reaching %q; observed %v; last watch error: %v\nlast-seen status:\n%s",
			o.flow.want(), o.states(), lastWatchErr, dumpStatus(o.lastSeen))
		return nil, true
	}
	closed.Stop()

	current, failure := o.relist(ctx)
	if failure != "" {
		o.failure = failure
		return nil, true
	}
	o.workload.SetResourceVersion(current.GetResourceVersion())
	if o.record(ctx, current) {
		return nil, true
	}
	fresh, err := o.flow.openWatch(ctx, o.workload)
	if err != nil {
		o.failure = err.Error()
		return nil, true
	}
	return fresh, false
}

// relist fetches the workload's current object after a watch drop, retrying transient control-plane blips (a
// heavy operator can briefly reject the Get while its authorizer reloads). It returns the object to resume
// from, or a non-empty failure message when it must give up: the workload is gone, or the context ended.
func (o *observation) relist(ctx context.Context) (*unstructured.Unstructured, string) {
	current := gvkOnly(o.workload)
	for {
		err := o.flow.client().Get(ctx, client.ObjectKeyFromObject(o.workload), current)
		if err == nil {
			return current, ""
		}
		if apierrors.IsNotFound(err) {
			return nil, fmt.Sprintf("workload gone during flow (re-list 404); observed %v", o.states())
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Sprintf("re-list after watch closed kept failing: %v; observed %v", err, o.states())
		case <-time.After(2 * time.Second):
		}
	}
}

// keep appends cr as a new snapshot, unless it duplicates the last kept one (same significant fields).
func (o *observation) keep(cr *unstructured.Unstructured, state kartav1alpha1.ResourceStatus) {
	sig := significantFields(cr)
	if o.lastSig != nil && reflect.DeepEqual(o.lastSig, sig) {
		return
	}
	o.lastSig = sig
	o.snapshots = append(o.snapshots, snapshot{state: state, cr: cr.DeepCopy()})
}

// attach records the action fired at the snapshot just kept.
func (o *observation) attach(action *RecordedAction) {
	if len(o.snapshots) > 0 {
		o.snapshots[len(o.snapshots)-1].action = action
	}
}

// states is the ordered sequence of states the workload passed through - the walk the order check validates.
func (o *observation) states() []kartav1alpha1.ResourceStatus {
	out := make([]kartav1alpha1.ResourceStatus, len(o.snapshots))
	for i, s := range o.snapshots {
		out[i] = s.state
	}
	return out
}

// reachedTerminal reports whether state is the flow's terminal state and every checkpoint has already fired.
func (o *observation) reachedTerminal(state kartav1alpha1.ResourceStatus) bool {
	return state == o.flow.want() && len(o.pending) == 0
}
