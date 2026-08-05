// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

const (
	flagConfig     = "config"
	flagKubeconfig = "kubeconfig"
	flagNamespace  = "namespace"
	flagOutput     = "output"
)

// withKubeconfig registers the --kubeconfig persistent flag on cmd.
func withKubeconfig(cmd *cobra.Command) {
	cmd.PersistentFlags().String(flagKubeconfig, "",
		"Path to the kubeconfig file to use (defaults to $KUBECONFIG or ~/.kube/config)")
}

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

// withNamespace registers the -n/--namespace persistent flag on cmd.
func withNamespace(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP(flagNamespace, "n", "",
		"Namespace scope for workload commands")
}

// withConfig registers the --config persistent flag on cmd.
func withConfig(cmd *cobra.Command) {
	cmd.PersistentFlags().String(flagConfig, "",
		"Path to the config file (default $KARTA_CONFIG or $HOME/.karta/config.yaml)")
}
