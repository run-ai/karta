// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/run-ai/karta/cli/pkg/generator"
)

const (
	flagConfig = "config"
	flagOutput = "output"
)

// ErrOutputFlagUnavailable reports -o/--output missing from a command's scope,
// a wiring mistake rather than anything a user can cause.
var ErrOutputFlagUnavailable = errors.New("output flag not available")

var kubeFlags *genericclioptions.ConfigFlags

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

// supportedOutput rejects a format the caller does not render. The allowed set
// is a parameter because commands differ. Validating before any cluster read
// keeps a bad format from costing a round trip.
func supportedOutput(cmd *cobra.Command, allowed ...generator.Output) (generator.Output, error) {
	format, err := outputFormat(cmd)
	if err != nil {
		return "", err
	}
	if !slices.Contains(allowed, format) {
		return "", usageError(cmd, unsupportedOutputError(format, allowed))
	}
	return format, nil
}

// withConfig registers the --config persistent flag on cmd.
func withConfig(cmd *cobra.Command) {
	cmd.PersistentFlags().String(flagConfig, "",
		"Path to the config file (default $KARTA_CONFIG or $HOME/.karta/config.yaml)")
}
