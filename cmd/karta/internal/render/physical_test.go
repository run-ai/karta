// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"bytes"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/karta/cmd/karta/internal/physical"
)

func snapshot() *physical.Snapshot {
	return &physical.Snapshot{
		DRAAvailable: true,
		Nodes: map[string]physical.NodeFacts{
			"node-a": {Name: "node-a", Ready: true, Domain: "clq-1"},
			"node-b": {Name: "node-b", Ready: false, Domain: "clq-2"},
			"node-c": {Name: "node-c", Ready: true, Unschedulable: true, Domain: "clq-1"},
		},
		Devices: map[string][]physical.Device{
			physical.PodKey("ns", "worker-0"): {{Name: "gpu-0"}, {Name: "gpu-1"}},
			physical.PodKey("ns", "worker-1"): {{Name: "gpu-2"}},
		},
	}
}

func TestEnrichAnnotatesPodsAndRollsUp(t *testing.T) {
	view := WorkloadView{
		Namespace: "ns",
		Components: []ComponentView{{
			Name: "worker",
			Pods: []PodView{
				{Name: "worker-0", Node: "node-a"},
				{Name: "worker-1", Node: "node-b"},
			},
		}},
	}

	Enrich(&view, snapshot())
	c := view.Components[0]

	if got := c.Pods[0].NodeCondition; got != "" {
		t.Errorf("healthy pod NodeCondition = %q, want empty", got)
	}
	if got := c.Pods[1].NodeCondition; got != "NotReady" {
		t.Errorf("degraded pod NodeCondition = %q, want NotReady", got)
	}
	if got := c.Pods[0].Domain; got != "clq-1" {
		t.Errorf("pod Domain = %q, want clq-1", got)
	}
	if len(c.Pods[0].Devices) != 2 || len(c.Pods[1].Devices) != 1 {
		t.Errorf("device attachment wrong: %+v / %+v", c.Pods[0].Devices, c.Pods[1].Devices)
	}
	if c.DeviceCount != 3 {
		t.Errorf("DeviceCount = %d, want 3", c.DeviceCount)
	}
	if len(c.DegradedNodes) != 1 || c.DegradedNodes[0] != "node-b" {
		t.Errorf("DegradedNodes = %v, want [node-b]", c.DegradedNodes)
	}
	if !SplitAcrossDomains(c) {
		t.Errorf("expected component to report a domain split, domains=%v", c.Domains)
	}
}

func TestEnrichCordonedNodeCountsAsDegraded(t *testing.T) {
	// A cordoned node still runs its pods, so nothing in the logical view
	// changes. It is exactly the state a drain-impact question cares about.
	view := WorkloadView{
		Namespace:  "ns",
		Components: []ComponentView{{Name: "worker", Pods: []PodView{{Name: "worker-0", Node: "node-c"}}}},
	}
	Enrich(&view, snapshot())

	if got := view.Components[0].Pods[0].NodeCondition; got != "cordoned" {
		t.Errorf("NodeCondition = %q, want cordoned", got)
	}
	if len(view.Components[0].DegradedNodes) != 1 {
		t.Errorf("cordoned node did not roll up as degraded: %v", view.Components[0].DegradedNodes)
	}
}

func TestEnrichRollsUpThroughChildren(t *testing.T) {
	view := WorkloadView{
		Namespace: "ns",
		Components: []ComponentView{{
			Name: "group",
			Children: []ComponentView{
				{Name: "leader", Pods: []PodView{{Name: "worker-0", Node: "node-a"}}},
				{Name: "worker", Pods: []PodView{{Name: "worker-1", Node: "node-b"}}},
			},
		}},
	}

	Enrich(&view, snapshot())
	parent := view.Components[0]

	if parent.DeviceCount != 3 {
		t.Errorf("parent DeviceCount = %d, want 3", parent.DeviceCount)
	}
	if len(parent.DegradedNodes) != 1 || parent.DegradedNodes[0] != "node-b" {
		t.Errorf("parent DegradedNodes = %v, want [node-b]", parent.DegradedNodes)
	}
	if !SplitAcrossDomains(parent) {
		t.Errorf("parent should report the split across its children, domains=%v", parent.Domains)
	}
}

func TestEnrichUnresolvedNodeIsInert(t *testing.T) {
	// The common case when the user cannot read nodes: the snapshot is empty,
	// and the tree must render exactly as it did before.
	view := WorkloadView{
		Namespace:  "ns",
		Components: []ComponentView{{Name: "worker", Pods: []PodView{{Name: "worker-0", Node: "node-z"}}}},
	}
	Enrich(&view, &physical.Snapshot{Nodes: map[string]physical.NodeFacts{}, Devices: map[string][]physical.Device{}})

	c := view.Components[0]
	if c.Pods[0].NodeCondition != "" || c.Pods[0].Domain != "" || len(c.DegradedNodes) != 0 {
		t.Errorf("unresolved node produced annotations: %+v", c)
	}
}

func TestEnrichNilSnapshotIsSafe(t *testing.T) {
	view := WorkloadView{Namespace: "ns", Components: []ComponentView{{Name: "worker"}}}
	Enrich(&view, nil)
	if view.Components[0].DeviceCount != 0 {
		t.Errorf("nil snapshot mutated the view")
	}
}

