// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package reader

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/test/e2e/recorder"
)

// TestReaderIterates walks a recording step by step and reconstructs the CR at each step.
func TestReaderIterates(t *testing.T) {
	rec := recorder.Recording{Steps: []recorder.Step{
		{State: "Initializing", CR: map[string]interface{}{"status": map[string]interface{}{"active": float64(1)}}},
		{State: "Running", Patch: map[string]interface{}{"status": map[string]interface{}{"ready": float64(1)}}},
	}}

	r, err := New(rec)
	if err != nil {
		t.Fatal(err)
	}
	var states []string
	for r.Next() {
		states = append(states, r.Step().State)
	}
	if !reflect.DeepEqual(states, []string{"Initializing", "Running"}) {
		t.Errorf("reader states = %v", states)
	}

	r2, _ := New(rec)
	r2.Next()
	r2.Next()
	if v, _, _ := unstructured.NestedFloat64(r2.CR().Object, "status", "ready"); v != 1 {
		t.Errorf("reader CR[1] status.ready = %v, want 1", v)
	}
}
