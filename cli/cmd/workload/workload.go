// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package workload holds the "karta workload" command tree: operational
// visibility into workloads running in a namespace.
package workload

import (
	"github.com/run-ai/karta/cli/cmd/flags"

	"github.com/spf13/cobra"
)

// NewCommand builds the "workload" command. The MVP is single-namespace only,
// so -n/--namespace is a required persistent flag inherited by every workload
// subcommand; cobra rejects any invocation that omits it.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Inspect workloads running in a namespace",
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	flags.WithNamespace(cmd)

	return cmd
}
