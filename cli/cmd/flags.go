// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/run-ai/karta/cli/pkg/generator"
)

const (
	flagConfig = "config"
	flagOutput = "output"
)

var kubeFlags *genericclioptions.ConfigFlags

// withOutput registers the -o/--output enum persistent flag on cmd, backed by
// generator.Output, along with its shell completion.
func withOutput(cmd *cobra.Command) *Enum[generator.Output] {
	out := NewOutputFlag()
	cmd.PersistentFlags().VarP(out, flagOutput, "o",
		"Output format: one of "+strings.Join(out.Allowed(), ", "))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc(flagOutput,
		func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return out.Allowed(), cobra.ShellCompDirectiveNoFileComp
		}))
	return out
}

// withConfig registers the --config persistent flag on cmd.
func withConfig(cmd *cobra.Command) {
	cmd.PersistentFlags().String(flagConfig, "",
		"Path to the config file (default $KARTA_CONFIG or $HOME/.karta/config.yaml)")
}
