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

// newVersionCommand builds the "karta version" subcommand.
func newVersionCommand(out *Enum[generator.Output]) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the karta version",
		Args:  cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Best effort. buildConfig applies the merged output preference to
			// the -o flag; a broken config must not stop version from reporting
			// the build, so the error is deliberately dropped.
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
			case generator.OutputTable, generator.OutputWide:
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				for _, row := range [][2]string{
					{"Version", info.Version},
					{"Commit", info.Commit},
					{"Build Date", info.BuildDate},
					{"Go Version", info.GoVersion},
					{"Platform", info.Platform},
				} {
					// Write errors surface from Flush.
					_, _ = fmt.Fprintf(tw, "%s:\t%s\n", row[0], row[1])
				}
				return tw.Flush()
			default:
				return fmt.Errorf("unsupported output format %q", format)
			}
		},
	}
}
