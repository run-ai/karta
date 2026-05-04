// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/run-ai/karta/cmd/karta/internal/definitions"
	"github.com/run-ai/karta/cmd/karta/internal/kube"
	"github.com/run-ai/karta/cmd/karta/internal/loader"
	"github.com/run-ai/karta/cmd/karta/internal/render"
	"github.com/run-ai/karta/pkg/tree"
)

func newWorkloadListCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workloads discovered via known Karta definitions",
		RunE: func(c *cobra.Command, _ []string) error {
			ctx := c.Context()

			client, err := kube.NewClient(opts.configFlags)
			if err != nil {
				return fmt.Errorf("failed to build kube client: %w", err)
			}

			registry, err := definitions.Load()
			if err != nil {
				return fmt.Errorf("load community Karta definitions: %w", err)
			}

			workloads, pods, err := loader.ListWorkloads(ctx, client, registry, client.Namespace())
			if err != nil {
				return err
			}

			rows := make([]render.ListRow, 0, len(workloads))
			for _, w := range workloads {
				wt, err := tree.Build(ctx, w.Karta, w.Workload, pods, tree.JQMatcher{})
				if err != nil {
					return fmt.Errorf("build tree for %s/%s: %w", w.Workload.GetKind(), w.Workload.GetName(), err)
				}
				view := render.Build(wt, w.Workload.GetKind(), w.Workload.GetName(), w.Workload.GetNamespace())
				rows = append(rows, render.ListRow{
					Namespace:  w.Workload.GetNamespace(),
					Name:       w.Workload.GetName(),
					Kind:       w.Workload.GetKind(),
					Phases:     view.Phases,
					Components: render.SummarizeComponents(view),
					GPU:        render.TotalGPUs(view),
					Age:        time.Since(w.Workload.GetCreationTimestamp().Time),
				})
			}

			sort.SliceStable(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
			return render.List(c.OutOrStdout(), rows, opts.styleFor(c.OutOrStdout()))
		},
	}
}
