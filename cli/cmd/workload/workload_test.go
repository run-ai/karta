// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package workload

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestTree wires the workload command (which owns the required namespace
// flag) under a bare root, plus a dummy runnable subcommand that inherits the
// flag.
func newTestTree() *cobra.Command {
	root := &cobra.Command{Use: "root", SilenceUsage: true, SilenceErrors: true}

	wl := NewCommand()
	wl.AddCommand(&cobra.Command{
		Use:  "dummy",
		RunE: func(*cobra.Command, []string) error { return nil },
	})
	root.AddCommand(wl)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	return root
}

func TestWorkloadRequiresNamespace(t *testing.T) {
	root := newTestTree()
	root.SetArgs([]string{"workload", "dummy"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error when namespace is omitted, got nil")
	}
	if !strings.Contains(err.Error(), "namespace") {
		t.Fatalf("expected a namespace-required error, got %v", err)
	}
}

func TestWorkloadWithNamespace(t *testing.T) {
	root := newTestTree()
	root.SetArgs([]string{"workload", "dummy", "-n", "ml-team"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error with namespace set: %v", err)
	}
}
