// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func objWithStatus(status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"status": status}}
}

func TestClassifyPicksMostAdvancedState(t *testing.T) {
	states := []NamedState{
		{Running, IntAtLeast(1, "status", "active")},
		{Completed, CondTrue("Complete")},
	}
	if got := Classify(objWithStatus(map[string]any{"active": int64(1)}), states); got != Running {
		t.Errorf("active=1 -> %q, want %q", got, Running)
	}
	both := objWithStatus(map[string]any{"active": int64(1), "conditions": []any{map[string]any{"type": "Complete", "status": "True"}}})
	if got := Classify(both, states); got != Completed {
		t.Errorf("Complete=True -> %q, want %q", got, Completed)
	}
	if got := Classify(objWithStatus(map[string]any{}), states); got != "" {
		t.Errorf("no match -> %q, want empty", got)
	}
}
