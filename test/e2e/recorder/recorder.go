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
	"os"
	"strings"
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
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Run applies the manifest, drives the workload through the journey, and writes the recording. On a
// flow-level failure the recording is still written (succeeded:false) for triage.
func (f *Flow) Run(ctx context.Context) (*Recording, error) {
	if len(f.journey) == 0 {
		return nil, fmt.Errorf("flow %s declares no stops", f.name)
	}
	workload, err := f.applyManifest(ctx)
	if err != nil {
		return nil, err
	}
	defer f.deleteWorkload(ctx, workload)

	flowCtx, cancel := context.WithTimeout(ctx, f.rec.timeout)
	defer cancel()

	obs := f.observe(flowCtx, workload)
	orderErr := observedOrderErr(f.journey, obs.states(), f.want())
	out, err := f.write(obs, obs.failure == "" && orderErr == nil)
	if err != nil {
		return nil, err
	}
	if obs.failure != "" {
		return out, errors.New(obs.failure)
	}
	return out, orderErr
}

// applyManifest reads the flow's workload manifest and creates it in the recorder's namespace.
func (f *Flow) applyManifest(ctx context.Context) (*unstructured.Unstructured, error) {
	raw, err := os.ReadFile(f.manifest)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", f.manifest, err)
	}
	workload := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(raw, workload); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", f.manifest, err)
	}
	workload.SetNamespace(f.rec.cluster.Namespace)
	if err := f.client().Create(ctx, workload); err != nil {
		return nil, fmt.Errorf("create workload for %s: %w", f.name, err)
	}
	return workload, nil
}

// deleteWorkload removes the workload once the flow is done. It runs on a fresh, bounded context so cleanup
// still happens after the flow's own context was cancelled; a failed delete is logged, not fatal.
func (f *Flow) deleteWorkload(ctx context.Context, workload *unstructured.Unstructured) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := f.client().Delete(ctx, workload); err != nil && !apierrors.IsNotFound(err) {
		fmt.Fprintf(f.log(), "cleanup: delete %s/%s failed: %v\n", workload.GetNamespace(), workload.GetName(), err)
	}
}

// observe watches the workload and records every distinct settled CR until the flow finishes, firing the
// journey's checkpoints as their states are reached. Its failure is set if the terminal state was not met.
func (f *Flow) observe(ctx context.Context, workload *unstructured.Unstructured) *observation {
	o := &observation{flow: f, workload: workload, pending: checkpoints(f.journey)}

	// A workload already at its terminal state when Create returns never fires a watch event (the watch
	// replays only newer resourceVersions), so record it straight from the create response.
	if statusSettled(workload) && o.reachedTerminal(Classify(workload, f.rec.states)) {
		o.record(ctx, workload)
		return o
	}
	o.follow(ctx)
	return o
}

// write persists the run under <outputDir>/<operator>/<version>/<kartaName>/<flow>.yaml and returns it: a
// STATE event per distinct CR, and an ACTION event after a state where the flow fired one.
func (f *Flow) write(obs *observation, succeeded bool) (*Recording, error) {
	out := Recording{
		SchemaVersion: SchemaVersion,
		Operator:      f.rec.operator,
		Version:       f.rec.version,
		KartaName:     f.rec.kartaName,
		Flow:          f.name,
		Want:          string(f.want()),
		Succeeded:     succeeded,
		KartaFile:     strings.TrimPrefix(f.rec.kartaFile, "../../"),
	}
	for _, snap := range obs.snapshots {
		out.Events = append(out.Events, Event{Kind: EventState, State: string(snap.state), Object: significantFields(snap.cr)})
		if snap.action != nil {
			out.Events = append(out.Events, Event{Kind: EventAction, Action: snap.action})
		}
	}

	out.Path = RecordingPath(f.rec.outputDir, out)
	if err := WriteRecording(out.Path, out); err != nil {
		return nil, fmt.Errorf("write recording %s: %w", out.Path, err)
	}
	fmt.Fprintf(f.log(), "recorded %s/%s/%s/%s.yaml (%d events %v)\n",
		f.rec.operator, f.rec.version, f.rec.kartaName, f.name, len(out.Events), obs.states())
	return &out, nil
}

