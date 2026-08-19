// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// A recording round-trips through writeRecording/loadRecording: metadata, the ordered STATE states, and a
// performed action all survive the file.
func TestRecordingRoundTrips(t *testing.T) {
	rec := Recording{
		SchemaVersion: schemaVersion,
		Operator:      "batch-job",
		KartaName:     "batch-job-v1",
		Flow:          "resumed",
		Want:          string(v1alpha1.CompletedStatus),
		Result:        Result{Failures: []string{"watch lost its position"}},
		Events: []Event{
			{Kind: EventState, State: "Suspended", ResourceVersion: "101", Object: map[string]interface{}{
				"kind": "Job", "metadata": map[string]interface{}{"name": "j"},
				"status": map[string]interface{}{"active": float64(0)},
			}},
			{Kind: EventAction, Action: &RecordedAction{
				Name: "Resume",
				Operation: Operation{
					Verb: "PATCH", PatchType: "application/merge-patch+json",
					Payload: map[string]interface{}{"spec": map[string]interface{}{"suspend": false}},
				},
			}},
			{Kind: EventState, State: "Completed", StaleObservedGeneration: true, Object: map[string]interface{}{
				"kind": "Job", "metadata": map[string]interface{}{"name": "j"},
				"status": map[string]interface{}{"active": float64(0), "succeeded": float64(1)},
			}},
		},
	}

	path := filepath.Join(t.TempDir(), "op", "v1", "batch-job-v1", "resumed.yaml")
	if err := writeRecording(path, rec); err != nil {
		t.Fatal(err)
	}
	got, err := loadRecording(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operator != "batch-job" || got.Flow != "resumed" || len(got.Events) != 3 {
		t.Fatalf("recording round-trip mismatch: %+v", got)
	}
	if !reflect.DeepEqual(got.states(), []string{"Suspended", "Completed"}) {
		t.Errorf("states round-trip mismatch: %v", got.states())
	}
	act := got.Events[1].Action
	if act == nil || act.Name != "Resume" || act.Operation.Verb != "PATCH" {
		t.Fatalf("action did not round-trip: %+v", act)
	}
	if got.Events[0].StaleObservedGeneration || !got.Events[2].StaleObservedGeneration {
		t.Errorf("staleObservedGeneration flag did not round-trip: %v, %v", got.Events[0].StaleObservedGeneration, got.Events[2].StaleObservedGeneration)
	}
	if got.Result.Succeeded || len(got.Result.Failures) != 1 || got.Result.Failures[0] != "watch lost its position" || got.Events[0].ResourceVersion != "101" {
		t.Errorf("result/resourceVersion did not round-trip: %+v, %q", got.Result, got.Events[0].ResourceVersion)
	}
	if v, ok := act.Operation.Payload["spec"].(map[string]interface{})["suspend"]; !ok || v != false {
		t.Errorf("action payload did not round-trip: %+v", act.Operation.Payload)
	}
}

// TestReaderWalksStates walks a recording's STATE events, skipping the ACTION event between them.
func TestReaderWalksStates(t *testing.T) {
	rec := Recording{Events: []Event{
		{Kind: EventState, State: "Initializing", Object: map[string]interface{}{"status": map[string]interface{}{"active": float64(1)}}},
		{Kind: EventAction, Action: &RecordedAction{Name: "Scale"}},
		{Kind: EventState, State: "Running", Object: map[string]interface{}{"status": map[string]interface{}{"ready": float64(1)}}},
	}}

	r := newReader(rec)
	var states []string
	for r.Next() {
		states = append(states, r.State())
	}
	if !reflect.DeepEqual(states, []string{"Initializing", "Running"}) {
		t.Errorf("reader states = %v, want [Initializing Running]", states)
	}

	r2 := newReader(rec)
	r2.Next()
	r2.Next()
	if v, _, _ := unstructured.NestedFloat64(r2.Object().Object, "status", "ready"); v != 1 {
		t.Errorf("reader object[1] status.ready = %v, want 1", v)
	}
}
