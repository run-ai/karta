// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/run-ai/karta/cmd/karta/internal/definitions"
	"github.com/run-ai/karta/cmd/karta/internal/kube"
	"github.com/run-ai/karta/cmd/karta/internal/loader"
	"github.com/run-ai/karta/cmd/karta/internal/render"
	"github.com/run-ai/karta/pkg/tree"
)

func newWorkloadTreeCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "tree <name>",
		Short: "Render a workload as a hierarchical tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			name := args[0]

			client, err := kube.NewClient(opts.configFlags)
			if err != nil {
				return fmt.Errorf("failed to build kube client: %w", err)
			}

			registry, err := definitions.Load()
			if err != nil {
				return fmt.Errorf("load community Karta definitions: %w", err)
			}

			res, err := loader.FindWorkload(ctx, client, registry, client.Namespace(), name)
			if err != nil {
				return err
			}

			wt, err := tree.Build(ctx, res.Karta, res.Workload, res.Pods, tree.JQMatcher{})
			if err != nil {
				return fmt.Errorf("build workload tree: %w", err)
			}

			view := render.Build(wt, res.Workload.GetKind(), res.Workload.GetName(), res.Workload.GetNamespace())
			return render.Tree(c.OutOrStdout(), view, opts.styleFor(c.OutOrStdout()))
		},
	}
}
