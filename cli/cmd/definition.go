// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"github.com/spf13/cobra"
)

// newDefinitionCommand builds the "karta definition" command tree: inspection
// of the Karta definitions the CLI understands. Definitions are cluster-scoped,
// so these commands do not require a namespace.
func newDefinitionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "definition",
		Short: "Inspect the Karta definitions the CLI understands",
		Args: func(c *cobra.Command, args []string) error {
			if err := cobra.NoArgs(c, args); err != nil {
				return exitError{code: ExitUsage, err: err}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
}
