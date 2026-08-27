// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/run-ai/karta/cli/pkg/workload"
)

func TestComponentsElidesLongLists(t *testing.T) {
	// Milvus declares eighteen components; a row that printed them all would
	// push GPU and AGE off screen.
	many := make([]workload.ComponentView, 0, 18)
	for _, name := range []string{
		"standalone", "proxy", "mixcoord", "datanode", "querynode",
		"streamingnode", "indexnode", "rootcoord",
	} {
		many = append(many, workload.ComponentView{Name: name, Replicas: 1})
	}

	got := components(many)
	if len(got) > componentsWidth+len(", +8 more") {
		t.Errorf("cell not elided: %q", got)
	}
	if !strings.Contains(got, "more") {
		t.Errorf("expected an elision marker, got %q", got)
	}
	if !strings.HasPrefix(got, "standalone(1)") {
		t.Errorf("expected the first components to survive, got %q", got)
	}
}

func TestComponentsRendersRoleAndCount(t *testing.T) {
	got := components([]workload.ComponentView{
		{Name: "master", Replicas: 1},
		{Name: "worker", Replicas: 4},
	})
	if got != "master(1), worker(4)" {
		t.Errorf("unexpected cell: %q", got)
	}
}

// A workload whose definition extracts no pod-bearing component still needs a
// cell, so the columns stay aligned.
func TestComponentsWithNone(t *testing.T) {
	if got := components(nil); got != "<none>" {
		t.Errorf("unexpected cell: %q", got)
	}
}

func TestRenderTableColumns(t *testing.T) {
	views := []workload.View{{
		Name:       "preprocess",
		Namespace:  "ml-team",
		Kind:       "JobSet",
		Phases:     []string{"Completed"},
		GPUs:       0,
		Origin:     "community",
		Components: []workload.ComponentView{{Name: "etl", Replicas: 3}},
	}}

	for _, tc := range []struct {
		name    string
		opts    Options
		want    []string
		notWant []string
	}{
		{
			name:    "default columns",
			opts:    Options{Output: OutputTable},
			want:    []string{"NAME", "NAMESPACE", "ml-team", "PHASE", "COMPONENTS", "GPU", "AGE", "etl(3)"},
			notWant: []string{"ORIGIN"},
		},
		{
			name: "wide adds ORIGIN",
			opts: Options{Output: OutputWide},
			want: []string{"ORIGIN", "community"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if err := Render(&out, &errOut, views, tc.opts); err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q\n%s", want, out.String())
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(out.String(), notWant) {
					t.Errorf("unexpected %q\n%s", notWant, out.String())
				}
			}
		})
	}
}

// The empty notice must not reach stdout, where it would corrupt a pipe.
func TestRenderEmptyGoesToStderr(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := Render(&out, &errOut, nil, Options{Output: OutputTable, Namespace: "ml-team"}); err != nil {
		t.Fatalf("render: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "No workloads found in namespace ml-team.") {
		t.Errorf("unexpected stderr: %q", errOut.String())
	}
}

func TestRenderYAMLIsAlwaysASequence(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := Render(&out, &errOut, nil, Options{Output: OutputYAML}); err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Errorf("expected an empty sequence, got %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("machine output must not carry the empty notice, got %q", errOut.String())
	}
}

func TestRenderUnsetAgeIsUnknown(t *testing.T) {
	var out, errOut bytes.Buffer
	views := []workload.View{{Name: "offline", Namespace: "ml-team"}}
	if err := Render(&out, &errOut, views, Options{Output: OutputTable}); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out.String(), "<unknown>") {
		t.Errorf("expected <unknown> for an unset timestamp\n%s", out.String())
	}
}

// A multi-instance component takes its name from the instance key, which can
// outgrow the cell on its own.
func TestComponentsCapsASingleLongEntry(t *testing.T) {
	got := components([]workload.ComponentView{
		{Name: strings.Repeat("very-long-service-name", 4), Replicas: 2},
	})
	if len(got) > componentsWidth {
		t.Errorf("cell exceeds the width budget at %d chars: %q", len(got), got)
	}
}
