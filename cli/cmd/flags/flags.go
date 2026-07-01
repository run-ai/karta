// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package flags defines and registers the karta CLI's flags on cobra commands,
// backing them with the value structures in the flagtypes package.
package flags

import (
	"github.com/run-ai/karta/cli/internal/flagtypes"

	"github.com/spf13/cobra"
)

// AddGlobals registers the karta binary's global persistent flags on cmd:
// --kubeconfig and the -o/--output enum (backed by flagtypes.Output).
func AddGlobals(cmd *cobra.Command) {
	fs := cmd.PersistentFlags()
	fs.String("kubeconfig", "",
		"Path to the kubeconfig file to use (defaults to $KUBECONFIG or ~/.kube/config)")
	fs.VarP(flagtypes.NewOutput(), "output", "o", "Output format: one of table, wide, json, yaml")
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("output",
		func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return flagtypes.OutputValues, cobra.ShellCompDirectiveNoFileComp
		}))
}

// AddNamespace registers the required -n/--namespace persistent flag for the
// workload command subtree (single-namespace MVP).
func AddNamespace(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP("namespace", "n", "",
		"Namespace scope for workload commands (required)")
	cobra.CheckErr(cmd.MarkPersistentFlagRequired("namespace"))
}
