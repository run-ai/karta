// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/run-ai/karta/cli/pkg/generator"
)

const (
	flagConfig = "config"
	flagOutput = "output"
)

var ErrOutputFlagUnavailable = errors.New("output flag not available")

var kubeFlags *genericclioptions.ConfigFlags

// withOutput registers the -o/--output enum on flags, backed by generator.Output,
// along with its shell completion. A command registering its own on cmd.Flags()
// shadows the root's for that command.
func withOutput(cmd *cobra.Command, flags *pflag.FlagSet, supportsWide bool) {
	out := NewOutputFlag(supportsWide)
	flags.VarP(out, flagOutput, "o",
		"Output format: one of "+strings.Join(out.Allowed(), ", "))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc(flagOutput,
		func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return out.Allowed(), cobra.ShellCompDirectiveNoFileComp
		}))
}

// outputFormat returns the typed value of -o/--output. Lookup resolves the flag
// wherever an ancestor registered it, so a leaf reads what the root parsed.
func outputFormat(cmd *cobra.Command) (generator.Output, error) {
	f := cmd.Flags().Lookup(flagOutput)
	if f == nil {
		return "", fmt.Errorf("%w: --%s is not registered", ErrOutputFlagUnavailable, flagOutput)
	}
	out, ok := f.Value.(*Enum[generator.Output])
	if !ok {
		return "", fmt.Errorf("%w: --%s is backed by %T", ErrOutputFlagUnavailable, flagOutput, f.Value)
	}
	return out.Get(), nil
}

// withConfig registers the --config persistent flag on cmd.
func withConfig(cmd *cobra.Command) {
	cmd.PersistentFlags().String(flagConfig, "",
		"Path to the config file (default $KARTA_CONFIG or $HOME/.karta/config.yaml)")
}
