// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestTreeRendering(t *testing.T) {
	view := WorkloadView{
		Kind: "PyTorchJob",
		Name: "demo",
		Phases: []string{"Running"},
		Components: []ComponentView{
			{
				Name:           "master",
				DesiredReplicas: 1,
				CurrentReplicas: 1,
				ReadyCount:     1,
				GPUs:           1,
				Nodes:          []string{"node-01"},
				Pods: []PodView{
					{Name: "demo-master-0", Phase: "Running", Ready: true, Node: "node-01", GPUs: 1},
				},
			},
			{
				Name:           "worker",
				DesiredReplicas: 4,
				CurrentReplicas: 4,
				ReadyCount:     3,
				GPUs:           32,
				Nodes:          []string{"node-02", "node-03", "node-04"},
				Pods: []PodView{
					{Name: "demo-worker-0", Phase: "Running", Ready: true, Node: "node-02", GPUs: 8},
					{Name: "demo-worker-1", Phase: "Running", Ready: true, Node: "node-03", GPUs: 8},
					{Name: "demo-worker-2", Phase: "Running", Ready: true, Node: "node-04", GPUs: 8},
					{Name: "demo-worker-3", Phase: "Pending", Node: "", GPUs: 8},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := Tree(&buf, view, PlainStyle()); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()

	wants := []string{
		"PyTorchJob/demo [Running]",
		// master/worker share name width (6); gpu column aligns to "gpu: 32" (width 7)
		"├── master  (1/1 replicas)  1/1 ready  gpu: 1   nodes: node-01",
		"└── worker  (4/4 replicas)  3/4 ready  gpu: 32  nodes: node-02,node-03,node-04",
		// pods within master: single row, no padding pressure
		"│   └── Pod/demo-master-0  Running  gpu: 1  node-01",
		// pods within worker: all 17 chars wide, aligned
		"    └── Pod/demo-worker-3  Pending  gpu: 8  <none>",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("expected output to contain %q, got:\n%s", w, got)
		}
	}
}

func TestPhasesString(t *testing.T) {
	if got := PhasesString(nil); got != "-" {
		t.Errorf("empty phases want \"-\", got %q", got)
	}
	if got := PhasesString([]string{"Running"}); got != "Running" {
		t.Errorf("single phase want \"Running\", got %q", got)
	}
	if got := PhasesString([]string{"Running", "Degraded"}); got != "Running,Degraded" {
		t.Errorf("multi phase want \"Running,Degraded\", got %q", got)
	}
}

func TestFormatNodes(t *testing.T) {
	if got := FormatNodes(nil); got != "<none>" {
		t.Errorf("empty nodes want <none>, got %q", got)
	}
	if got := FormatNodes([]string{"a", "b"}); got != "a,b" {
		t.Errorf("nodes want a,b, got %q", got)
	}
}
