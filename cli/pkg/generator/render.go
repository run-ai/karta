// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/cli/pkg/workload"
)

// Options controls how a set of workload views is rendered.
type Options struct {
	// Output selects the format. The zero value renders the default table, so
	// Options{} is usable as-is.
	Output Output
	// Namespace is reported in the empty-result message.
	Namespace string
	// AllNamespaces drops the namespace from the empty-result message.
	AllNamespaces bool
}

// Render writes views to out. The empty-result notice goes to errOut so it
// cannot corrupt piped output.
func Render(out, errOut io.Writer, views []workload.View, opts Options) error {
	switch opts.Output {
	case OutputJSON, OutputYAML:
		// Always an array, unlike kubectl: consumers never branch on shape.
		if views == nil {
			views = []workload.View{}
		}
		return encode(out, views, opts.Output)
	// An unset Output is the caller not choosing, which the flag layer defaults
	// to a table. Only a value that is set and unrecognized is an error.
	case "", OutputTable, OutputWide:
		if len(views) == 0 {
			if opts.AllNamespaces {
				fmt.Fprintln(errOut, "No workloads found in any namespace.")
			} else {
				fmt.Fprintf(errOut, "No workloads found in namespace %s.\n", opts.Namespace)
			}
			return nil
		}
		return renderTable(out, views, opts)
	default:
		// The flag layer rejects an unknown format, so reaching this means a
		// programmatic caller passed one; a table would misrepresent the ask.
		return fmt.Errorf("unsupported output format %q", opts.Output)
	}
}

// encode writes views in a machine-readable format. Only the JSON branch adds a
// trailing newline; yaml.JSONToYAML already ends its output with one.
func encode(out io.Writer, views []workload.View, format Output) error {
	encoded, err := json.MarshalIndent(views, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workloads: %w", err)
	}
	if format == OutputYAML {
		if encoded, err = yaml.JSONToYAML(encoded); err != nil {
			return fmt.Errorf("encode workloads as yaml: %w", err)
		}
		_, err = out.Write(encoded)
		return err
	}
	_, err = fmt.Fprintf(out, "%s\n", encoded)
	return err
}

func renderTable(out io.Writer, views []workload.View, opts Options) error {
	writer := printers.GetNewTabWriter(out)

	headers := []string{"NAME", "NAMESPACE", "PHASE", "AGE"}
	if opts.Output == OutputWide {
		headers = append(headers, "ORIGIN")
	}
	fmt.Fprintln(writer, strings.Join(headers, "\t"))

	now := time.Now()
	for _, view := range views {
		cells := []string{
			view.Name,
			view.Namespace,
			strings.Join(view.Phases, ","),
			age(now, view.CreatedAt),
		}
		if opts.Output == OutputWide {
			cells = append(cells, view.Origin)
		}
		fmt.Fprintln(writer, strings.Join(cells, "\t"))
	}

	return writer.Flush()
}

// age formats a timestamp, unset for a workload resolved outside a live read.
func age(now, createdAt time.Time) string {
	if createdAt.IsZero() {
		return "<unknown>"
	}
	return duration.HumanDuration(now.Sub(createdAt))
}
