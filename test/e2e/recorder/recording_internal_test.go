// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// A recording round-trips through WriteRecording/LoadRecording: metadata, the ordered STATE states, and a
// fired action all survive the file.
func TestRecordingRoundTrips(t *testing.T) {
	rec := Recording{
		SchemaVersion: SchemaVersion,
		Operator:      "batch-job",
		KartaName:     "batch-job-v1",
		Flow:          "resumed",
		Want:          string(v1alpha1.CompletedStatus),
		Events: []Event{
			{Kind: EventState, State: "Suspended", Object: map[string]interface{}{
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
			{Kind: EventState, State: "Completed", Object: map[string]interface{}{
				"kind": "Job", "metadata": map[string]interface{}{"name": "j"},
				"status": map[string]interface{}{"active": float64(0), "succeeded": float64(1)},
			}},
		},
	}

	path := filepath.Join(t.TempDir(), "op", "v1", "batch-job-v1", "resumed.yaml")
	if err := WriteRecording(path, rec); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRecording(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operator != "batch-job" || got.Flow != "resumed" || len(got.Events) != 3 {
		t.Fatalf("recording round-trip mismatch: %+v", got)
	}
	if !reflect.DeepEqual(got.States(), []string{"Suspended", "Completed"}) {
		t.Errorf("states round-trip mismatch: %v", got.States())
	}
	act := got.Events[1].Action
	if act == nil || act.Name != "Resume" || act.Operation.Verb != "PATCH" {
		t.Errorf("action did not round-trip: %+v", act)
	}
	if v, ok := act.Operation.Payload["spec"].(map[string]interface{})["suspend"]; !ok || v != false {
		t.Errorf("action payload did not round-trip: %+v", act.Operation.Payload)
	}
}
