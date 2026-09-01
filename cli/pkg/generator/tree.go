// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator

import (
	"fmt"
	"io"
	"strings"

	"github.com/run-ai/karta/cli/pkg/workload"
)

// Tree writes view as an ASCII tree: one line per component or instance,
// one line per pod, indented to show the hierarchy.
func Tree(w io.Writer, view *workload.TreeView) error {
	header := fmt.Sprintf("%s/%s", view.Kind, view.Name)
	fmt.Fprintf(w, "%s [%s]\n", header, strings.Join(view.Phases, ","))

	nodes := visibleNodes(view.Nodes)
	for i, node := range nodes {
		writeTreeNode(w, node, "", i == len(nodes)-1)
	}
	return nil
}

// visibleNodes drops a leaf component the workload never instantiated
// (no live pods and no desired replicas), matching the "get" table's rule
// that a component absent from both spec and cluster does not render.
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

func writeTreeNode(w io.Writer, node workload.TreeNode, prefix string, last bool) {
	branch, childPrefix := "├─", prefix+"│ "
	if last {
		branch, childPrefix = "└─", prefix+"  "
	}

	fmt.Fprintf(w, "%s%s %s\n", prefix, branch, componentLine(node))

	for i, child := range node.Children {
		writeTreeNode(w, child, childPrefix, i == len(node.Children)-1)
	}
	if len(node.Children) == 0 {
		for i, pod := range node.Pods {
			writePodLine(w, pod, childPrefix, i == len(node.Pods)-1)
		}
	}
}

func componentLine(node workload.TreeNode) string {
	fields := []string{
		node.Name,
		fmt.Sprintf("(%d/%d replicas)", node.CurrentReplicas, node.DesiredReplicas),
		fmt.Sprintf("%d/%d ready", node.ReadyReplicas, node.CurrentReplicas),
		fmt.Sprintf("gpu: %d", node.GPUs),
	}
	if len(node.NodeNames) > 0 {
		fields = append(fields, strings.Join(node.NodeNames, ","))
	}
	return strings.Join(fields, "  ")
}

func writePodLine(w io.Writer, pod workload.PodNode, prefix string, last bool) {
	branch := "├─"
	if last {
		branch = "└─"
	}
	node := pod.Node
	if node == "" {
		node = "<none>"
	}
	fmt.Fprintf(w, "%s%s Pod/%s  %s  gpu: %d  %s\n", prefix, branch, pod.Name, pod.Phase, pod.GPUs, node)
}
