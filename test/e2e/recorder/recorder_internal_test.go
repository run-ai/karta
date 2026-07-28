// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/e2e/cases"
)

// Plain unit tests for the pure recorder helpers; no cluster (RunSpecs is not invoked).

func obj(status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"status": status}}
}

// TestRecorderCatchesBackwardsJump reproduces the JobSet bug: Running returns to a byte-identical
// Initializing before Completed. Consecutive dedup keeps the return, and the strict order check
// flags it unless the journey declares the dip explicitly.
func TestRecorderCatchesBackwardsJump(t *testing.T) {
	initializing := kartav1alpha1.InitializingStatus
	running := kartav1alpha1.RunningStatus
	completed := kartav1alpha1.CompletedStatus

	states := []cases.NamedState{
		{Name: initializing, Match: cases.JobsetInitializing()},
		{Name: running, Match: cases.JobsetRunning()},
		{Name: completed, Match: cases.CondTrue("Completed")},
	}
	initCR := func() *unstructured.Unstructured {
		return obj(map[string]any{"replicatedJobsStatus": []any{map[string]any{"active": int64(1), "ready": int64(0)}}})
	}
	seq := []*unstructured.Unstructured{
		initCR(),
		obj(map[string]any{"replicatedJobsStatus": []any{map[string]any{"active": int64(1), "ready": int64(1)}}}),
		initCR(),
		obj(map[string]any{"conditions": []any{map[string]any{"type": "Completed", "status": "True"}}}),
	}

	rec := &recording{}
	for _, u := range seq {
		rec.keep(u, cases.Classify(u, states))
	}

	want := []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}
	if !reflect.DeepEqual(rec.order, want) {
		t.Fatalf("recorder dropped the return: got %v, want %v", rec.order, want)
	}
	if observedOrderErr(cases.Flow{Journey: cases.Steps(initializing, running, completed)}, rec.order) == nil {
		t.Error("strict journey should reject the undeclared Running -> Initializing dip")
	}
	if err := observedOrderErr(cases.Flow{Journey: cases.Steps(initializing, running, initializing, completed)}, rec.order); err != nil {
		t.Errorf("declaring the Initializing revisit should accept the dip, got %v", err)
	}
}

func TestObservedOrder(t *testing.T) {
	initializing := kartav1alpha1.InitializingStatus
	running := kartav1alpha1.RunningStatus
	completed := kartav1alpha1.CompletedStatus
	failed := kartav1alpha1.FailedStatus

	tests := []struct {
		name     string
		fl       cases.Flow
		observed []kartav1alpha1.ResourceStatus
		ok       bool
	}{
		{"exact", cases.Flow{Journey: cases.Steps(initializing, running, completed)}, []kartav1alpha1.ResourceStatus{initializing, running, completed}, true},
		{"skip a step", cases.Flow{Journey: cases.Steps(initializing, running, completed)}, []kartav1alpha1.ResourceStatus{initializing, completed}, true},
		{"undeclared state", cases.Flow{Journey: cases.Steps(initializing, running)}, []kartav1alpha1.ResourceStatus{initializing, failed}, false},
		{"backwards not declared", cases.Flow{Journey: cases.Steps(initializing, running, completed)}, []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}, false},
		{"backwards declared", cases.Flow{Journey: cases.Steps(initializing, running, initializing, completed)}, []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}, true},
		{"wrong terminal", cases.Flow{Journey: cases.Steps(initializing, running, completed)}, []kartav1alpha1.ResourceStatus{initializing, running}, false},
	}
	for _, c := range tests {
		err := observedOrderErr(c.fl, c.observed)
		if c.ok && err != nil {
			t.Errorf("%s: want ok, got %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

func TestCasesValid(t *testing.T) {
	for _, tc := range cases.All {
		if err := tc.Validate(); err != nil {
			t.Errorf("%s: %v", tc.Name, err)
		}
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
		t.Error("missing observedGeneration should be settled")
	}
}
