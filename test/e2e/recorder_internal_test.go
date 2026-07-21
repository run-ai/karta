// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Plain unit tests for the pure recorder helpers; no cluster (RunSpecs is not invoked).

func obj(status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"status": status}}
}

// TestRecorderCatchesBackwardsJump reproduces the JobSet bug: Running returns to a byte-identical
// Initializing before Completed. Consecutive dedup keeps the return, and the strict order check
// flags it unless the journey declares the dip (or the flow sets mayGoBackwards).
func TestRecorderCatchesBackwardsJump(t *testing.T) {
	states := []namedState{
		{initializing, jobsetInitializing()},
		{running, jobsetRunning()},
		{completed, condTrue("Completed")},
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
		rec.keep(u, classify(u, states))
	}

	want := []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}
	if !reflect.DeepEqual(rec.order, want) {
		t.Fatalf("recorder dropped the return: got %v, want %v", rec.order, want)
	}
	if observedOrderErr(flow{journey: steps(initializing, running, completed)}, rec.order) == nil {
		t.Error("strict journey should reject the undeclared Running -> Initializing dip")
	}
	if err := observedOrderErr(flow{journey: steps(initializing, running, completed), mayGoBackwards: true}, rec.order); err != nil {
		t.Errorf("mayGoBackwards should accept the dip, got %v", err)
	}
}

func TestClassifyPicksMostAdvancedState(t *testing.T) {
	states := []namedState{
		{running, intAtLeast(1, "status", "active")},
		{completed, condTrue("Complete")},
	}
	if got := classify(obj(map[string]any{"active": int64(1)}), states); got != running {
		t.Errorf("active=1 -> %q, want %q", got, running)
	}
	both := obj(map[string]any{"active": int64(1), "conditions": []any{map[string]any{"type": "Complete", "status": "True"}}})
	if got := classify(both, states); got != completed {
		t.Errorf("Complete=True -> %q, want %q", got, completed)
	}
	if got := classify(obj(map[string]any{}), states); got != "" {
		t.Errorf("no match -> %q, want empty", got)
	}
}

func TestObservedOrder(t *testing.T) {
	cases := []struct {
		name     string
		fl       flow
		observed []kartav1alpha1.ResourceStatus
		ok       bool
	}{
		{"exact", flow{journey: steps(initializing, running, completed)}, []kartav1alpha1.ResourceStatus{initializing, running, completed}, true},
		{"skip a step", flow{journey: steps(initializing, running, completed)}, []kartav1alpha1.ResourceStatus{initializing, completed}, true},
		{"undeclared state", flow{journey: steps(initializing, running)}, []kartav1alpha1.ResourceStatus{initializing, failed}, false},
		{"backwards not declared", flow{journey: steps(initializing, running, completed)}, []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}, false},
		{"backwards declared", flow{journey: steps(initializing, running, initializing, completed)}, []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}, true},
		{"wrong terminal", flow{journey: steps(initializing, running, completed)}, []kartav1alpha1.ResourceStatus{initializing, running}, false},
		{"mayGoBackwards", flow{journey: steps(initializing, running, completed), mayGoBackwards: true}, []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}, true},
	}
	for _, c := range cases {
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
	for _, tc := range allCases {
		if err := tc.validate(); err != nil {
			t.Errorf("%s: %v", tc.name, err)
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
