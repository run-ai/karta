// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cmd wires the Karta CLI Cobra command tree.
package cmd

import (
	"github.com/run-ai/karta/cli/cmd/definition"
	"github.com/run-ai/karta/cli/cmd/flags"
	"github.com/run-ai/karta/cli/cmd/workload"

	"github.com/spf13/cobra"
)

// NewRootCommand builds the root command for the karta binary. Global flags are
// registered as persistent flags so every subcommand inherits them.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "karta",
		Short: "Workload-aware visibility for any Kubernetes workload type",
		Long: "Karta gives operators a uniform view of any Kubernetes workload type, " +
			"built on the Karta abstraction layer. Inspect workloads running in a " +
			"namespace and the definitions Karta understands.",
		SilenceUsage: true,
	}

	flags.AddGlobals(cmd)

	cmd.AddCommand(workload.NewCommand())
	cmd.AddCommand(definition.NewCommand())

	return cmd
}
