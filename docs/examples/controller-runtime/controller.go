// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package main

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// Annotation keys written back to every reconciled workload. The controller
// never references a CRD-specific field path to compute these values — Karta
// resolves them from the structure definition stored in the cluster.
const (
	annotationStatus   = "karta.run.ai/status"
	annotationReplicas = "karta.run.ai/replicas"
	annotationCPU      = "karta.run.ai/cpu-request"
	annotationMemory   = "karta.run.ai/memory-request"
	annotationGPU      = "karta.run.ai/gpu-request"

	// managedByLabel is injected into the pod template. Pod templates are
	// immutable on a running Job, so this is applied only while suspended.
	managedByLabelKey   = "app.kubernetes.io/managed-by"
	managedByLabelValue = "karta"

	gpuResourceName = "nvidia.com/gpu"
)

// jobGVK is the workload this example watches. Swapping it (plus the Karta
// object in the cluster) is all it takes to manage a different workload type —
// no other code in this file changes.
var jobGVK = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}

// JobReconciler inspects and mutates Jobs through a Karta definition that lives
// in the cluster as a Karta custom resource.
type JobReconciler struct {
	client.Client
	Recorder events.EventRecorder
}

// Reconcile reacts to every change of a watched Job (and to changes of the
// Karta definition itself). The body contains no switch on workload kind: the
// Karta object absorbs all structural differences.
func (r *JobReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	// Step 0: discover the workload structure from the cluster by GVK. The
	// controller does not assume a Karta name: it selects the Karta whose root
	// component matches the watched workload's GVK, the same way a real consumer
	// resolves a definition for an arbitrary workload type.
	karta, err := r.kartaForGVK(ctx, jobGVK)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("no Karta describes this workload; apply one before workloads", "gvk", jobGVK.String())
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Step 1: fetch the workload as an unstructured object.
	job := &unstructured.Unstructured{}
	job.SetGroupVersionKind(jobGVK)
	if err := r.Get(ctx, req.NamespacedName, job); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil // deleted; nothing to do
		}
		return reconcile.Result{}, fmt.Errorf("get Job %s: %w", req.NamespacedName, err)
	}

	// Step 2: the single Karta entry point for all read and write operations.
	factory := resource.NewComponentFactoryFromObject(karta, job)
	root, err := factory.GetRootComponent()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get root component: %w", err)
	}

	// Step 3: inspect status, scale and resource requests uniformly.
	summary, suspended, err := inspect(ctx, root)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Step 4: mutate the pod template (label only) while the workload sits in
	// its mutable window. A Job's spec.template is immutable once it is running,
	// so for a running Job the label is skipped and an event explains why.
	labelChanged := false
	if suspended {
		changed, err := injectPodTemplateLabel(ctx, root)
		if err != nil {
			return reconcile.Result{}, err
		}
		labelChanged = changed
		// GetResource hands back the full mutated object; continue working on it
		// so a single Update persists both the label and the annotations below.
		mutated, err := factory.GetResource()
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("get mutated resource: %w", err)
		}
		job = mutated.(*unstructured.Unstructured)
		if labelChanged {
			r.Recorder.Eventf(job, nil, corev1.EventTypeNormal, "PodTemplateLabeled", "InjectLabel",
				"Injected pod-template label %s=%s via Karta", managedByLabelKey, managedByLabelValue)
		}
	}

	// Step 5: report the inspection on the workload's own metadata. Object
	// annotations are always mutable, so this persists on any Job.
	annotationsChanged := applyAnnotations(job, summary)

	if !annotationsChanged && !labelChanged {
		return reconcile.Result{}, nil // idempotent: nothing to write
	}

	if err := r.Update(ctx, job); err != nil {
		if apierrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, fmt.Errorf("update Job %s: %w", req.NamespacedName, err)
	}
	if !suspended {
		r.Recorder.Eventf(job, nil, corev1.EventTypeNormal, "PodTemplateImmutable", "InjectLabel",
			"Pod template is immutable while the Job runs; suspend it to inject labels")
	}
	r.Recorder.Eventf(job, nil, corev1.EventTypeNormal, "Inspected", "Inspect",
		"Karta: status=%s replicas=%s cpu=%s memory=%s gpu=%s",
		summary[annotationStatus], summary[annotationReplicas],
		summary[annotationCPU], summary[annotationMemory], summary[annotationGPU])
	logger.Info("reconciled", "job", req.NamespacedName, "status", summary[annotationStatus], "suspended", suspended)
	return reconcile.Result{}, nil
}

