// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package workload holds the "karta workload" command tree: operational
// visibility into workloads running in a namespace.
package workload

import (
	"errors"

	"github.com/spf13/cobra"
)

// ErrNamespaceRequired is returned when a workload command runs without a
// namespace. The MVP is single-namespace only, so -n/--namespace is mandatory.
var ErrNamespaceRequired = errors.New("namespace is required: set -n/--namespace")

// NewCommand builds the "workload" command. Its PersistentPreRunE enforces the
// namespace requirement for every workload subcommand via inherited run hooks.
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "workload",
		Short: "Inspect workloads running in a namespace",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			namespace, err := cmd.Flags().GetString("namespace")
			if err != nil {
				return err
			}
			if namespace == "" {
				return ErrNamespaceRequired
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
}
