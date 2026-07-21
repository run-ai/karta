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

func TestSanitizeRemovesVolatileKeepsRest(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]interface{}{
			"name":            "j",
			"resourceVersion": "123",
			"uid":             "abc",
			"managedFields":   []interface{}{map[string]interface{}{"manager": "x"}},
			"annotations": map[string]interface{}{
				"kubectl.kubernetes.io/last-applied-configuration": "{...}",
				"keep": "me",
			},
		},
		"status": map[string]interface{}{
			"active": int64(1),
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True", "lastTransitionTime": "2026-01-01T00:00:00Z"},
			},
		},
	}}

	Sanitize(u)

	md := u.Object["metadata"].(map[string]interface{})
	for _, k := range []string{"resourceVersion", "uid", "managedFields"} {
		if _, ok := md[k]; ok {
			t.Errorf("metadata.%s should be removed", k)
		}
	}
	if md["name"] != "j" {
		t.Error("metadata.name must be kept")
	}
	ann := md["annotations"].(map[string]interface{})
	if _, ok := ann["kubectl.kubernetes.io/last-applied-configuration"]; ok {
		t.Error("last-applied annotation should be removed")
	}
	if ann["keep"] != "me" {
		t.Error("other annotations must be kept")
	}
	cond := u.Object["status"].(map[string]interface{})["conditions"].([]interface{})[0].(map[string]interface{})
	if _, ok := cond["lastTransitionTime"]; ok {
		t.Error("conditions[].lastTransitionTime should be removed")
	}
	if cond["type"] != "Complete" {
		t.Error("condition type must be kept")
	}
	if u.Object["status"].(map[string]interface{})["active"] != int64(1) {
		t.Error("status.active must be kept")
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sd := SnapshotDir(0, "Completed")
	f := Fixture{
		SchemaVersion:  SchemaVersion,
		Operator:       "jobset",
		Version:        "0.9.1",
		KartaName:      "x",
		Flow:           "happy",
		Want:           v1alpha1.CompletedStatus,
		ObservedStates: []string{"Completed"},
		Snapshots:      []Snapshot{{State: "Completed", Dir: sd}},
	}
	data := map[string]SnapshotData{
		sd: {
			CR: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind":     "Job",
				"metadata": map[string]interface{}{"name": "j"},
			}},
			Expected: Expected{MatchedStatuses: []v1alpha1.ResourceStatus{v1alpha1.CompletedStatus}},
		},
	}

	if err := Write(dir, f, data); err != nil {
		t.Fatal(err)
	}
	got, gotData, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operator != "jobset" || got.Flow != "happy" || len(got.Snapshots) != 1 {
		t.Fatalf("fixture round-trip mismatch: %+v", got)
	}
	if got.Want != v1alpha1.CompletedStatus {
		t.Errorf("want round-trip mismatch: %q", got.Want)
	}
	if !reflect.DeepEqual(got.ObservedStates, f.ObservedStates) {
		t.Errorf("observedStates round-trip mismatch: %v", got.ObservedStates)
	}
	if gotData[sd].CR.Object["kind"] != "Job" {
		t.Error("cr round-trip mismatch")
	}
	if want := gotData[sd].Expected.MatchedStatuses; len(want) != 1 || want[0] != v1alpha1.CompletedStatus {
		t.Errorf("expected round-trip mismatch: %v", want)
	}
}

func TestCollapseConsecutive(t *testing.T) {
	cases := []struct{ in, want []string }{
		{nil, nil},
		{[]string{"Running"}, []string{"Running"}},
		{[]string{"Suspended", "Suspended", "Running", "Running", "Completed"}, []string{"Suspended", "Running", "Completed"}},
		{[]string{"A", "B", "A"}, []string{"A", "B", "A"}}, // non-adjacent repeats are kept
	}
	for _, c := range cases {
		if got := CollapseConsecutive(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("CollapseConsecutive(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestReplayRunsAndIsDeterministic proves the shared read path executes against a
// real Karta definition and object and is stable, which is the property the offline
// golden test depends on.
func TestReplayRunsAndIsDeterministic(t *testing.T) {
	karta := types.PyFlowKarta()
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(types.NewPyFlowObject())
	if err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{Object: m}

	a, err := Replay(karta, u)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	b, err := Replay(karta, u)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Replay must be deterministic:\n a=%+v\n b=%+v", a, b)
	}
}
