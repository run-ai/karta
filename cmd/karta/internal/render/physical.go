// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"sort"

	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/cmd/karta/internal/physical"
)

// maxDevicesPerRow caps how many device names a single tree row prints before
// collapsing the tail into "+N more". Eight is one full node's worth on the
// common GPU SKUs, which keeps a per-pod row readable.
const maxDevicesPerRow = 8

// Enrich folds a physical snapshot into an already-built WorkloadView,
// annotating pods with their node condition, topology domain, and allocated
// devices, then rolling those up to each component.
//
// It is a separate pass rather than a parameter to Build so that callers that
// only want the logical tree (the list command, tests, any consumer that has
// no node read permission) are unaffected.
func Enrich(view *WorkloadView, snap *physical.Snapshot) {
	if view == nil || snap == nil {
		return
	}
	for i := range view.Components {
		enrichComponent(&view.Components[i], view.Namespace, snap)
	}
}

// enrichComponent annotates one component and returns the GPU count it
// recovered from DRA, so parents can fold it into their own roll-up.
func enrichComponent(c *ComponentView, namespace string, snap *physical.Snapshot) int64 {
	degraded := map[string]string{}
	domains := map[string]struct{}{}
	deviceCount := 0
	var gpuDelta int64

	for i := range c.Pods {
		p := &c.Pods[i]
		if facts, ok := snap.NodeFor(p.Node); ok {
			p.NodeCondition = facts.Condition()
			p.Domain = facts.Domain
			if !facts.Healthy() {
				degraded[facts.Name] = facts.Condition()
			}
			if facts.Domain != "" {
				domains[facts.Domain] = struct{}{}
			}
		}
		p.Devices = snap.DevicesFor(namespace, p.Name)
		deviceCount += len(p.Devices)

		// A DRA pod requests devices through spec.resourceClaims, not through
		// the nvidia.com/gpu extended resource, so the count derived from the
		// pod spec is zero for every GPU workload on a DRA cluster. The
		// allocation result is the only truthful count available, so use it
		// when the spec-derived one has nothing to say.
		if p.GPUs == 0 && len(p.Devices) > 0 {
			p.GPUs = int64(len(p.Devices))
			gpuDelta += p.GPUs
		}
	}

	for i := range c.Children {
		child := &c.Children[i]
		gpuDelta += enrichComponent(child, namespace, snap)
		deviceCount += child.DeviceCount
		for _, n := range child.DegradedNodes {
			degraded[n] = child.degradedConditions[n]
		}
		for _, d := range child.Domains {
			domains[d] = struct{}{}
		}
	}

	c.GPUs += gpuDelta
	c.DeviceCount = deviceCount
	c.degradedConditions = degraded
	c.DegradedNodes = sortedMapKeys(degraded)
	c.Domains = sortedSetKeys(domains)
	return gpuDelta
}

func sortedMapKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedSetKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TreePods narrows a namespace-wide pod list to the pods that actually appear
// in a rendered view. The loader lists every pod in the namespace so the tree
// builder can match them itself; the physical resolver wants only the matched
// subset, since it reads one node per distinct pod placement.
func TreePods(view WorkloadView, pods []corev1.Pod) []corev1.Pod {
	names := map[string]struct{}{}
	var walk func(c ComponentView)
	walk = func(c ComponentView) {
		for _, p := range c.Pods {
			names[p.Name] = struct{}{}
		}
		for _, child := range c.Children {
			walk(child)
		}
	}
	for _, c := range view.Components {
		walk(c)
	}

	out := make([]corev1.Pod, 0, len(names))
	for i := range pods {
		if _, ok := names[pods[i].Name]; ok {
			out = append(out, pods[i])
		}
	}
	return out
}

// SplitAcrossDomains reports whether a component's pods landed in more than
// one topology domain. For a gang-scheduled component this is the interesting
// case: the workload is running, every pod is Ready, and the logical view
// looks perfect, while the collective is crossing a domain boundary.
func SplitAcrossDomains(c ComponentView) bool { return len(c.Domains) > 1 }
