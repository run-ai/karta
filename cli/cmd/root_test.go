// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// execute runs the root command with the given args, capturing its output. A
// noop runnable subcommand is attached so persistent pre-run hooks (e.g. output
// validation) fire; a bare parent command would only print help and skip them.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	cmd.AddCommand(&cobra.Command{
		Use:  "noop",
		RunE: func(*cobra.Command, []string) error { return nil },
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestRootHelp(t *testing.T) {
	out, err := execute(t, "--help")
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	for _, want := range []string{"--kubeconfig", "--namespace", "--output", "workload", "definition"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q\n%s", want, out)
		}
	}
}

func TestOutputValidation(t *testing.T) {
	for _, format := range []string{outputTable, outputWide, outputJSON, outputYAML} {
		if _, err := execute(t, "-o", format, "noop"); err != nil {
			t.Errorf("output %q rejected: %v", format, err)
		}
	}

	if _, err := execute(t, "-o", "bogus", "noop"); err == nil {
		t.Error("expected error for invalid output format, got nil")
	}
}
