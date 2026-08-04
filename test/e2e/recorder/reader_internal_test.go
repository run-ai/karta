// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestReaderWalksStates walks a recording's STATE events, skipping the ACTION event between them.
func TestReaderWalksStates(t *testing.T) {
	rec := Recording{Events: []Event{
		{Kind: EventState, State: "Initializing", Object: map[string]interface{}{"status": map[string]interface{}{"active": float64(1)}}},
		{Kind: EventAction, Action: &RecordedAction{Name: "Scale"}},
		{Kind: EventState, State: "Running", Object: map[string]interface{}{"status": map[string]interface{}{"ready": float64(1)}}},
	}}

	r := NewReader(rec)
	var states []string
	for r.Next() {
		states = append(states, r.State())
	}
	if !reflect.DeepEqual(states, []string{"Initializing", "Running"}) {
		t.Errorf("reader states = %v, want [Initializing Running]", states)
	}

	r2 := NewReader(rec)
	r2.Next()
	r2.Next()
	if v, _, _ := unstructured.NestedFloat64(r2.Object().Object, "status", "ready"); v != 1 {
		t.Errorf("reader object[1] status.ready = %v, want 1", v)
	}
}
