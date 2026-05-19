// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ansiCSIRe matches ANSI CSI escape sequences (SGR color codes etc.) so we
// can compute the visible width of a styled cell. Go's text/tabwriter does
// not exclude content inside its Escape brackets from width calculation, so
// we pad columns ourselves.
var ansiCSIRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func visibleWidth(s string) int {
	return utf8.RuneCountInString(ansiCSIRe.ReplaceAllString(s, ""))
}

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

// List writes the workload list table to w. Columns are padded by visible
// width so ANSI color codes don't throw off alignment.
func List(w io.Writer, rows []ListRow, s Style) error {
	headers := []string{"NAMESPACE", "NAME", "KIND", "PHASE", "COMPONENTS", "GPU", "AGE"}
	cells := make([][]string, 0, len(rows)+1)

	headerRow := make([]string, len(headers))
	for i, h := range headers {
		headerRow[i] = s.Bold(h)
	}
	cells = append(cells, headerRow)

	for _, r := range rows {
		cells = append(cells, []string{
			r.Namespace,
			s.Bold(r.Name),
			s.Cyan(r.Kind),
			s.Phases(r.Phases),
			componentsColored(r.Components, s),
			gpuTableCell(r.GPU, s),
			s.Dim(formatAge(r.Age)),
		})
	}

	widths := make([]int, len(headers))
	for _, row := range cells {
		for i, c := range row {
			if vw := visibleWidth(c); vw > widths[i] {
				widths[i] = vw
			}
		}
	}

	const interColPadding = 3
	var buf strings.Builder
	for _, row := range cells {
		for i, c := range row {
			buf.WriteString(c)
			if i < len(row)-1 {
				buf.WriteString(strings.Repeat(" ", widths[i]-visibleWidth(c)+interColPadding))
			}
		}
		buf.WriteByte('\n')
	}
	_, err := io.WriteString(w, buf.String())
	return err
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
