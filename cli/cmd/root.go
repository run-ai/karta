// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cmd wires the Karta CLI Cobra command tree.
package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/run-ai/karta/pkg/version"
)

// NewRootCommand builds the root command for the karta binary. Global flags are
// registered as persistent flags so every subcommand inherits them.
func NewRootCommand() *cobra.Command {
	kubeFlags = genericclioptions.NewConfigFlags(true)

	cmd := &cobra.Command{
		Use:   "karta",
		Short: "Workload-aware visibility for any Kubernetes workload type",
		Long: "Karta gives operators a uniform view of any Kubernetes workload type, " +
			"built on the Karta abstraction layer. Inspect workloads running in a " +
			"namespace and the definitions Karta understands.",
		Version:      version.String(),
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Name() == cobra.ShellCompRequestCmd {
				return nil
			}
			var err error
			config, err = buildConfig(cmd)
			return err
		},
	}

	// Print the bare version, matching what the operator's --version reports.
	cmd.SetVersionTemplate("{{.Version}}\n")

	kubeFlags.AddFlags(cmd.PersistentFlags())
	withOutput(cmd)
	withConfig(cmd)

	cmd.AddCommand(newWorkloadCommand())
	cmd.AddCommand(newDefinitionCommand())

	cmd.InitDefaultCompletionCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "completion" {
			sub.PersistentPreRunE = func(*cobra.Command, []string) error { return nil }
			break
		}
	}

	return cmd
}