// inspect reads the unified status, replica count and aggregate resource
// requests for the root component. It returns the annotation summary and
// whether the workload is currently suspended (its pod-template mutable window).
func inspect(ctx context.Context, root *resource.Component) (map[string]string, bool, error) {
	status, err := root.GetStatus(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("get status: %w", err)
	}

	suspended := false
	statuses := make([]string, 0, len(status.MatchedStatuses))
	for _, s := range status.MatchedStatuses {
		statuses = append(statuses, string(s))
		if s == v1alpha1.SuspendedStatus {
			suspended = true
		}
	}
	statusText := strings.Join(statuses, ",")
	if statusText == "" {
		statusText = string(v1alpha1.UndefinedStatus)
	}

	scales, err := root.GetScale(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("get scale: %w", err)
	}
	replicas := int32(0)
	for _, scale := range scales {
		if scale.Replicas != nil {
			replicas += *scale.Replicas
		}
	}

	cpu, mem, gpu := apiresource.Quantity{}, apiresource.Quantity{}, apiresource.Quantity{}
	podTemplateSpecs, err := root.GetPodTemplateSpec(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("get pod template spec: %w", err)
	}
	for _, pts := range podTemplateSpecs {
		for _, c := range pts.Spec.Containers {
			req := c.Resources.Requests
			cpu.Add(*req.Cpu())
			mem.Add(*req.Memory())
			if g, ok := req[corev1.ResourceName(gpuResourceName)]; ok {
				gpu.Add(g)
			} else if g, ok := c.Resources.Limits[corev1.ResourceName(gpuResourceName)]; ok {
				gpu.Add(g)
			}
		}
	}

	summary := map[string]string{
		annotationStatus:   statusText,
		annotationReplicas: fmt.Sprintf("%d", replicas),
		annotationCPU:      cpu.String(),
		annotationMemory:   mem.String(),
		annotationGPU:      gpu.String(),
	}
	return summary, suspended, nil
}

// applyAnnotations writes the summary onto the object and reports whether any
// value actually changed, so the caller can avoid no-op updates.
func applyAnnotations(obj *unstructured.Unstructured, summary map[string]string) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	changed := false
	for k, v := range summary {
		if annotations[k] != v {
			annotations[k] = v
			changed = true
		}
	}
	if changed {
		obj.SetAnnotations(annotations)
	}
	return changed
}

// injectPodTemplateLabel adds the managed-by label to every pod template of the
// root component through Karta, regardless of where the template lives in the
// underlying CRD. It reports whether any template actually changed so callers
// can skip a no-op Update.
func injectPodTemplateLabel(ctx context.Context, root *resource.Component) (bool, error) {
	podTemplateSpecs, err := root.GetPodTemplateSpec(ctx)
	if err != nil {
		return false, fmt.Errorf("get pod template spec: %w", err)
	}
	changed := false
	updates := make(map[string]corev1.PodTemplateSpec, len(podTemplateSpecs))
	for id, pts := range podTemplateSpecs {
		if pts.Labels[managedByLabelKey] != managedByLabelValue {
			if pts.Labels == nil {
				pts.Labels = map[string]string{}
			}
			pts.Labels[managedByLabelKey] = managedByLabelValue
			changed = true
		}
		updates[id] = pts
	}
	if !changed {
		return false, nil
	}
	if err := root.UpdatePodTemplateSpec(ctx, updates); err != nil {
		return false, fmt.Errorf("update pod template spec: %w", err)
	}
	return true, nil
}

// kartaForGVK selects the Karta whose root component describes the given GVK.
// This mirrors how a real consumer resolves a definition for an arbitrary
// workload type instead of hard-coding a Karta name.
func (r *JobReconciler) kartaForGVK(ctx context.Context, gvk schema.GroupVersionKind) (*v1alpha1.Karta, error) {
	list := &v1alpha1.KartaList{}
	if err := r.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list Kartas: %w", err)
	}
	for i := range list.Items {
		if rootGVK := rootComponentGVK(&list.Items[i]); rootGVK != nil && *rootGVK == gvk {
			return &list.Items[i], nil
		}
	}
	return nil, apierrors.NewNotFound(v1alpha1.Resource("karta"), fmt.Sprintf("for gvk %s", gvk))
}

// rootComponentGVK returns the GVK of a Karta's root component, or nil if unset.
func rootComponentGVK(karta *v1alpha1.Karta) *schema.GroupVersionKind {
	kind := karta.Spec.StructureDefinition.RootComponent.Kind
	if kind == nil {
		return nil
	}
	return &schema.GroupVersionKind{Group: kind.Group, Version: kind.Version, Kind: kind.Kind}
}

// SetupWithManager wires the reconciler to watch Jobs and the Karta definition.
// A change to the Karta object re-enqueues every Job so the new structure takes
// effect live, with no redeploy.
func (r *JobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	jobType := &unstructured.Unstructured{}
	jobType.SetGroupVersionKind(jobGVK)

	return ctrl.NewControllerManagedBy(mgr).
		For(jobType).
		Watches(&v1alpha1.Karta{}, handler.EnqueueRequestsFromMapFunc(r.jobsForKarta)).
		Complete(r)
}

// jobsForKarta enqueues all Jobs when a Karta describing this workload changes.
func (r *JobReconciler) jobsForKarta(ctx context.Context, obj client.Object) []reconcile.Request {
	karta, ok := obj.(*v1alpha1.Karta)
	if !ok {
		return nil
	}
	if rootGVK := rootComponentGVK(karta); rootGVK == nil || *rootGVK != jobGVK {
		return nil
	}
	jobs := &unstructured.UnstructuredList{}
	jobs.SetGroupVersionKind(schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "JobList"})
	if err := r.List(ctx, jobs); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0, len(jobs.Items))
	for i := range jobs.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: jobs.Items[i].GetNamespace(),
				Name:      jobs.Items[i].GetName(),
			},
		})
	}
	return requests
}
