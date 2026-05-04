// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package render

import (
	"fmt"
	"io"
)

// Tree writes a styled ASCII workload tree to w. Pass PlainStyle() (or use
// AutoStyle on os.Stdout) to control whether ANSI color is emitted.
func Tree(w io.Writer, view WorkloadView, s Style) error {
	header := fmt.Sprintf("%s/%s", view.Kind, view.Name)
	fmt.Fprintf(w, "%s [%s]\n", s.Header(header), s.Phases(view.Phases))
	for i, c := range view.Components {
		isLast := i == len(view.Components)-1
		writeComponentAt(w, c, "", isLast, s)
	}
	return nil
}

func writeComponentAt(w io.Writer, c ComponentView, parentPrefix string, isLast bool, s Style) {
	branch := "├──"
	childPrefix := parentPrefix + "│   "
	if isLast {
		branch = "└──"
		childPrefix = parentPrefix + "    "
	}

	fmt.Fprintf(w, "%s%s %s   %s   %s   %s   %s\n",
		s.Dim(parentPrefix),
		s.Dim(branch),
		s.Bold(s.Cyan(c.Name)),
		"("+s.Ratio(c.CurrentReplicas, c.DesiredReplicas, "replicas")+")",
		s.Ratio(c.ReadyCount, c.CurrentReplicas, "ready"),
		gpuLabel(c.GPUs, s),
		s.Dim("nodes: ")+nodeListColored(c.Nodes, s),
	)

	for j, child := range c.Children {
		writeComponentAt(w, child, childPrefix, j == len(c.Children)-1, s)
	}
	for j, p := range c.Pods {
		podBranch := "├──"
		if j == len(c.Pods)-1 {
			podBranch = "└──"
		}
		node := p.Node
		if node == "" {
			node = s.Dim("<none>")
		} else {
			node = s.Dim(node)
		}
		fmt.Fprintf(w, "%s%s %s    %s   %s   %s\n",
			s.Dim(childPrefix),
			s.Dim(podBranch),
			s.Dim("Pod/")+p.Name,
			s.Phase(p.Phase),
			gpuLabel(p.GPUs, s),
			node,
		)
	}
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
