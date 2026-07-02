// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package workload holds the "karta workload" command tree.
package workload

import (
	"github.com/run-ai/karta/cli/cmd/flags"

	"github.com/spf13/cobra"
)

// NewCommand builds the "workload" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Inspect workloads running in a namespace",
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	flags.WithNamespace(cmd)

	return cmd
}
