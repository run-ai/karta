// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"strings"

	"github.com/run-ai/karta/cli/pkg/cliflag"

	"github.com/spf13/cobra"
)

// withKubeconfig registers the --kubeconfig persistent flag on cmd.
func withKubeconfig(cmd *cobra.Command) {
	cmd.PersistentFlags().String("kubeconfig", "",
		"Path to the kubeconfig file to use (defaults to $KUBECONFIG or ~/.kube/config)")
}

// withOutput registers the -o/--output enum persistent flag on cmd, backed by
// cliflag.Output, along with its shell completion.
func withOutput(cmd *cobra.Command) {
	out := cliflag.NewOutput()
	cmd.PersistentFlags().VarP(out, "output", "o",
		"Output format: one of "+strings.Join(out.Allowed(), ", "))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("output",
		func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return out.Allowed(), cobra.ShellCompDirectiveNoFileComp
		}))
}

// withNamespace registers the -n/--namespace persistent flag on cmd.
func withNamespace(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("namespace", "n", "",
		"Namespace scope for workload commands")
}
