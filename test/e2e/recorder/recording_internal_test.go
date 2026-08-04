// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// MergePatch(prev, cur) applied to prev must reproduce cur, omit unchanged fields, and null a removed key.
func TestMergePatchRoundTrip(t *testing.T) {
	prev := map[string]interface{}{
		"spec": map[string]interface{}{"suspend": true, "parallelism": float64(1)},
		"status": map[string]interface{}{
			"conditions": []interface{}{map[string]interface{}{"type": "Suspended", "status": "True"}},
			"active":     float64(0),
		},
		"gone": "x",
	}
	cur := map[string]interface{}{
		"spec": map[string]interface{}{"suspend": false, "parallelism": float64(1)},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Suspended", "status": "True"},
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
			"active": float64(1),
		},
	}

	patch := MergePatch(prev, cur)

	got := runtime.DeepCopyJSON(prev)
	applyMergePatch(got, patch)
	if !reflect.DeepEqual(got, cur) {
		t.Fatalf("apply(prev, patch) != cur\npatch=%v\ngot =%v\nwant=%v", patch, got, cur)
	}
	if spec, ok := patch["spec"].(map[string]interface{}); ok {
		if _, has := spec["parallelism"]; has {
			t.Error("patch should omit unchanged spec.parallelism")
		}
	}
	if v, ok := patch["gone"]; !ok || v != nil {
		t.Error("patch should null the removed key 'gone'")
	}
}

// CRs rebuilds each step from the patches; WriteRecording/LoadRecording round-trips a file and its action.
func TestRecordingReconstructsAndRoundTrips(t *testing.T) {
	rec := Recording{
		SchemaVersion: SchemaVersion,
		Operator:      "batch-job",
		KartaName:     "batch-job-v1",
		Flow:          "resumed",
		Want:          string(v1alpha1.CompletedStatus),
		Steps: []Step{
			{
				State: "Suspended",
				CR:    map[string]interface{}{"kind": "Job", "metadata": map[string]interface{}{"name": "j"}, "status": map[string]interface{}{"active": float64(0)}},
				Action: &ActionRecord{
					Type:    "Unsuspend",
					Request: map[string]interface{}{"spec": map[string]interface{}{"suspend": false}},
				},
			},
			{
				State: "Completed",
				Patch: map[string]interface{}{"status": map[string]interface{}{"active": float64(0), "succeeded": float64(1)}},
			},
		},
	}

	crs, err := rec.CRs()
	if err != nil {
		t.Fatal(err)
	}
	if len(crs) != 2 {
		t.Fatalf("want 2 reconstructed CRs, got %d", len(crs))
	}
	if v, _, _ := unstructured.NestedFloat64(crs[1].Object, "status", "succeeded"); v != 1 {
		t.Errorf("cr[1] status.succeeded = %v, want 1", v)
	}

	path := filepath.Join(t.TempDir(), "op", "v1", "batch-job-v1", "resumed.yaml")
	if err := WriteRecording(path, rec); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRecording(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operator != "batch-job" || got.Flow != "resumed" || len(got.Steps) != 2 {
		t.Fatalf("recording round-trip mismatch: %+v", got)
	}
	if got.Steps[0].Action == nil || got.Steps[0].Action.Type != "Unsuspend" {
		t.Errorf("action did not round-trip: %+v", got.Steps[0].Action)
	}
	if !reflect.DeepEqual(got.States(), []string{"Suspended", "Completed"}) {
		t.Errorf("states round-trip mismatch: %v", got.States())
	}
}
