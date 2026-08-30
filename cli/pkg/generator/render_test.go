// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/run-ai/karta/cli/pkg/workload"
)

func TestRenderTableColumns(t *testing.T) {
	views := []workload.View{{
		Name:      "preprocess",
		Namespace: "ml-team",
		Kind:      "JobSet",
		Phases:    []string{"Completed"},
		Origin:    "catalog",
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
			want:    []string{"NAME", "NAMESPACE", "ml-team", "PHASE", "Completed", "AGE"},
			notWant: []string{"ORIGIN", "COMPONENTS", "GPU"},
		},
		{
			name: "wide adds ORIGIN",
			opts: Options{Output: OutputWide},
			want: []string{"ORIGIN", "catalog"},
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
