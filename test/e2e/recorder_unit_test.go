// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// These are plain unit tests for the pure recorder helpers; they do not touch the
// cluster (RunSpecs is not invoked), so `go test -run TestClassify` runs offline.

func obj(status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"status": status}}
}

func TestClassifyPicksMostAdvancedState(t *testing.T) {
	states := []namedState{
		{"Running", intAtLeast(1, "status", "active")},
		{"Completed", condTrue("Complete")},
	}

	if got := classify(obj(map[string]any{"active": int64(1)}), states); got != "Running" {
		t.Errorf("active=1 -> %q, want Running", got)
	}
	// Both predicates match; the last (most advanced) wins.
	completed := obj(map[string]any{
		"active":     int64(1),
		"conditions": []any{map[string]any{"type": "Complete", "status": "True"}},
	})
	if got := classify(completed, states); got != "Completed" {
		t.Errorf("Complete=True -> %q, want Completed", got)
	}
	if got := classify(obj(map[string]any{}), states); got != "" {
		t.Errorf("no match -> %q, want empty", got)
	}
}

func TestReplicasDegraded(t *testing.T) {
	deg := replicasDegraded()
	cases := []struct {
		name                     string
		replicas, ready, updated int64
		want                     bool
	}{
		{"settled degraded", 2, 1, 2, true},
		{"mid-rollout, replica not yet created", 2, 1, 1, false},
		{"fully ready", 2, 2, 2, false},
		{"none ready", 2, 0, 2, false},
	}
	for _, c := range cases {
		got := deg(obj(map[string]any{
			"replicas":        c.replicas,
			"readyReplicas":   c.ready,
			"updatedReplicas": c.updated,
		}))
		if got != c.want {
			t.Errorf("%s: replicas=%d ready=%d updated=%d -> %v, want %v", c.name, c.replicas, c.ready, c.updated, got, c.want)
		}
	}
	if deg(obj(map[string]any{})) {
		t.Error("empty status should not be degraded")
	}
}

func TestStatusSettled(t *testing.T) {
	withGen := func(gen int64, obs *int64) *unstructured.Unstructured {
		status := map[string]any{}
		if obs != nil {
			status["observedGeneration"] = *obs
		}
		return &unstructured.Unstructured{Object: map[string]any{
			"metadata": map[string]any{"generation": gen},
			"status":   status,
		}}
	}
	two := int64(2)
	if !statusSettled(withGen(2, &two)) {
		t.Error("observedGeneration == generation should be settled")
	}
	if statusSettled(withGen(3, &two)) {
		t.Error("observedGeneration < generation should NOT be settled")
	}
	if !statusSettled(withGen(2, nil)) {
		t.Error("missing observedGeneration should be settled (gate is a no-op)")
	}
}
