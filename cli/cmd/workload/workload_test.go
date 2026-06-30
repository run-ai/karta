// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package workload

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

// newTestTree wires the workload command under a root that registers the shared
// namespace flag, plus a dummy runnable subcommand so the inherited
// PersistentPreRunE actually fires.
func newTestTree() *cobra.Command {
	cobra.EnableTraverseRunHooks = true

	root := &cobra.Command{Use: "root", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringP("namespace", "n", "", "")

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
	if err := root.Execute(); !errors.Is(err, ErrNamespaceRequired) {
		t.Fatalf("expected ErrNamespaceRequired, got %v", err)
	}
}

func TestWorkloadWithNamespace(t *testing.T) {
	root := newTestTree()
	root.SetArgs([]string{"-n", "ml-team", "workload", "dummy"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error with namespace set: %v", err)
	}
}
