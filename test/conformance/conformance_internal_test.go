// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/types"
)

// TestMergePatchRoundTrip is the core of the recording format: MergePatch(prev, cur) applied to prev
// must reproduce cur, the patch must omit unchanged fields, and a removed key must be nulled.
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

// TestRecordingReconstructsAndRoundTrips checks that CRs() rebuilds each snapshot by applying the
// patches in order, and that WriteRecording/LoadRecording round-trips.
func TestRecordingReconstructsAndRoundTrips(t *testing.T) {
	first := map[string]interface{}{
		"kind":     "Job",
		"metadata": map[string]interface{}{"name": "j"},
		"status":   map[string]interface{}{"active": float64(1)},
	}
	rec := Recording{
		SchemaVersion: SchemaVersion,
		Operator:      "batch-job",
		KartaName:     "batch-job-v1",
		Flow:          "completed",
		Want:          v1alpha1.CompletedStatus,
		Steps: []Step{
			{State: "Initializing", CR: first},
			{State: "Completed", Patch: map[string]interface{}{
				"status": map[string]interface{}{"active": float64(0), "succeeded": float64(1)},
			}},
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

	dir := t.TempDir()
	if err := WriteRecording(dir, rec); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRecording(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operator != "batch-job" || got.Flow != "completed" || got.Want != v1alpha1.CompletedStatus || len(got.Steps) != 2 {
		t.Fatalf("recording round-trip mismatch: %+v", got)
	}
	if !reflect.DeepEqual(got.States(), []string{"Initializing", "Completed"}) {
		t.Errorf("states round-trip mismatch: %v", got.States())
	}
}

// TestReadRunsKarta checks the shared Read helper drives the Karta library on a CR without error.
func TestReadRunsKarta(t *testing.T) {
	karta := types.PyFlowKarta()
	obj := types.NewPyFlowObject()
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Read(karta, &unstructured.Unstructured{Object: m}); err != nil {
		t.Fatalf("read: %v", err)
	}
}
