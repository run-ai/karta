// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"github.com/spf13/cobra"
)

// newWorkloadCommand builds the "karta workload" command tree.
func newWorkloadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Inspect workloads running in a namespace",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	withNamespace(cmd)

	return cmd
}
