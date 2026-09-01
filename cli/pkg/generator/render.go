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

// componentsWidth caps the COMPONENTS cell so a workload declaring many
// components cannot push the later columns off screen.
const componentsWidth = 44

// Options controls how a set of workload views is rendered.
type Options struct {
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
		return marshal(out, views, opts.Output)
	}

	if len(views) == 0 {
		if opts.AllNamespaces {
			fmt.Fprintln(errOut, "No workloads found in any namespace.")
		} else {
			fmt.Fprintf(errOut, "No workloads found in namespace %s.\n", opts.Namespace)
		}
		return nil
	}
	return renderTable(out, views, opts)
}

func marshal(out io.Writer, views []workload.View, format Output) error {
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

	headers := []string{"NAME", "NAMESPACE", "PHASE", "COMPONENTS", "PODS", "GPUS", "AGE"}
	if opts.Output == OutputWide {
		headers = append(headers, "ORIGIN")
	}
	fmt.Fprintln(writer, strings.Join(headers, "\t"))

	now := time.Now()
	for _, view := range views {
		cells := append([]string{view.Name, view.Namespace},
			strings.Join(view.Phases, ","),
			components(view.Components),
			fmt.Sprintf("%d/%d", view.PodStats.PodsRunning, view.PodStats.PodsTotal),
			fmt.Sprintf("%d/%d", view.PodStats.AllocatedGPUs, view.GPUs),
			age(now, view.CreatedAt),
		)
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

// components renders the role(count) breakdown, eliding the tail once the cell
// grows past componentsWidth so the remaining columns stay aligned.
func components(views []workload.ComponentView) string {
	if len(views) == 0 {
		return "<none>"
	}

	var built strings.Builder
	for i, view := range views {
		entry := fmt.Sprintf("%s(%d)", view.Name, view.Replicas)
		// A multi-instance component takes its name from the instance key, which
		// on its own can outgrow the cell.
		if i == 0 && len(entry) > componentsWidth {
			entry = entry[:componentsWidth-3] + "..."
		}
		if i > 0 && built.Len()+len(entry)+2 > componentsWidth {
			fmt.Fprintf(&built, ", +%d more", len(views)-i)
			break
		}
		if i > 0 {
			built.WriteString(", ")
		}
		built.WriteString(entry)
	}
	return built.String()
}
