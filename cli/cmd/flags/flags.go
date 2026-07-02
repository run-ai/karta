// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package flags defines and registers the karta CLI's flags on cobra commands,
// backing them with the value structures in the flagtypes package.
package flags

import (
	"strings"

	"github.com/run-ai/karta/cli/internal/flagtypes"

	"github.com/spf13/cobra"
)

// WithKubeconfig registers the --kubeconfig persistent flag on cmd.
func WithKubeconfig(cmd *cobra.Command) {
	cmd.PersistentFlags().String("kubeconfig", "",
		"Path to the kubeconfig file to use (defaults to $KUBECONFIG or ~/.kube/config)")
}

// WithOutput registers the -o/--output enum persistent flag on cmd, backed by
// flagtypes.Output, along with its shell completion.
func WithOutput(cmd *cobra.Command) {
	out := flagtypes.NewOutput()
	cmd.PersistentFlags().VarP(out, "output", "o",
		"Output format: one of "+strings.Join(out.Allowed(), ", "))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("output",
		func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return out.Allowed(), cobra.ShellCompDirectiveNoFileComp
		}))
}

// WithNamespace registers the required -n/--namespace persistent flag on cmd
// (single-namespace MVP).
func WithNamespace(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("namespace", "n", "",
		"Namespace scope for workload commands (required)")
	cobra.CheckErr(cmd.MarkPersistentFlagRequired("namespace"))
}
