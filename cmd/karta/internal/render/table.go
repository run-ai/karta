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
	Name            string
	CurrentReplicas int32
}

// List writes the workload list table to w. Columns mirror the HLD example.
// Color escape sequences (when the style emits any) are wrapped in
// tabwriter's escape byte so column alignment stays correct.
func List(w io.Writer, rows []ListRow, s Style) error {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', tabwriter.StripEscape)
	headers := []string{"NAMESPACE", "NAME", "KIND", "PHASE", "COMPONENTS", "GPU", "AGE"}
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, escapeAnsi(s.Bold(h), s))
	}
	fmt.Fprintln(tw)

	for _, r := range rows {
		fields := []string{
			r.Namespace,
			s.Bold(r.Name),
			s.Cyan(r.Kind),
			s.Phases(r.Phases),
			componentsColored(r.Components, s),
			gpuTableCell(r.GPU, s),
			s.Dim(formatAge(r.Age)),
		}
		for i, f := range fields {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, escapeAnsi(f, s))
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

// escapeAnsi wraps every ANSI escape sequence in the tabwriter Escape byte
// (\xff) so tabwriter measures the visible width without counting the
// non-printing color codes.
func escapeAnsi(text string, s Style) string {
	if !s.enabled || !strings.Contains(text, "\x1b[") {
		return text
	}
	var out strings.Builder
	out.Grow(len(text) + 8)
	i := 0
	for i < len(text) {
		next := strings.Index(text[i:], "\x1b[")
		if next < 0 {
			out.WriteString(text[i:])
			break
		}
		out.WriteString(text[i : i+next])
		// Find the end of this CSI sequence — terminator is the byte 0x40-0x7E.
		j := i + next + 2
		for j < len(text) {
			b := text[j]
			j++
			if b >= 0x40 && b <= 0x7E {
				break
			}
		}
		out.WriteByte(0xff)
		out.WriteString(text[i+next : j])
		out.WriteByte(0xff)
		i = j
	}
	return out.String()
}

func componentsColored(comps []ComponentSummary, s Style) string {
	if len(comps) == 0 {
		return s.Dim("-")
	}
	parts := make([]string, 0, len(comps))
	for _, c := range comps {
		parts = append(parts, s.Cyan(c.Name)+s.Dim(fmt.Sprintf("(%d)", c.CurrentReplicas)))
	}
	return strings.Join(parts, s.Dim(", "))
}

func gpuTableCell(n int64, s Style) string {
	if n == 0 {
		return s.Dim("0")
	}
	return s.Bold(s.Magenta(fmt.Sprintf("%d", n)))
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
