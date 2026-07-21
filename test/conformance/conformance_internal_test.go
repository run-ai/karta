// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
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

// TestReplayKeepsFullExtractionAndStrips checks the two properties golden relies on: the
// reading keeps the WHOLE extracted pod template (not a hand-picked subset), and a per-run
// volatile field is stripped from it. Together they let golden guard every field without
// the fixture churning between records.
func TestReplayKeepsFullExtractionAndStrips(t *testing.T) {
	karta := types.PyFlowKarta()
	obj := types.NewPyFlowObject()
	obj.Spec.Master.Template.Spec.NodeName = "worker-node-7" // a volatile field the pod template carries

	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Replay(karta, &unstructured.Unstructured{Object: m})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	s := string(blob)
	if !strings.Contains(s, "tensorflow/tensorflow:2.8.0-gpu") {
		t.Error("reading dropped the pod template: container image missing")
	}
	if strings.Contains(s, "worker-node-7") || strings.Contains(s, "nodeName") {
		t.Error("reading kept the volatile nodeName; stripVolatile should have removed it")
	}
}

// TestRecordsEveryDistinctCR guards recorder granularity: it keeps every distinct CR a
// workload passes through, not one per state. So if a workload stays Running across several
// distinct CRs, golden replays each - and a library change that reads a middle Running CR
// as Degraded is caught, which one-per-state would hide. It fails if no flow captured a
// repeated state, and checks that repeated-state snapshots are really distinct CRs
// (identical ones should have been deduped).
func TestRecordsEveryDistinctCR(t *testing.T) {
	root := "fixtures"

	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "fixture.yaml" {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if len(dirs) == 0 {
		t.Skip("no conformance fixtures recorded yet (run: make record-e2e)")
	}

	flowsWithRepeat := 0
	for _, dir := range dirs {
		fx, data, err := Load(dir)
		if err != nil {
			t.Fatalf("load %s: %v", dir, err)
		}
		for i := 1; i < len(fx.Snapshots); i++ {
			prev, cur := fx.Snapshots[i-1], fx.Snapshots[i]
			if prev.State != cur.State {
				continue // a state boundary, not a repeated state
			}
			// Same state across two snapshots: dedup-by-content means they are distinct
			// CRs, and golden replays both.
			a, _ := json.Marshal(data[prev.Dir].CR.Object)
			b, _ := json.Marshal(data[cur.Dir].CR.Object)
			if string(a) == string(b) {
				t.Errorf("%s/%s: %s and %s share a state and an identical CR; dedup should have collapsed them",
					fx.Operator, fx.Flow, prev.Dir, cur.Dir)
			}
			flowsWithRepeat++
			break
		}
	}
	if flowsWithRepeat == 0 {
		t.Error("no flow captured a repeated state (one across distinct CRs); the recorder may have " +
			"collapsed to one snapshot per state, which would hide a middle-state reading regression")
	} else {
		t.Logf("%d recorded flow(s) capture a repeated state across distinct CRs", flowsWithRepeat)
	}
}
