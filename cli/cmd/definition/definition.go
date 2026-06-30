// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package definition holds the "karta definition" command tree: inspection of
// the Karta definitions the CLI understands. Definitions are cluster-scoped, so
// these commands do not require a namespace.
package definition

import (
	"github.com/spf13/cobra"
)

// NewCommand builds the "definition" command.
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "definition",
		Short: "Inspect the Karta definitions the CLI understands",
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
}
