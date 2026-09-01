// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/run-ai/karta/cli/pkg/physical"
	"github.com/run-ai/karta/cli/pkg/workload"
)

// maxDevicesPerRow caps how many device names a single tree row prints before
// collapsing the tail into "+N more". Eight is one full node's worth on the
// common GPU SKUs, which keeps a per-pod row readable.
const maxDevicesPerRow = 8

// columnGap is the number of spaces inserted between aligned columns.
const columnGap = "  "

func visibleLen(s string) int { return utf8.RuneCountInString(s) }

// Tree writes a styled ASCII workload tree to w. Pass PlainStyle() (or use
// AutoStyle on os.Stdout) to control whether ANSI color is emitted.
//
// Layout strategy: per-sibling-group widths for name/replicas/ready/podname/
// phase so siblings align without trailing whitespace bleeding across the
// tree, plus global widths for the GPU column so it lands at the same X
// coordinate on every row, including across component and pod rows.
func Tree(w io.Writer, view *workload.TreeView, s Style) error {
	header := fmt.Sprintf("%s/%s", view.Kind, view.Name)
	fmt.Fprintf(w, "%s [%s]\n", s.Header(header), s.Phases(view.Phases))

	nodes := visibleNodes(view.Nodes)
	layout := computeLayout(nodes)
	widths := computeNodeWidths(nodes)
	for i, node := range nodes {
		writeTreeNode(w, node, "", i == len(nodes)-1, widths, layout, s)
	}
	return nil
}

// visibleNodes drops a leaf component the workload never instantiated (no
// live pods and no desired replicas), matching the "get" table's rule that a
// component absent from both spec and cluster does not render.
func visibleNodes(nodes []workload.TreeNode) []workload.TreeNode {
	var out []workload.TreeNode
	for _, node := range nodes {
		if len(node.Children) == 0 && node.DesiredReplicas == 0 && node.CurrentReplicas == 0 {
			continue
		}
		node.Children = visibleNodes(node.Children)
		out = append(out, node)
	}
	return out
}

// treeLayout captures the global X-coordinate target the gpu column lands on.
type treeLayout struct {
	maxLeading int
	maxGPU     int
}

func computeLayout(nodes []workload.TreeNode) treeLayout {
	return computeLayoutAt(nodes, "")
}

func computeLayoutAt(nodes []workload.TreeNode, parentPrefix string) treeLayout {
	var l treeLayout
	if len(nodes) == 0 {
		return l
	}
	nw := computeNodeWidths(nodes)

	for i, n := range nodes {
		branch := "├─"
		childPrefix := parentPrefix + "│ "
		if i == len(nodes)-1 {
			branch = "└─"
			childPrefix = parentPrefix + "  "
		}

		lead := visibleLen(parentPrefix) + visibleLen(branch) + 1 +
			nw.name + visibleLen(columnGap) + nw.replicas + visibleLen(columnGap) + nw.ready
		l.maxLeading = maxInt(l.maxLeading, lead)
		l.maxGPU = maxInt(l.maxGPU, nw.gpu)

		child := computeLayoutAt(n.Children, childPrefix)
		l.maxLeading = maxInt(l.maxLeading, child.maxLeading)
		l.maxGPU = maxInt(l.maxGPU, child.maxGPU)

		if len(n.Children) == 0 && len(n.Pods) > 0 {
			pw := computePodWidths(n.Pods)
			for j := range n.Pods {
				podBranch := "├─"
				if j == len(n.Pods)-1 {
					podBranch = "└─"
				}
				podLead := visibleLen(childPrefix) + visibleLen(podBranch) + 1 +
					pw.name + visibleLen(columnGap) + pw.phase
				l.maxLeading = maxInt(l.maxLeading, podLead)
			}
			l.maxGPU = maxInt(l.maxGPU, pw.gpu)
		}
	}
	return l
}

type nodeColWidths struct{ name, replicas, ready, gpu int }
type podColWidths struct{ name, phase, gpu int }

func writeTreeNode(w io.Writer, n workload.TreeNode, parentPrefix string, last bool, widths nodeColWidths, layout treeLayout, s Style) {
	branch, childPrefix := "├─", parentPrefix+"│ "
	if last {
		branch, childPrefix = "└─", parentPrefix+"  "
	}

	namePlain, repPlain, readyPlain, gpuPlain := nodeFields(n)
	nameStyled := padTo(s.Bold(s.Cyan(n.Name)), len(namePlain), widths.name)
	repStyled := padTo("("+s.Ratio(n.CurrentReplicas, n.DesiredReplicas, "replicas")+")", len(repPlain), widths.replicas)
	readyStyled := padTo(s.Ratio(n.ReadyReplicas, n.CurrentReplicas, "ready"), len(readyPlain), widths.ready)
	gpuStyled := padTo(gpuLabel(n.GPUs, s), len(gpuPlain), layout.maxGPU)

	leadingPlain := visibleLen(parentPrefix) + visibleLen(branch) + 1 +
		widths.name + visibleLen(columnGap) + widths.replicas + visibleLen(columnGap) + widths.ready
	leadingPad := maxInt(0, layout.maxLeading-leadingPlain)

	fmt.Fprintf(w, "%s%s %s%s%s%s%s%s%s%s%s\n",
		s.Dim(parentPrefix), s.Dim(branch),
		nameStyled, columnGap,
		repStyled, columnGap,
		readyStyled,
		strings.Repeat(" ", leadingPad)+columnGap,
		gpuStyled, columnGap,
		nodeTrailing(n, s),
	)

	if len(n.Children) > 0 {
		childWidths := computeNodeWidths(n.Children)
		for j, child := range n.Children {
			writeTreeNode(w, child, childPrefix, j == len(n.Children)-1, childWidths, layout, s)
		}
		return
	}
	if len(n.Pods) > 0 {
		podWidths := computePodWidths(n.Pods)
		for j, p := range n.Pods {
			writePod(w, p, childPrefix, j == len(n.Pods)-1, podWidths, layout, s)
		}
	}
}

