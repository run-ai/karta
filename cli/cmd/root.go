// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cmd wires the Karta CLI Cobra command tree.
package cmd

import (
	"fmt"

	"github.com/run-ai/karta/cli/cmd/definition"
	"github.com/run-ai/karta/cli/cmd/workload"

	"github.com/spf13/cobra"
)

// Output formats accepted by the global -o/--output flag.
const (
	outputTable = "table"
	outputWide  = "wide"
	outputJSON  = "json"
	outputYAML  = "yaml"
)

// NewRootCommand builds the root command for the karta binary. Global flags are
// registered as persistent flags so every subcommand inherits them.
func NewRootCommand() *cobra.Command {
	// Run every PersistentPreRunE from the root down, not only the deepest one,
	// so the root's output validation and the workload subtree's namespace guard
	// both execute under nested commands.
	cobra.EnableTraverseRunHooks = true

	var (
		kubeconfig string
		namespace  string
		output     string
	)

	cmd := &cobra.Command{
		Use:   "karta",
		Short: "Workload-aware visibility for any Kubernetes workload type",
		Long: "Karta gives operators a uniform view of any Kubernetes workload type, " +
			"built on the Karta abstraction layer. Inspect workloads running in a " +
			"namespace and the definitions Karta understands.",
		SilenceUsage: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			switch output {
			case outputTable, outputWide, outputJSON, outputYAML:
				return nil
			default:
				return fmt.Errorf("invalid output format %q: must be one of table, wide, json, yaml", output)
			}
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVar(&kubeconfig, "kubeconfig", "",
		"Path to the kubeconfig file to use (defaults to $KUBECONFIG or ~/.kube/config)")
	flags.StringVarP(&namespace, "namespace", "n", "",
		"Namespace scope for workload commands")
	flags.StringVarP(&output, "output", "o", outputTable,
		"Output format: table, wide, json, or yaml")

	cmd.AddCommand(workload.NewCommand())
	cmd.AddCommand(definition.NewCommand())

	return cmd
}
