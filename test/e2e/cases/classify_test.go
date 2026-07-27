// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func obj(status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"status": status}}
}

func TestClassifyPicksMostAdvancedState(t *testing.T) {
	states := []NamedState{
		{running, IntAtLeast(1, "status", "active")},
		{completed, CondTrue("Complete")},
	}
	if got := Classify(obj(map[string]any{"active": int64(1)}), states); got != running {
		t.Errorf("active=1 -> %q, want %q", got, running)
	}
	both := obj(map[string]any{"active": int64(1), "conditions": []any{map[string]any{"type": "Complete", "status": "True"}}})
	if got := Classify(both, states); got != completed {
		t.Errorf("Complete=True -> %q, want %q", got, completed)
	}
	if got := Classify(obj(map[string]any{}), states); got != "" {
		t.Errorf("no match -> %q, want empty", got)
	}
}
