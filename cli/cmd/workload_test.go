// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestTree wires the workload command (which owns the namespace flag) under
// a bare root, plus a dummy runnable subcommand that inherits the flag.
func newTestTree() (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "root", SilenceUsage: true, SilenceErrors: true}

	wl := newWorkloadCommand()
	wl.AddCommand(&cobra.Command{
		Use:  "dummy",
		RunE: func(*cobra.Command, []string) error { return nil },
	})
	root.AddCommand(wl)

	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	return root, out
}

func TestWorkloadWithNamespace(t *testing.T) {
	root, _ := newTestTree()
	root.SetArgs([]string{"workload", "dummy", "-n", "ml-team"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error with namespace set: %v", err)
	}
}

// The workload parent command with a namespace but no subcommand prints help
// rather than erroring.
func TestWorkloadParentShowsHelp(t *testing.T) {
	root, out := newTestTree()
	root.SetArgs([]string{"workload", "-n", "ml-team"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Inspect workloads running in a namespace") {
		t.Fatalf("expected workload help output, got %q", out.String())
	}
}
