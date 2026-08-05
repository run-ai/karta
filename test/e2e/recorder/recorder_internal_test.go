// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Plain unit tests for the pure recorder helpers; no cluster (RunSpecs is not invoked).

func TestAddStateRejectsEmptyName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("AddState with an empty name did not panic")
		}
	}()
	(&Recorder{}).AddState("", nil)
}

func TestSetTimeoutRejectsNonPositive(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		func(d time.Duration) {
			defer func() {
				if recover() == nil {
					t.Errorf("SetTimeout(%s) did not panic", d)
				}
			}()
			(&Recorder{}).SetTimeout(d)
		}(d)
	}
}

func TestAddStateRejectsNilMatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("AddState with a nil predicate did not panic")
		}
	}()
	(&Recorder{}).AddState("Running", nil)
}

func TestNewValidatesRequiredInput(t *testing.T) {
	valid := Config{OutputDir: "out"}
	cases := []struct {
		name                                    string
		cfg                                     Config
		operator, version, kartaName, kartaFile string
	}{
		{"empty operator", valid, "", "v", "n", "f"},
		{"empty version", valid, "op", "", "n", "f"},
		{"empty kartaName", valid, "op", "v", "", "f"},
		{"empty kartaFile", valid, "op", "v", "n", ""},
		{"empty outputDir", Config{}, "op", "v", "n", "f"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("New did not panic")
				}
			}()
			New(c.cfg, c.operator, c.version, c.kartaName, c.kartaFile)
		})
	}
}

func objWithStatus(status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"status": status}}
}

// intAtLeast and condTrue are minimal StateCheck helpers for these unit tests; the flows package holds the
// full predicate vocabulary.
func intAtLeast(n int64, path ...string) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		v, _, _ := unstructured.NestedInt64(u.Object, path...)
		return v >= n
	}
}

func condTrue(condType string) StateCheck {
	return func(u *unstructured.Unstructured) bool {
		conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conds {
			if m, ok := c.(map[string]any); ok && m["type"] == condType && m["status"] == "True" {
				return true
			}
		}
		return false
	}
}

// TestClassifyPicksMostAdvancedState: Classify keeps the furthest state a workload matches.
func TestClassifyPicksMostAdvancedState(t *testing.T) {
	running := kartav1alpha1.RunningStatus
	completed := kartav1alpha1.CompletedStatus
	states := []NamedState{
		{Name: running, Match: intAtLeast(1, "status", "active")},
		{Name: completed, Match: condTrue("Complete")},
	}
	if got := Classify(objWithStatus(map[string]any{"active": int64(1)}), states); got != running {
		t.Errorf("active=1 -> %q, want %q", got, running)
	}
	both := objWithStatus(map[string]any{"active": int64(1), "conditions": []any{map[string]any{"type": "Complete", "status": "True"}}})
	if got := Classify(both, states); got != completed {
		t.Errorf("Complete=True -> %q, want %q", got, completed)
	}
	if got := Classify(objWithStatus(map[string]any{}), states); got != "" {
		t.Errorf("no match -> %q, want empty", got)
	}
}

func terminal(journey []journeyStep) kartav1alpha1.ResourceStatus {
	return journey[len(journey)-1].State
}

// TestRecorderCatchesBackwardsJump: a Running -> byte-identical Initializing dip survives dedup, and the
// strict order check flags it unless the journey declares the dip.
func TestRecorderCatchesBackwardsJump(t *testing.T) {
	initializing := kartav1alpha1.InitializingStatus
	running := kartav1alpha1.RunningStatus
	completed := kartav1alpha1.CompletedStatus

	states := []NamedState{
		{Name: initializing, Match: intAtLeast(1, "status", "active")},
		{Name: running, Match: intAtLeast(1, "status", "ready")},
		{Name: completed, Match: condTrue("Complete")},
	}
	initCR := func() *unstructured.Unstructured {
		return objWithStatus(map[string]any{"active": int64(1), "ready": int64(0)})
	}
	seq := []*unstructured.Unstructured{
		initCR(),
		objWithStatus(map[string]any{"active": int64(1), "ready": int64(1)}),
		initCR(),
		objWithStatus(map[string]any{"conditions": []any{map[string]any{"type": "Complete", "status": "True"}}}),
	}

	o := &observation{}
	for _, cr := range seq {
		o.keep(cr, Classify(cr, states))
	}

	want := []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}
	if !reflect.DeepEqual(o.states(), want) {
		t.Fatalf("recorder dropped the return: got %v, want %v", o.states(), want)
	}
	if observedOrderErr(steps(initializing, running, completed), o.states(), completed) == nil {
		t.Error("strict journey should reject the undeclared Running -> Initializing dip")
	}
	if err := observedOrderErr(steps(initializing, running, initializing, completed), o.states(), completed); err != nil {
		t.Errorf("declaring the Initializing revisit should accept the dip, got %v", err)
	}
}

func TestObservedOrder(t *testing.T) {
	initializing := kartav1alpha1.InitializingStatus
	running := kartav1alpha1.RunningStatus
	completed := kartav1alpha1.CompletedStatus
	failed := kartav1alpha1.FailedStatus

	tests := []struct {
		name     string
		journey  []journeyStep
		observed []kartav1alpha1.ResourceStatus
		ok       bool
	}{
		{"exact", steps(initializing, running, completed), []kartav1alpha1.ResourceStatus{initializing, running, completed}, true},
		{"skip a required step fails", steps(initializing, running, completed), []kartav1alpha1.ResourceStatus{initializing, completed}, false},
		{"skip an optional step is ok", []journeyStep{{State: initializing}, {State: running, Optional: true}, {State: completed}}, []kartav1alpha1.ResourceStatus{initializing, completed}, true},
		{"undeclared state", steps(initializing, running), []kartav1alpha1.ResourceStatus{initializing, failed}, false},
		{"repeat dip missed is ok", steps(initializing, running, initializing, completed), []kartav1alpha1.ResourceStatus{initializing, running, completed}, true},
		{"optional dip missed is ok", []journeyStep{{State: initializing}, {State: running}, {State: initializing, Optional: true}, {State: completed}}, []kartav1alpha1.ResourceStatus{initializing, running, completed}, true},
		{"optional dip caught is ok", []journeyStep{{State: initializing}, {State: running}, {State: initializing, Optional: true}, {State: completed}}, []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}, true},
		{"undeclared dip fails", steps(initializing, running, completed), []kartav1alpha1.ResourceStatus{initializing, running, initializing, completed}, false},
		{"wrong terminal", steps(initializing, running, completed), []kartav1alpha1.ResourceStatus{initializing, running}, false},
	}
	for _, c := range tests {
		err := observedOrderErr(c.journey, c.observed, terminal(c.journey))
		if c.ok && err != nil {
			t.Errorf("%s: want ok, got %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: want error, got nil", c.name)
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
