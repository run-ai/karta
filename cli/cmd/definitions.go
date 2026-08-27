// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"github.com/spf13/cobra"
)

// newDefinitionsCommand builds the "karta definitions" command tree: inspection
// of the Karta definitions the CLI understands. Definitions are cluster-scoped,
// so these commands do not require a namespace.
func newDefinitionsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "definitions",
		Short: "Inspect the Karta definitions the CLI understands",
		Args:  usageArgs(cobra.NoArgs),
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
}
