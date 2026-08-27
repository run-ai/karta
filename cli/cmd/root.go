// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cmd wires the Karta CLI Cobra command tree.
package cmd

import (
	"fmt"

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
		Version: version.String(),
		// main prints the error, so it carries the lowercase "error:" prefix
		// rather than Cobra's "Error:".
		SilenceUsage:  true,
		SilenceErrors: true,
		// Cobra reports an unrecognised subcommand only for a non-runnable
		// command, and before validating args, so the root runs and rejects it.
		Args: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return nil
			}
			return exitError{code: ExitUsage,
				err: fmt.Errorf("unknown command %q for %q", args[0], c.CommandPath())}
		},
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Name() == cobra.ShellCompRequestCmd {
				return nil
			}
			var err error
			config, err = buildConfig(cmd)
			return err
		},
	}

	cmd.SetVersionTemplate("{{.Version}}\n")

	// Subcommands inherit this from the root, so every flag parse failure in the
	// tree is a usage error without per-command wiring.
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return exitError{code: ExitUsage, err: err}
	})

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
