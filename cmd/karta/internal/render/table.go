// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
)

// ListRow is one row in the workload list table — one Karta-aware workload
// in a namespace, summarized for the operational overview.
type ListRow struct {
	Namespace  string
	Name       string
	Kind       string
	Phases     []string
	Components []ComponentSummary
	GPU        int64
	Age        time.Duration
}

// ComponentSummary captures the per-component counts displayed in the
// COMPONENTS column of `karta workload list`. Format: "name(currentReplicas)".
type ComponentSummary struct {
	Name             string
	CurrentReplicas  int32
}

// List writes the workload list table to w. Columns mirror the HLD example.
func List(w io.Writer, rows []ListRow) error {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "NAMESPACE\tNAME\tKIND\tPHASE\tCOMPONENTS\tGPU\tAGE")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			r.Namespace, r.Name, r.Kind,
			PhasesString(r.Phases),
			formatComponents(r.Components),
			r.GPU,
			formatAge(r.Age),
		)
	}
	return tw.Flush()
}

func formatComponents(comps []ComponentSummary) string {
	if len(comps) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(comps))
	for _, c := range comps {
		parts = append(parts, fmt.Sprintf("%s(%d)", c.Name, c.CurrentReplicas))
	}
	return strings.Join(parts, ", ")
}

func formatAge(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
