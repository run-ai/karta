// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package workload

import (
	"context"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/run-ai/karta/pkg/physical"
)

// maxOwnerDepth bounds the owner-reference walk. Real controller chains (root
// -> intermediate -> Pod) are at most two or three hops; this guards against a
// cycle in adversarial or malformed data.
const maxOwnerDepth = 6

// PodStats aggregates live pod state for a workload, as opposed to View.GPUs
// and the component Replicas, which are the requested totals computed from
// the spec.
type PodStats struct {
	PodsTotal     int32 `json:"podsTotal"`
	PodsRunning   int32 `json:"podsRunning"`
	AllocatedGPUs int64 `json:"allocatedGpus"`
	RequestedCPU  int64 `json:"requestedCpuMillis"`
	AllocatedCPU  int64 `json:"allocatedCpuMillis"`
	RequestedMem  int64 `json:"requestedMemoryBytes"`
	AllocatedMem  int64 `json:"allocatedMemoryBytes"`
	// Restarts sums every container's restart count across attributed pods.
	Restarts int32 `json:"restarts"`
	// Nodes lists the distinct nodes attributed pods have been scheduled to.
	Nodes []string `json:"nodes,omitempty"`
	// PendingReason is the first scheduling failure found among attributed
	// pods that are not yet Running, empty when none is pending or unschedulable.
	PendingReason string `json:"pendingReason,omitempty"`
	// OldestPod and NewestPod are the creation timestamps of the oldest and
	// newest attributed pod, zero when there are none.
	OldestPod time.Time `json:"oldestPod,omitempty"`
	NewestPod time.Time `json:"newestPod,omitempty"`

	// DeviceCount, DegradedNodes and Domains stay zero unless EnrichStats has
	// run, so a plain "karta get" is unaffected.
	//
	// DeviceCount is the number of individually-identified DRA devices held
	// across the attributed pods. It differs from AllocatedGPUs, which is the
	// requested-and-running count off the pod spec: AllocatedGPUs is what was
	// asked for, DeviceCount is what was actually allocated and can be named.
	DeviceCount int `json:"deviceCount,omitempty"`
	// Devices lists each allocated device as "name (driver/pool)".
	Devices []string `json:"devices,omitempty"`
	// DegradedNodes lists nodes among the attributed pods' nodes that are
	// NotReady or cordoned.
	DegradedNodes []string `json:"degradedNodes,omitempty"`
	// Domains lists the distinct topology domains the attributed pods' nodes
	// belong to. More than one entry means the workload spans a fabric
	// boundary.
	Domains []string `json:"domains,omitempty"`
}

// PodAttributor attributes live Pods to a workload root by walking each
// candidate pod's owner-reference chain, fetching intermediate controller
// objects (ReplicaSet, Job, and so on) as needed. It caches fetched objects
// across calls, since sibling pods usually share an intermediate owner.
type PodAttributor struct {
	dyn    dynamic.Interface
	mapper meta.RESTMapper
	cache  map[ownerKey]*unstructured.Unstructured
}

type ownerKey struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

func NewPodAttributor(dyn dynamic.Interface, mapper meta.RESTMapper) *PodAttributor {
	return &PodAttributor{dyn: dyn, mapper: mapper, cache: map[ownerKey]*unstructured.Unstructured{}}
}

// Filter returns the pods whose owner-reference chain reaches rootUID. pods
// may span namespaces, as in a cluster-wide search.
func (a *PodAttributor) Filter(ctx context.Context, pods []corev1.Pod, rootUID types.UID) []corev1.Pod {
	var matched []corev1.Pod
	for i := range pods {
		if a.belongsTo(ctx, pods[i].OwnerReferences, pods[i].Namespace, rootUID, 0) {
			matched = append(matched, pods[i])
		}
	}
	return matched
}

// Attribute reports pod counts and allocated GPUs for the pods whose
// owner-reference chain reaches rootUID. pods may span namespaces, as in a
// cluster-wide search.
func (a *PodAttributor) Attribute(ctx context.Context, pods []corev1.Pod, rootUID types.UID) PodStats {
	var stats PodStats
	var nodes []string
	for _, pod := range a.Filter(ctx, pods, rootUID) {
		stats.PodsTotal++
		stats.RequestedCPU += podResource(pod.Spec, corev1.ResourceCPU)
		stats.RequestedMem += podResource(pod.Spec, corev1.ResourceMemory)
		stats.Restarts += restarts(pod.Status)

		created := pod.CreationTimestamp.Time
		if stats.OldestPod.IsZero() || created.Before(stats.OldestPod) {
			stats.OldestPod = created
		}
		if created.After(stats.NewestPod) {
			stats.NewestPod = created
		}
		if pod.Spec.NodeName != "" {
			nodes = append(nodes, pod.Spec.NodeName)
		}

		if pod.Status.Phase == corev1.PodRunning {
			stats.PodsRunning++
			stats.AllocatedGPUs += podGPUs(pod.Spec)
			stats.AllocatedCPU += podResource(pod.Spec, corev1.ResourceCPU)
			stats.AllocatedMem += podResource(pod.Spec, corev1.ResourceMemory)
			continue
		}
		if stats.PendingReason == "" {
			stats.PendingReason = pendingReason(pod.Status)
		}
	}

	slices.Sort(nodes)
	stats.Nodes = slices.Compact(nodes)
	return stats
}