func writePod(w io.Writer, p workload.PodNode, parentPrefix string, last bool, widths podColWidths, layout treeLayout, s Style) {
	branch := "├─"
	if last {
		branch = "└─"
	}
	namePlain, phasePlain, gpuPlain := podFields(p)
	nameStyled := padTo(s.Dim("Pod/")+p.Name, len(namePlain), widths.name)
	phaseStyled := padTo(s.Phase(p.Phase), len(phasePlain), widths.phase)
	gpuStyled := padTo(gpuLabel(p.GPUs, s), len(gpuPlain), layout.maxGPU)

	leadingPlain := visibleLen(parentPrefix) + visibleLen(branch) + 1 + widths.name + visibleLen(columnGap) + widths.phase
	leadingPad := maxInt(0, layout.maxLeading-leadingPlain)

	fmt.Fprintf(w, "%s%s %s%s%s%s%s%s%s\n",
		s.Dim(parentPrefix), s.Dim(branch),
		nameStyled, columnGap,
		phaseStyled,
		strings.Repeat(" ", leadingPad)+columnGap,
		gpuStyled, columnGap,
		podTrailing(p, s),
	)
}

func nodeFields(n workload.TreeNode) (name, replicas, ready, gpu string) {
	return n.Name,
		fmt.Sprintf("(%d/%d replicas)", n.CurrentReplicas, n.DesiredReplicas),
		fmt.Sprintf("%d/%d ready", n.ReadyReplicas, n.CurrentReplicas),
		fmt.Sprintf("gpu: %d", n.GPUs)
}

func podFields(p workload.PodNode) (name, phase, gpu string) {
	return "Pod/" + p.Name, p.Phase, fmt.Sprintf("gpu: %d", p.GPUs)
}

func computeNodeWidths(nodes []workload.TreeNode) nodeColWidths {
	var cw nodeColWidths
	for _, n := range nodes {
		name, rep, ready, gpu := nodeFields(n)
		cw.name, cw.replicas, cw.ready, cw.gpu = maxInt(cw.name, len(name)), maxInt(cw.replicas, len(rep)), maxInt(cw.ready, len(ready)), maxInt(cw.gpu, len(gpu))
	}
	return cw
}

func computePodWidths(pods []workload.PodNode) podColWidths {
	var pw podColWidths
	for _, p := range pods {
		name, phase, gpu := podFields(p)
		pw.name, pw.phase, pw.gpu = maxInt(pw.name, len(name)), maxInt(pw.phase, len(phase)), maxInt(pw.gpu, len(gpu))
	}
	return pw
}

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

func nodeListDim(names []string, s Style) string {
	if len(names) == 0 {
		return s.Dim("<none>")
	}
	return s.Dim(strings.Join(names, ","))
}

// nodeTrailing renders the right-most cell of a component row: the node
// list, plus the physical annotations when Enrich has run.
func nodeTrailing(n workload.TreeNode, s Style) string {
	parts := []string{nodeListDim(n.NodeNames, s)}

	if len(n.DegradedNodes) > 0 {
		parts = append(parts, s.Red("!"+strings.Join(n.DegradedNodes, ",")+" degraded"))
	}
	if n.DeviceCount > 0 {
		parts = append(parts, s.Dim("dev: ")+itoa(n.DeviceCount))
	}
	if len(n.Domains) > 0 {
		domains := s.Dim("@") + strings.Join(n.Domains, ",")
		if workload.SplitAcrossDomains(n) {
			domains += " " + s.Yellow("(split)")
		}
		parts = append(parts, domains)
	}
	return strings.Join(parts, columnGap)
}

// podTrailing renders the right-most cell of a pod row: the node it landed
// on, its condition when unhealthy, the topology domain, and its devices.
func podTrailing(p workload.PodNode, s Style) string {
	node := p.Node
	if node == "" {
		node = "<none>"
	}
	parts := []string{s.Dim(node)}

	if p.NodeCondition != "" {
		parts = append(parts, s.Red("!"+p.NodeCondition))
	}
	if p.Domain != "" {
		parts = append(parts, s.Dim("@")+p.Domain)
	}
	if len(p.Devices) > 0 {
		parts = append(parts, s.Dim("dev: ")+physical.FormatDevices(p.Devices, maxDevicesPerRow))
	}
	return strings.Join(parts, columnGap)
}
