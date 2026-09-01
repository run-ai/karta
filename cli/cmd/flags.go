// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	flagConfig = "config"
	flagOutput = "output"
	flagColor  = "color"
)

var kubeFlags *genericclioptions.ConfigFlags

// clusterAccess resolves the cluster connection commands read through. It is a
// variable so tests can point the command tree at a fake cluster.
var clusterAccess = func() genericclioptions.RESTClientGetter { return kubeFlags }

// withOutput registers the -o/--output enum persistent flag on cmd, backed by
// generator.Output, along with its shell completion.
func withOutput(cmd *cobra.Command) {
	out := NewOutputFlag()
	cmd.PersistentFlags().VarP(out, flagOutput, "o",
		"Output format: one of "+strings.Join(out.Allowed(), ", "))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc(flagOutput,
		func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return out.Allowed(), cobra.ShellCompDirectiveNoFileComp
		}))
}

// withConfig registers the --config persistent flag on cmd.
func withConfig(cmd *cobra.Command) {
	cmd.PersistentFlags().String(flagConfig, "",
		"Path to the config file (default $KARTA_CONFIG or $HOME/.karta/config.yaml)")
}

// withColor registers the --color persistent flag on cmd.
func withColor(cmd *cobra.Command) {
	cmd.PersistentFlags().String(flagColor, "auto", "Colorize output: auto, always, never")
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc(flagColor,
		func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return []string{"auto", "always", "never"}, cobra.ShellCompDirectiveNoFileComp
		}))
}

// colorFlag reads the persistent --color flag off the root command.
func colorFlag(cmd *cobra.Command) string {
	flag := cmd.Root().PersistentFlags().Lookup(flagColor)
	if flag == nil {
		return "auto"
	}
	return flag.Value.String()
}
