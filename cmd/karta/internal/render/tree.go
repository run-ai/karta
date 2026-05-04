// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"fmt"
	"io"
)

// Tree writes an ASCII workload tree to w. Output mirrors the HLD example:
//
//	PyTorchJob/llama-finetune [Running]
//	├── master   (1/1 replicas)   1/1 ready   gpu: 1    nodes: node-01
//	│   └── Pod/llama-finetune-master-0    Running   gpu: 1   node-01
//	└── worker   (4/4 replicas)   3/4 ready   gpu: 32   nodes: node-02,03,04
func Tree(w io.Writer, view WorkloadView) error {
	fmt.Fprintf(w, "%s/%s [%s]\n", view.Kind, view.Name, PhasesString(view.Phases))
	for i, c := range view.Components {
		isLast := i == len(view.Components)-1
		writeComponent(w, c, isLast)
	}
	return nil
}

func writeComponent(w io.Writer, c ComponentView, isLast bool) {
	writeComponentAt(w, c, "", isLast)
}

func writeComponentAt(w io.Writer, c ComponentView, parentPrefix string, isLast bool) {
	branch := "├──"
	childPrefix := parentPrefix + "│   "
	if isLast {
		branch = "└──"
		childPrefix = parentPrefix + "    "
	}
	fmt.Fprintf(w, "%s%s %s   (%d/%d replicas)   %d/%d ready   gpu: %d   nodes: %s\n",
		parentPrefix, branch, c.Name,
		c.CurrentReplicas, c.DesiredReplicas,
		c.ReadyCount, c.CurrentReplicas,
		c.GPUs,
		FormatNodes(c.Nodes),
	)
	for j, child := range c.Children {
		writeComponentAt(w, child, childPrefix, j == len(c.Children)-1)
	}
	for j, p := range c.Pods {
		podBranch := "├──"
		if j == len(c.Pods)-1 {
			podBranch = "└──"
		}
		node := p.Node
		if node == "" {
			node = "<none>"
		}
		fmt.Fprintf(w, "%s%s Pod/%s    %s   gpu: %d   %s\n",
			childPrefix, podBranch,
			p.Name, p.Phase, p.GPUs, node,
		)
	}
}
