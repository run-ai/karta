// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package physical

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNodeFacts(t *testing.T) {
	cases := []struct {
		name      string
		node      *corev1.Node
		wantOK    bool
		wantCond  string
		wantDom   string
		wantLabel string
	}{
		{
			name:     "ready node is healthy",
			node:     node("n1", corev1.ConditionTrue, false, nil),
			wantOK:   true,
			wantCond: "",
		},
		{
			name:     "not ready",
			node:     node("n1", corev1.ConditionFalse, false, nil),
			wantOK:   false,
			wantCond: "NotReady",
		},
		{
			// A kubelet that stopped reporting shows Unknown, which is a
			// failure for a running workload just as much as an explicit False.
			name:     "unknown counts as not ready",
			node:     node("n1", corev1.ConditionUnknown, false, nil),
			wantOK:   false,
			wantCond: "NotReady",
		},
		{
			name:     "cordoned but ready",
			node:     node("n1", corev1.ConditionTrue, true, nil),
			wantOK:   false,
			wantCond: "cordoned",
		},
		{
			// NotReady is the more severe signal and should win the label.
			name:     "not ready and cordoned reports NotReady",
			node:     node("n1", corev1.ConditionFalse, true, nil),
			wantOK:   false,
			wantCond: "NotReady",
		},
		{
			name:      "clique label wins over zone",
			node:      node("n1", corev1.ConditionTrue, false, map[string]string{"nvidia.com/gpu.clique": "clq-a", "topology.kubernetes.io/zone": "z1"}),
			wantOK:    true,
			wantDom:   "clq-a",
			wantLabel: "nvidia.com/gpu.clique",
		},
		{
			name:      "zone used when clique absent",
			node:      node("n1", corev1.ConditionTrue, false, map[string]string{"topology.kubernetes.io/zone": "z1"}),
			wantOK:    true,
			wantDom:   "z1",
			wantLabel: "topology.kubernetes.io/zone",
		},
		{
			name:    "empty label value is ignored",
			node:    node("n1", corev1.ConditionTrue, false, map[string]string{"nvidia.com/gpu.clique": ""}),
			wantOK:  true,
			wantDom: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nodeFacts(tc.node, DefaultTopologyLabels)
			if got.Healthy() != tc.wantOK {
				t.Errorf("Healthy() = %v, want %v", got.Healthy(), tc.wantOK)
			}
			if got.Condition() != tc.wantCond {
				t.Errorf("Condition() = %q, want %q", got.Condition(), tc.wantCond)
			}
			if got.Domain != tc.wantDom {
				t.Errorf("Domain = %q, want %q", got.Domain, tc.wantDom)
			}
			if tc.wantLabel != "" && got.DomainLabel != tc.wantLabel {
				t.Errorf("DomainLabel = %q, want %q", got.DomainLabel, tc.wantLabel)
			}
		})
	}
}

func TestNodeFactsNoReadyCondition(t *testing.T) {
	// A node object with no Ready condition at all (freshly registered, or a
	// mock) must not read as healthy.
	n := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n1"}}
	if got := nodeFacts(n, DefaultTopologyLabels); got.Healthy() {
		t.Errorf("node without Ready condition reported healthy")
	}
}

func TestAllocatedDevices(t *testing.T) {
	claim := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "resource.k8s.io/v1",
		"kind":       "ResourceClaim",
		"metadata":   map[string]any{"name": "claim-0", "namespace": "ns"},
		"status": map[string]any{
			"allocation": map[string]any{
				"devices": map[string]any{
					"results": []any{
						map[string]any{"driver": "gpu.nvidia.com", "pool": "node-1", "device": "gpu-1"},
						map[string]any{"driver": "gpu.nvidia.com", "pool": "node-1", "device": "gpu-0"},
						// A result with no device name is not addressable and
						// must be dropped rather than rendered as an empty id.
						map[string]any{"driver": "gpu.nvidia.com", "pool": "node-1"},
					},
				},
			},
		},
	}}

	got := allocatedDevices(claim)
	if len(got) != 2 {
		t.Fatalf("got %d devices, want 2: %+v", len(got), got)
	}
	if got[0].Name != "gpu-1" || got[0].Driver != "gpu.nvidia.com" || got[0].Pool != "node-1" {
		t.Errorf("unexpected first device: %+v", got[0])
	}
}

func TestAllocatedDevicesUnallocatedClaim(t *testing.T) {
	// A pending claim has no allocation stanza yet. That is the normal state
	// between pod creation and scheduling, not an error.
	claim := &unstructured.Unstructured{Object: map[string]any{
		"kind":     "ResourceClaim",
		"metadata": map[string]any{"name": "claim-0"},
		"status":   map[string]any{},
	}}
	if got := allocatedDevices(claim); len(got) != 0 {
		t.Errorf("got %d devices from an unallocated claim, want 0", len(got))
	}
}

func TestFormatDevices(t *testing.T) {
	devices := []Device{{Name: "gpu-0"}, {Name: "gpu-1"}, {Name: "gpu-2"}}

	if got := FormatDevices(devices, 8); got != "gpu-0,gpu-1,gpu-2" {
		t.Errorf("FormatDevices under cap = %q", got)
	}
	if got := FormatDevices(devices, 2); got != "gpu-0,gpu-1,+1 more" {
		t.Errorf("FormatDevices over cap = %q", got)
	}
	if got := FormatDevices(nil, 8); got != "" {
		t.Errorf("FormatDevices(nil) = %q, want empty", got)
	}
	// The cap must not mutate the caller's slice.
	if len(devices) != 3 {
		t.Errorf("FormatDevices mutated its input: %+v", devices)
	}
}

func TestSnapshotAccessorsOnNil(t *testing.T) {
	// The command path passes whatever Resolve returned straight into the
	// renderer, so the accessors must tolerate a nil snapshot.
	var snap *Snapshot
	if got := snap.DevicesFor("ns", "pod"); got != nil {
		t.Errorf("DevicesFor on nil snapshot = %+v", got)
	}
	if _, ok := snap.NodeFor("n1"); ok {
		t.Errorf("NodeFor on nil snapshot reported ok")
	}
}

func TestSnapshotNodeForEmptyName(t *testing.T) {
	// An unscheduled pod has no node name; that must not resolve.
	snap := &Snapshot{Nodes: map[string]NodeFacts{"": {Name: "phantom", Ready: true}}}
	if _, ok := snap.NodeFor(""); ok {
		t.Errorf("NodeFor(\"\") resolved")
	}
}

func node(name string, ready corev1.ConditionStatus, cordoned bool, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       corev1.NodeSpec{Unschedulable: cordoned},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: ready}},
		},
	}
}
