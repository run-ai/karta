// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator

import (
	"bytes"
	"slices"
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

	// Asserting the split cells rather than a substring of the whole table pins
	// the column set and its order. A substring check cannot: "NAME" is a
	// substring of "NAMESPACE", so it passes whether or not the column is there.
	for _, tc := range []struct {
		name    string
		opts    Options
		headers []string
		row     []string
	}{
		{
			name:    "default columns",
			opts:    Options{Output: OutputTable},
			headers: []string{"NAME", "NAMESPACE", "PHASE", "AGE"},
			row:     []string{"preprocess", "ml-team", "Completed", "<unknown>"},
		},
		{
			name:    "the zero value renders the default table",
			opts:    Options{},
			headers: []string{"NAME", "NAMESPACE", "PHASE", "AGE"},
			row:     []string{"preprocess", "ml-team", "Completed", "<unknown>"},
		},
		{
			name:    "wide adds ORIGIN",
			opts:    Options{Output: OutputWide},
			headers: []string{"NAME", "NAMESPACE", "PHASE", "AGE", "ORIGIN"},
			row:     []string{"preprocess", "ml-team", "Completed", "<unknown>", "catalog"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if err := Render(&out, &errOut, views, tc.opts); err != nil {
				t.Fatalf("render: %v", err)
			}

			lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
			if len(lines) != 2 {
				t.Fatalf("expected a header and one row, got %d lines:\n%s", len(lines), out.String())
			}
			// The tab writer pads with spaces, and no cell contains one.
			if got := strings.Fields(lines[0]); !slices.Equal(got, tc.headers) {
				t.Errorf("headers: got %v, want %v", got, tc.headers)
			}
			if got := strings.Fields(lines[1]); !slices.Equal(got, tc.row) {
				t.Errorf("row: got %v, want %v", got, tc.row)
			}
		})
	}
}

// The flag layer rejects an unknown format, so a programmatic caller passing one
// must not silently fall through to a table.
func TestRenderRejectsAnUnknownFormat(t *testing.T) {
	var out, errOut bytes.Buffer
	err := Render(&out, &errOut, []workload.View{{Name: "x"}}, Options{Output: "bogus"})
	if err == nil {
		t.Fatal("expected an error for an unsupported output format")
	}
	if out.Len() != 0 {
		t.Errorf("expected no output, got %q", out.String())
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