func TestTreePodsNarrowsToTheTree(t *testing.T) {
	view := WorkloadView{
		Namespace: "ns",
		Components: []ComponentView{{
			Name:     "group",
			Pods:     []PodView{{Name: "worker-0"}},
			Children: []ComponentView{{Name: "leader", Pods: []PodView{{Name: "worker-1"}}}},
		}},
	}
	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "worker-0", Namespace: "ns"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Namespace: "ns"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "ns"}},
	}

	got := TreePods(view, pods)
	if len(got) != 2 {
		t.Fatalf("TreePods returned %d pods, want 2: %+v", len(got), got)
	}
	for _, p := range got {
		if p.Name == "unrelated" {
			t.Errorf("TreePods included a pod outside the tree")
		}
	}
}

func TestTreeRendersPhysicalAnnotations(t *testing.T) {
	view := WorkloadView{
		Kind:      "PyTorchJob",
		Name:      "demo",
		Namespace: "ns",
		Components: []ComponentView{{
			Name:            "worker",
			DesiredReplicas: 2,
			CurrentReplicas: 2,
			ReadyCount:      2,
			GPUs:            3,
			Nodes:           []string{"node-a", "node-b"},
			Pods: []PodView{
				{Name: "worker-0", Phase: "Running", Ready: true, Node: "node-a", GPUs: 2},
				{Name: "worker-1", Phase: "Running", Ready: true, Node: "node-b", GPUs: 1},
			},
		}},
	}
	Enrich(&view, snapshot())

	var buf bytes.Buffer
	if err := Tree(&buf, view, PlainStyle()); err != nil {
		t.Fatalf("Tree: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"!NotReady",        // the degraded pod's node
		"!node-b degraded", // rolled up onto the component row
		"dev: gpu-0,gpu-1", // per-pod device identity
		"dev: 3",           // component device roll-up
		"@clq-1",           // topology domain
		"(split)",          // gang crossing a domain boundary
	} {
		if !strings.Contains(out, want) {
			t.Errorf("tree output missing %q\n%s", want, out)
		}
	}
}

func TestTreeWithoutEnrichIsUnchanged(t *testing.T) {
	// Guard the default path: without --physical the trailing column must stay
	// the bare node list it has always been.
	view := WorkloadView{
		Kind: "PyTorchJob", Name: "demo", Namespace: "ns",
		Components: []ComponentView{{
			Name: "worker", DesiredReplicas: 1, CurrentReplicas: 1, ReadyCount: 1,
			Nodes: []string{"node-a"},
			Pods:  []PodView{{Name: "worker-0", Phase: "Running", Ready: true, Node: "node-a"}},
		}},
	}

	var buf bytes.Buffer
	if err := Tree(&buf, view, PlainStyle()); err != nil {
		t.Fatalf("Tree: %v", err)
	}
	out := buf.String()

	for _, unwanted := range []string{"!", "@", "dev:", "(split)"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("un-enriched tree leaked %q\n%s", unwanted, out)
		}
	}
}

func TestEnrichRecoversGPUCountFromDRA(t *testing.T) {
	// On a DRA cluster a GPU pod carries no nvidia.com/gpu resource, so the
	// spec-derived count is 0 for every GPU workload. The allocation result is
	// the only truthful count.
	view := WorkloadView{
		Namespace: "ns",
		Components: []ComponentView{{
			Name: "worker",
			Pods: []PodView{
				{Name: "worker-0", Node: "node-a", GPUs: 0},
				{Name: "worker-1", Node: "node-a", GPUs: 0},
			},
		}},
	}

	Enrich(&view, snapshot())
	c := view.Components[0]

	if c.Pods[0].GPUs != 2 || c.Pods[1].GPUs != 1 {
		t.Errorf("pod GPU recovery = %d/%d, want 2/1", c.Pods[0].GPUs, c.Pods[1].GPUs)
	}
	if c.GPUs != 3 {
		t.Errorf("component GPUs = %d, want 3", c.GPUs)
	}
}

func TestEnrichDoesNotOverrideSpecGPUCount(t *testing.T) {
	// A classic device-plugin pod already reports a real count. DRA devices
	// must not double it.
	view := WorkloadView{
		Namespace:  "ns",
		Components: []ComponentView{{Name: "worker", GPUs: 8, Pods: []PodView{{Name: "worker-0", Node: "node-a", GPUs: 8}}}},
	}
	Enrich(&view, snapshot())

	if got := view.Components[0].Pods[0].GPUs; got != 8 {
		t.Errorf("pod GPUs = %d, want 8 (spec count preserved)", got)
	}
	if got := view.Components[0].GPUs; got != 8 {
		t.Errorf("component GPUs = %d, want 8", got)
	}
}

func TestEnrichGPURecoveryRollsUpThroughChildren(t *testing.T) {
	view := WorkloadView{
		Namespace: "ns",
		Components: []ComponentView{{
			Name: "group",
			Children: []ComponentView{
				{Name: "leader", Pods: []PodView{{Name: "worker-0", Node: "node-a"}}},
				{Name: "worker", Pods: []PodView{{Name: "worker-1", Node: "node-b"}}},
			},
		}},
	}
	Enrich(&view, snapshot())

	if got := view.Components[0].GPUs; got != 3 {
		t.Errorf("parent GPUs = %d, want 3", got)
	}
}
