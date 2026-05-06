// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// visibleLen returns the number of visible terminal columns a string takes,
// approximated as the rune count. For ASCII this equals byte length; for the
// box-drawing glyphs in tree branches (│ ├ └ ─) it correctly returns 1 per
// glyph instead of the 3 bytes their UTF-8 encoding occupies.
func visibleLen(s string) int { return utf8.RuneCountInString(s) }

// columnGap is the number of spaces inserted between aligned columns.
// Two spaces is the kubectl convention; tight enough to feel dense,
// wide enough to read.
const columnGap = "  "

// Tree writes a styled ASCII workload tree to w. Pass PlainStyle() (or use
// AutoStyle on os.Stdout) to control whether ANSI color is emitted.
//
// Layout strategy:
//   - Per-sibling-group widths for name / replicas / ready / podname / phase
//     so siblings align without trailing whitespace bleeding across the tree
//     when names vary in length between branches.
//   - Global widths for the GPU and nodes/node columns so those right-side
//     fields land at the same X coordinate on every row, including across
//     component and pod rows. The renderer pads the leading section of each
//     row to a globally-computed target before writing gpu.
//
// Padding is computed from plain-text widths and applied as trailing
// spaces, so ANSI escape sequences inside styled segments don't affect
// the alignment math.
func Tree(w io.Writer, view WorkloadView, s Style) error {
	header := fmt.Sprintf("%s/%s", view.Kind, view.Name)
	fmt.Fprintf(w, "%s [%s]\n", s.Header(header), s.Phases(view.Phases))

	layout := computeLayout(view.Components, "")
	widths := computeComponentWidths(view.Components)
	for i, c := range view.Components {
		writeComponentAt(w, c, "", i == len(view.Components)-1, widths, layout, s)
	}
	return nil
}

// treeLayout captures the global X-coordinate targets that gpu and the
// trailing column should land on. maxLeading is the visible width from
// column 0 up to (but not including) the gpu cell; maxGPU is the gpu cell
// width itself, so the trailing nodes/node column lands at the same X for
// every row.
type treeLayout struct {
	maxLeading int
	maxGPU     int
}

// computeLayout walks the tree once and returns the global X-coordinate
// targets needed to align the gpu and trailing columns across both
// component and pod rows.
func computeLayout(comps []ComponentView, parentPrefix string) treeLayout {
	var l treeLayout
	if len(comps) == 0 {
		return l
	}
	cw := computeComponentWidths(comps)

	for i, c := range comps {
		branch := "├─"
		childPrefix := parentPrefix + "│ "
		if i == len(comps)-1 {
			branch = "└─"
			childPrefix = parentPrefix + "  "
		}

		compLead := visibleLen(parentPrefix) + visibleLen(branch) + 1 +
			cw.name + visibleLen(columnGap) +
			cw.replicas + visibleLen(columnGap) +
			cw.ready
		if compLead > l.maxLeading {
			l.maxLeading = compLead
		}
		if cw.gpu > l.maxGPU {
			l.maxGPU = cw.gpu
		}

		child := computeLayout(c.Children, childPrefix)
		if child.maxLeading > l.maxLeading {
			l.maxLeading = child.maxLeading
		}
		if child.maxGPU > l.maxGPU {
			l.maxGPU = child.maxGPU
		}

		if len(c.Pods) > 0 {
			pw := computePodWidths(c.Pods)
			for j := range c.Pods {
				podBranch := "├─"
				if j == len(c.Pods)-1 {
					podBranch = "└─"
				}
				podLead := visibleLen(childPrefix) + visibleLen(podBranch) + 1 +
					pw.name + visibleLen(columnGap) + pw.phase
				if podLead > l.maxLeading {
					l.maxLeading = podLead
				}
			}
			if pw.gpu > l.maxGPU {
				l.maxGPU = pw.gpu
			}
		}
	}
	return l
}

// componentColWidths captures the maximum plain-text length of each
// component-row column across a sibling set.
type componentColWidths struct {
	name, replicas, ready, gpu int
}

// podColWidths captures the maximum plain-text length of each pod-row
// column across a sibling set.
type podColWidths struct {
	name, phase, gpu int
}

