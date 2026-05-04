// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"github.com/spf13/cobra"
)

func newWorkloadCmd(opts *rootOptions) *cobra.Command {
	c := &cobra.Command{
		Use:   "workload",
		Short: "Operate on workloads in the cluster",
	}

	c.AddCommand(newWorkloadListCmd(opts))
	c.AddCommand(newWorkloadTreeCmd(opts))

	return c
}
