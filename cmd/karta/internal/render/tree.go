// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"fmt"
	"io"
	"strings"
)

// columnGap is the number of spaces inserted between aligned columns.
// Two spaces is the kubectl convention; tight enough to feel dense,
// wide enough to read.
const columnGap = "  "

// Tree writes a styled ASCII workload tree to w. Pass PlainStyle() (or use
// AutoStyle on os.Stdout) to control whether ANSI color is emitted.
//
// Within each sibling group, columns are pre-measured and padded so that
// rows under the same parent line up. Indentation uses the kubectl-tree
// style (2-char indent + 2-char branch glyph) for tighter horizontal use.
//
// Padding is computed from plain-text widths and applied as trailing
// spaces, so ANSI escape sequences inside styled segments don't affect
// the alignment math.
func Tree(w io.Writer, view WorkloadView, s Style) error {
	header := fmt.Sprintf("%s/%s", view.Kind, view.Name)
	fmt.Fprintf(w, "%s [%s]\n", s.Header(header), s.Phases(view.Phases))

	widths := computeComponentWidths(view.Components)
	for i, c := range view.Components {
		writeComponentAt(w, c, "", i == len(view.Components)-1, widths, s)
	}
	return nil
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

func writeComponentAt(w io.Writer, c ComponentView, parentPrefix string, isLast bool, widths componentColWidths, s Style) {
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
	gpuStyled := padTo(gpuLabel(c.GPUs, s), len(gpuPlain), widths.gpu)

	fmt.Fprintf(w, "%s%s %s%s%s%s%s%s%s%s%s\n",
		s.Dim(parentPrefix),
		s.Dim(branch),
		nameStyled, columnGap,
		repStyled, columnGap,
		readyStyled, columnGap,
		gpuStyled, columnGap,
		s.Dim("nodes: ")+nodeListColored(c.Nodes, s),
	)

	if len(c.Children) > 0 {
		childWidths := computeComponentWidths(c.Children)
		for j, child := range c.Children {
			writeComponentAt(w, child, childPrefix, j == len(c.Children)-1, childWidths, s)
		}
	}

	if len(c.Pods) > 0 {
		podWidths := computePodWidths(c.Pods)
		for j, p := range c.Pods {
			writePod(w, p, childPrefix, j == len(c.Pods)-1, podWidths, s)
		}
	}
}

func writePod(w io.Writer, p PodView, parentPrefix string, isLast bool, widths podColWidths, s Style) {
	branch := "├─"
	if isLast {
		branch = "└─"
	}
	namePlain, phasePlain, gpuPlain := podFields(p)
	nameStyled := padTo(s.Dim("Pod/")+p.Name, len(namePlain), widths.name)
	phaseStyled := padTo(s.Phase(p.Phase), len(phasePlain), widths.phase)
	gpuStyled := padTo(gpuLabel(p.GPUs, s), len(gpuPlain), widths.gpu)

	node := p.Node
	if node == "" {
		node = "<none>"
	}

	fmt.Fprintf(w, "%s%s %s%s%s%s%s%s%s\n",
		s.Dim(parentPrefix),
		s.Dim(branch),
		nameStyled, columnGap,
		phaseStyled, columnGap,
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