// openWatch starts a resilient watch of the workload by name that resumes after transient drops.
func (f *Flow) openWatch(ctx context.Context, workload *unstructured.Unstructured) (watch.Interface, error) {
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
		current := gvkOnly(workload)
		if err := f.client().Get(ctx, client.ObjectKeyFromObject(workload), current); err != nil {
			return nil, fmt.Errorf("get %s for watch resource version: %w", workload.GetName(), err)
		}
		startRV = current.GetResourceVersion()
	}

	return watchtools.NewRetryWatcherWithContext(ctx, startRV, &cache.ListWatch{
		WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", workload.GetName()).String()
			return f.rec.cluster.Dynamic.Resource(mapping.Resource).Namespace(namespace).Watch(ctx, opts)
		},
	})
}

// fireAction sends an action's merge-patch to the workload and returns the recorded operation. The result
// object is not captured - the STATE events that follow already show where the operator drives it.
func (f *Flow) fireAction(ctx context.Context, workload *unstructured.Unstructured, action *Action) (*RecordedAction, error) {
	target := gvkOnly(workload)
	target.SetName(workload.GetName())
	target.SetNamespace(workload.GetNamespace())
	if err := f.client().Patch(ctx, target, client.RawPatch(types.MergePatchType, action.Patch)); err != nil {
		return nil, fmt.Errorf("action %s on %s/%s: %w", action.Type, workload.GetNamespace(), workload.GetName(), err)
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

// checkpoints are the journey stops the recorder must reach and fire in order: those carrying an action or
// an action predicate. Plain stops between them are recorded as they pass but are not checkpoints.
func checkpoints(journey []journeyStep) []journeyStep {
	var out []journeyStep
	for _, step := range journey {
		if step.Action != nil || step.ActionPredicate != nil {
			out = append(out, step)
		}
	}
	return out
}

// significantFields drops the fields the apiserver bumps on every event (resourceVersion, managedFields) so
// keep dedups on real changes.
func significantFields(cr *unstructured.Unstructured) map[string]any {
	stripped := cr.DeepCopy().Object
	unstructured.RemoveNestedField(stripped, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(stripped, "metadata", "managedFields")
	return stripped
}

// statusSettled reports whether the controller has caught up (observedGeneration >= generation); workloads
// without those fields count as settled.
func statusSettled(cr *unstructured.Unstructured) bool {
	gen, hasGen, _ := unstructured.NestedInt64(cr.Object, "metadata", "generation")
	observed, hasObserved, _ := unstructured.NestedInt64(cr.Object, "status", "observedGeneration")
	if !hasGen || !hasObserved {
		return true
	}
	return observed >= gen
}

// gvkOnly returns a fresh object carrying only src's GVK, so a merge-patch or a Get never sends back a stale
// spec or status.
func gvkOnly(src *unstructured.Unstructured) *unstructured.Unstructured {
	blank := &unstructured.Unstructured{}
	blank.SetGroupVersionKind(src.GroupVersionKind())
	return blank
}

// dumpStatus renders a CR's status block for the failure messages; safe on a nil object.
func dumpStatus(cr *unstructured.Unstructured) string {
	if cr == nil {
		return "(no object observed)"
	}
	status, _, _ := unstructured.NestedMap(cr.Object, "status")
	b, err := json.MarshalIndent(status, "  ", "  ")
	if err != nil {
		return fmt.Sprintf("(status marshal error: %v)", err)
	}
	return "  " + string(b)
}

func journeySteps(steps []journeyStep) []JourneyStep {
	out := make([]JourneyStep, len(steps))
	for i, step := range steps {
		out[i] = JourneyStep{State: step.State, Optional: step.Optional}
	}
	return out
}

// observedOrderErr runs the observed states through the same order check the replay golden uses.
func observedOrderErr(journey []journeyStep, observed []kartav1alpha1.ResourceStatus, want kartav1alpha1.ResourceStatus) error {
	return ObservedOrderErr(journeySteps(journey), observed, want)
}