// EnrichStats folds a physical snapshot into an already-computed PodStats,
// annotating it with node health, topology domain and device identity for
// pods (expected to already be Filter()-ed to one workload). It is a
// separate pass, mirroring workload.EnrichTree, so a caller with no
// node/ResourceClaim read permission or one that only wants spec-derived
// stats is unaffected.
func EnrichStats(stats *PodStats, pods []corev1.Pod, namespace string, snap *physical.Snapshot) {
	if snap == nil {
		return
	}
	degraded := map[string]struct{}{}
	domains := map[string]struct{}{}
	var devices []string
	var deviceGPUs int64

	for _, pod := range pods {
		if facts, ok := snap.NodeFor(pod.Spec.NodeName); ok {
			if !facts.Healthy() {
				degraded[facts.Name] = struct{}{}
			}
			if facts.Domain != "" {
				domains[facts.Domain] = struct{}{}
			}
		}
		podDevices := snap.DevicesFor(namespace, pod.Name)
		for _, d := range podDevices {
			devices = append(devices, physical.FormatDevices([]physical.Device{d}, 0))
		}
		if pod.Status.Phase == corev1.PodRunning && len(podDevices) > 0 {
			deviceGPUs += int64(len(podDevices))
		}
	}

	stats.DeviceCount = len(devices)
	stats.Devices = devices
	stats.DegradedNodes = sortedSetKeys(degraded)
	stats.Domains = sortedSetKeys(domains)
	// A DRA pod requests devices through spec.resourceClaims, not through the
	// nvidia.com/gpu extended resource, so the spec-derived count is zero for
	// every GPU workload on a DRA cluster. The allocation result is the only
	// truthful count available in that case.
	if stats.AllocatedGPUs == 0 && deviceGPUs > 0 {
		stats.AllocatedGPUs = deviceGPUs
	}
}

// pendingReason reports the first scheduling failure on pod, empty when it is
// merely waiting on something routine like an image pull.
func pendingReason(status corev1.PodStatus) string {
	for _, condition := range status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			if condition.Message != "" {
				return condition.Message
			}
			return condition.Reason
		}
	}
	if status.Phase == corev1.PodFailed && status.Reason != "" {
		return status.Reason
	}
	return ""
}

// restarts sums every container's restart count, matching what `kubectl get
// pods` reports in its RESTARTS column.
func restarts(status corev1.PodStatus) int32 {
	var total int32
	for _, container := range status.ContainerStatuses {
		total += container.RestartCount
	}
	return total
}

// podResource mirrors podGPUs' effective-request rule for an arbitrary
// resource: max(largest init container, running containers plus sidecars).
func podResource(spec corev1.PodSpec, name corev1.ResourceName) int64 {
	running := sumContainerResource(spec.Containers, name)

	var largestInit int64
	for _, container := range spec.InitContainers {
		value := resourceValue(container.Resources, name)
		if container.RestartPolicy != nil && *container.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			running += value
			continue
		}
		largestInit = max(largestInit, value)
	}

	return max(largestInit, running)
}

func sumContainerResource(containers []corev1.Container, name corev1.ResourceName) int64 {
	var total int64
	for _, container := range containers {
		total += resourceValue(container.Resources, name)
	}
	return total
}

// resourceValue falls back to limits, matching gpusIn: extended resources and
// unset requests are often declared there only.
func resourceValue(requirements corev1.ResourceRequirements, name corev1.ResourceName) int64 {
	quantity, ok := requirements.Requests[name]
	if !ok {
		quantity, ok = requirements.Limits[name]
	}
	if !ok {
		return 0
	}
	if name == corev1.ResourceCPU {
		return quantity.MilliValue()
	}
	return quantity.Value()
}

// belongsTo reports whether any owner in refs is, or transitively leads to,
// rootUID.
func (a *PodAttributor) belongsTo(
	ctx context.Context, refs []metav1.OwnerReference, namespace string, rootUID types.UID, depth int,
) bool {
	if depth >= maxOwnerDepth {
		return false
	}
	for _, ref := range refs {
		if types.UID(ref.UID) == rootUID {
			return true
		}
	}
	controller := controllerRef(refs)
	if controller == nil {
		return false
	}
	owner, err := a.get(ctx, *controller, namespace)
	if err != nil || owner == nil {
		return false
	}
	return a.belongsTo(ctx, owner.GetOwnerReferences(), namespace, rootUID, depth+1)
}

// controllerRef returns the single owner reference with Controller set, the
// only one an owner chain can be walked through unambiguously.
func controllerRef(refs []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range refs {
		if refs[i].Controller != nil && *refs[i].Controller {
			return &refs[i]
		}
	}
	return nil
}

func (a *PodAttributor) get(
	ctx context.Context, ref metav1.OwnerReference, namespace string,
) (*unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("parse owner apiVersion %q: %w", ref.APIVersion, err)
	}
	gvk := gv.WithKind(ref.Kind)
	key := ownerKey{gvk: gvk, namespace: namespace, name: ref.Name}
	if cached, ok := a.cache[key]; ok {
		return cached, nil
	}

	mapping, err := a.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", gvk.Kind, err)
	}
	obj, err := a.dyn.Resource(mapping.Resource).Namespace(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	a.cache[key] = obj // cache the miss too, so a broken chain is not re-fetched per pod
	if err != nil {
		return nil, err
	}
	return obj, nil
}
