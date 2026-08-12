// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/cli/pkg/generator"
	"github.com/run-ai/karta/pkg/version"
)

func newVersionCommand(out *Enum[generator.Output]) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the cli version",
		Args:  cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// config may set the output format; do not fail on error.
			if cfg, err := buildConfig(cmd); err == nil {
				config = cfg
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := version.Get()
			w := cmd.OutOrStdout()

			switch format := out.Get(); format {
			case generator.OutputJSON:
				b, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal version: %w", err)
				}
				_, err = fmt.Fprintf(w, "%s\n", b)
				return err
			case generator.OutputYAML:
				b, err := yaml.Marshal(info)
				if err != nil {
					return fmt.Errorf("marshal version: %w", err)
				}
				_, err = w.Write(b)
				return err
			case generator.OutputWide:
				return fmt.Errorf("output format %q is not supported by the version command; use table, json, or yaml", format)
			case generator.OutputTable:
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				for _, row := range [][2]string{
					{"Version", info.Version},
					{"Commit", info.Commit},
					{"Build Date", info.BuildDate},
					{"Go Version", info.GoVersion},
					{"Platform", info.Platform},
				} {
					_, _ = fmt.Fprintf(tw, "%s:\t%s\n", row[0], row[1])
				}
				return tw.Flush()
			default:
				return fmt.Errorf("unsupported output format %q", format)
			}
		},
	}
}
