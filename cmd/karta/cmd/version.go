// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build metadata injected via ldflags at release time. Defaults make sense
// for `go build` developer builds.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print karta version, commit, and build date",
		Args:  cobra.NoArgs,
		Run: func(c *cobra.Command, _ []string) {
			fmt.Fprintf(c.OutOrStdout(), "karta %s (commit %s, built %s)\n", version, commit, date)
		},
	}
}