func writeComponentAt(w io.Writer, c ComponentView, parentPrefix string, isLast bool, widths componentColWidths, layout treeLayout, s Style) {
	branch := "├─"
	childPrefix := parentPrefix + "│ "
	if isLast {
		branch = "└─"
		childPrefix = parentPrefix + "  "
	}

	namePlain, repPlain, readyPlain, gpuPlain := componentFields(c)
	nameStyled := padTo(s.Bold(s.Cyan(c.Name)), len(namePlain), widths.name)
	repStyled := padTo("("+s.Ratio(c.CurrentReplicas, c.DesiredReplicas, "replicas")+")", len(repPlain), widths.replicas)
	readyStyled := padTo(s.Ratio(c.ReadyCount, c.CurrentReplicas, "ready"), len(readyPlain), widths.ready)
	gpuStyled := padTo(gpuLabel(c.GPUs, s), len(gpuPlain), layout.maxGPU)

	leadingPlain := visibleLen(parentPrefix) + visibleLen(branch) + 1 +
		widths.name + visibleLen(columnGap) +
		widths.replicas + visibleLen(columnGap) +
		widths.ready
	leadingPad := layout.maxLeading - leadingPlain
	if leadingPad < 0 {
		leadingPad = 0
	}

	fmt.Fprintf(w, "%s%s %s%s%s%s%s%s%s%s%s\n",
		s.Dim(parentPrefix),
		s.Dim(branch),
		nameStyled, columnGap,
		repStyled, columnGap,
		readyStyled,
		strings.Repeat(" ", leadingPad)+columnGap,
		gpuStyled, columnGap,
		s.Dim("nodes: ")+nodeListColored(c.Nodes, s),
	)

	if len(c.Children) > 0 {
		childWidths := computeComponentWidths(c.Children)
		for j, child := range c.Children {
			writeComponentAt(w, child, childPrefix, j == len(c.Children)-1, childWidths, layout, s)
		}
	}

	if len(c.Pods) > 0 {
		podWidths := computePodWidths(c.Pods)
		for j, p := range c.Pods {
			writePod(w, p, childPrefix, j == len(c.Pods)-1, podWidths, layout, s)
		}
	}
}

func writePod(w io.Writer, p PodView, parentPrefix string, isLast bool, widths podColWidths, layout treeLayout, s Style) {
	branch := "├─"
	if isLast {
		branch = "└─"
	}
	namePlain, phasePlain, gpuPlain := podFields(p)
	nameStyled := padTo(s.Dim("Pod/")+p.Name, len(namePlain), widths.name)
	phaseStyled := padTo(s.Phase(p.Phase), len(phasePlain), widths.phase)
	gpuStyled := padTo(gpuLabel(p.GPUs, s), len(gpuPlain), layout.maxGPU)

	node := p.Node
	if node == "" {
		node = "<none>"
	}

	leadingPlain := visibleLen(parentPrefix) + visibleLen(branch) + 1 +
		widths.name + visibleLen(columnGap) + widths.phase
	leadingPad := layout.maxLeading - leadingPlain
	if leadingPad < 0 {
		leadingPad = 0
	}

	fmt.Fprintf(w, "%s%s %s%s%s%s%s%s%s\n",
		s.Dim(parentPrefix),
		s.Dim(branch),
		nameStyled, columnGap,
		phaseStyled,
		strings.Repeat(" ", leadingPad)+columnGap,
		gpuStyled, columnGap,
		s.Dim(node),
	)
}

func componentFields(c ComponentView) (name, replicas, ready, gpu string) {
	name = c.Name
	replicas = fmt.Sprintf("(%d/%d replicas)", c.CurrentReplicas, c.DesiredReplicas)
	ready = fmt.Sprintf("%d/%d ready", c.ReadyCount, c.CurrentReplicas)
	gpu = fmt.Sprintf("gpu: %d", c.GPUs)
	return
}

func podFields(p PodView) (name, phase, gpu string) {
	name = "Pod/" + p.Name
	phase = p.Phase
	gpu = fmt.Sprintf("gpu: %d", p.GPUs)
	return
}

func computeComponentWidths(comps []ComponentView) componentColWidths {
	var cw componentColWidths
	for _, c := range comps {
		n, r, rd, g := componentFields(c)
		cw.name = maxInt(cw.name, len(n))
		cw.replicas = maxInt(cw.replicas, len(r))
		cw.ready = maxInt(cw.ready, len(rd))
		cw.gpu = maxInt(cw.gpu, len(g))
	}
	return cw
}

func computePodWidths(pods []PodView) podColWidths {
	var pw podColWidths
	for _, p := range pods {
		n, ph, g := podFields(p)
		pw.name = maxInt(pw.name, len(n))
		pw.phase = maxInt(pw.phase, len(ph))
		pw.gpu = maxInt(pw.gpu, len(g))
	}
	return pw
}

// padTo right-pads styled with spaces so its visible width reaches w.
// plainLen is the visible length of styled (i.e. excluding ANSI escapes).
func padTo(styled string, plainLen, w int) string {
	if w <= plainLen {
		return styled
	}
	return styled + strings.Repeat(" ", w-plainLen)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func gpuLabel(n int64, s Style) string {
	if n == 0 {
		return s.Dim("gpu: 0")
	}
	return s.Dim("gpu: ") + s.Bold(s.Magenta(itoa(int(n))))
}

func nodeListColored(ns []string, s Style) string {
	if len(ns) == 0 {
		return s.Dim("<none>")
	}
	out := ""
	for i, n := range ns {
		if i > 0 {
			out += s.Dim(",")
		}
		out += n
	}
	return out
}
