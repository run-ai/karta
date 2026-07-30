// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/types"
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

// CRs() and Readings() rebuild each step from the patches; WriteRecording/LoadRecording round-trips a file.
func TestRecordingReconstructsAndRoundTrips(t *testing.T) {
	rec := Recording{
		SchemaVersion: SchemaVersion,
		Operator:      "batch-job",
		KartaName:     "batch-job-v1",
		Flow:          "completed",
		Want:          string(v1alpha1.CompletedStatus),
		Steps: []Step{
			{
				State:    "Initializing",
				CR:       map[string]interface{}{"kind": "Job", "metadata": map[string]interface{}{"name": "j"}, "status": map[string]interface{}{"active": float64(1)}},
				Expected: map[string]interface{}{"matchedStatuses": []interface{}{"Initializing"}},
			},
			{
				State:         "Completed",
				Patch:         map[string]interface{}{"status": map[string]interface{}{"active": float64(0), "succeeded": float64(1)}},
				ExpectedPatch: map[string]interface{}{"matchedStatuses": []interface{}{"Completed"}},
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
	if v, _, _ := unstructured.NestedFloat64(crs[0].Object, "status", "active"); v != 1 {
		t.Errorf("cr[0] status.active = %v, want 1", v)
	}
	if v, _, _ := unstructured.NestedFloat64(crs[1].Object, "status", "succeeded"); v != 1 {
		t.Errorf("cr[1] status.succeeded = %v, want 1", v)
	}
	if v, _, _ := unstructured.NestedFloat64(crs[1].Object, "status", "active"); v != 0 {
		t.Errorf("cr[1] status.active = %v, want 0", v)
	}

	readings, err := rec.Readings()
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 2 {
		t.Fatalf("want 2 reconstructed readings, got %d", len(readings))
	}
	if got := matchedStatuses(readings[0]); !reflect.DeepEqual(got, []string{"Initializing"}) {
		t.Errorf("reading[0] matchedStatuses = %v, want [Initializing]", got)
	}
	if got := matchedStatuses(readings[1]); !reflect.DeepEqual(got, []string{"Completed"}) {
		t.Errorf("reading[1] matchedStatuses = %v, want [Completed]", got)
	}

	path := filepath.Join(t.TempDir(), "op", "v1", "batch-job-v1", "completed.yaml")
	if err := WriteRecording(path, rec); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRecording(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operator != "batch-job" || got.Flow != "completed" || got.Want != string(v1alpha1.CompletedStatus) || len(got.Steps) != 2 {
		t.Fatalf("recording round-trip mismatch: %+v", got)
	}
	if !reflect.DeepEqual(got.States(), []string{"Initializing", "Completed"}) {
		t.Errorf("states round-trip mismatch: %v", got.States())
	}
}

func TestReadingRunsKarta(t *testing.T) {
	karta := types.PyFlowKarta()
	obj := types.NewPyFlowObject()
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	reading, err := Reading(t.Context(), karta, &unstructured.Unstructured{Object: m})
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if reading == nil {
		t.Fatal("reading is nil")
	}
}
