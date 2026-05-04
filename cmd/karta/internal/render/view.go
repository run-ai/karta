// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package render computes the CLI display layer on top of pkg/tree's raw
// WorkloadTree. The split mirrors the HLD: pkg/tree is the shared data model
// every consumer can use; the display fields (ready counts, GPU sums,
// rendered tree text) are derived here on traversal.
package render

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/tree"
)

// WorkloadView is the display-shaped projection of a WorkloadTree, with all
// the aggregate fields the CLI needs to render.
type WorkloadView struct {
	Kind       string
	Name       string
	Namespace  string
	Phases     []string
	Components []ComponentView
}

// ComponentView holds the rendered fields for a tree component.
type ComponentView struct {
	Name            string
	DesiredReplicas int32
	CurrentReplicas int32
	ReadyCount      int32
	GPUs            int64
	Nodes           []string
	Pods            []PodView
	Children        []ComponentView
}

// PodView holds the rendered fields for a single pod under a component.
type PodView struct {
	Name  string
	Phase string
	Ready bool
	Node  string
	GPUs  int64
}

// Build computes a WorkloadView from a raw WorkloadTree and the kind / name /
// namespace pulled off the workload object.
func Build(t *tree.WorkloadTree, kind, name, namespace string) WorkloadView {
	wv := WorkloadView{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Phases:    append([]string(nil), t.Status.Phases...),
	}
	for _, c := range t.Children {
		wv.Components = append(wv.Components, buildComponent(c))
	}
	return wv
}

func buildComponent(c tree.ComponentNode) ComponentView {
	// Multi-instance components (Dynamo's "service" split into Frontend /
	// PrefillWorker / DecodeWorker) render as a parent with one synthetic
	// child per instance, so each instance gets its own replica count, GPU
	// roll-up, and pod list — matching the HLD's example output.
	if isMultiInstance(c) {
		return buildMultiInstanceComponent(c)
	}

	cv := ComponentView{Name: c.Name}
	nodeSet := map[string]struct{}{}

	var podsAll []*corev1.Pod
	for _, inst := range c.Instances {
		podsAll = append(podsAll, inst.Pods...)
		if inst.Scale != nil && inst.Scale.Replicas != nil {
			cv.DesiredReplicas += *inst.Scale.Replicas
		}
		for _, child := range inst.Children {
			cv.Children = append(cv.Children, buildComponent(child))
		}
	}
	for _, p := range podsAll {
		cv.CurrentReplicas++
		pv := podView(p)
		cv.GPUs += pv.GPUs
		if pv.Ready {
			cv.ReadyCount++
		}
		if pv.Node != "" {
			nodeSet[pv.Node] = struct{}{}
		}
		cv.Pods = append(cv.Pods, pv)
	}

	if cv.DesiredReplicas == 0 {
		cv.DesiredReplicas = cv.CurrentReplicas
	}

	for n := range nodeSet {
		cv.Nodes = append(cv.Nodes, n)
	}
	sort.Strings(cv.Nodes)
	sort.SliceStable(cv.Pods, func(i, j int) bool { return cv.Pods[i].Name < cv.Pods[j].Name })

	// When children exist they own pod-level rendering; clear the per-component
	// Pods list so we don't double-render under a parent that's just grouping.
	if len(cv.Children) > 0 {
		cv.Pods = nil
	}

	return cv
}

// isMultiInstance reports whether a ComponentNode carries the multi-instance
// shape: more than one InstanceNode, with at least one InstanceKey set.
func isMultiInstance(c tree.ComponentNode) bool {
	if len(c.Instances) <= 1 {
		return false
	}
	for _, inst := range c.Instances {
		if inst.InstanceKey != nil {
			return true
		}
	}
	return false
}

// buildMultiInstanceComponent flattens the InstanceNodes of a multi-instance
// component into per-instance ComponentViews, each rendered as if it were
// its own component. Replica counts, GPU sums, and node lists roll up to
// the parent so the table view sees an aggregate.
func buildMultiInstanceComponent(c tree.ComponentNode) ComponentView {
	parent := ComponentView{Name: c.Name}
	parentNodes := map[string]struct{}{}

	for _, inst := range c.Instances {
		if inst.InstanceKey == nil {
			continue
		}
		child := ComponentView{Name: *inst.InstanceKey}
		childNodes := map[string]struct{}{}

		if inst.Scale != nil && inst.Scale.Replicas != nil {
			child.DesiredReplicas = *inst.Scale.Replicas
		}
		for _, p := range inst.Pods {
			child.CurrentReplicas++
			pv := podView(p)
			child.GPUs += pv.GPUs
			if pv.Ready {
				child.ReadyCount++
			}
			if pv.Node != "" {
				childNodes[pv.Node] = struct{}{}
				parentNodes[pv.Node] = struct{}{}
			}
			child.Pods = append(child.Pods, pv)
		}
		if child.DesiredReplicas == 0 {
			child.DesiredReplicas = child.CurrentReplicas
		}
		for n := range childNodes {
			child.Nodes = append(child.Nodes, n)
		}
		sort.Strings(child.Nodes)
		sort.SliceStable(child.Pods, func(i, j int) bool { return child.Pods[i].Name < child.Pods[j].Name })

		parent.CurrentReplicas += child.CurrentReplicas
		parent.DesiredReplicas += child.DesiredReplicas
		parent.ReadyCount += child.ReadyCount
		parent.GPUs += child.GPUs
		parent.Children = append(parent.Children, child)
	}
	for n := range parentNodes {
		parent.Nodes = append(parent.Nodes, n)
	}
	sort.Strings(parent.Nodes)
	return parent
}

func podView(p *corev1.Pod) PodView {
	v := PodView{
		Name:  p.Name,
		Phase: string(p.Status.Phase),
		Node:  p.Spec.NodeName,
		Ready: isReady(p),
		GPUs:  gpuSum(p),
	}
	return v
}

func isReady(p *corev1.Pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func gpuSum(p *corev1.Pod) int64 {
	var total int64
	for _, c := range p.Spec.Containers {
		if q, ok := c.Resources.Limits["nvidia.com/gpu"]; ok {
			total += q.Value()
			continue
		}
		if q, ok := c.Resources.Requests["nvidia.com/gpu"]; ok {
			total += q.Value()
		}
	}
	return total
}

// PhasesString returns a single-line summary of phases, suitable for the
// header bracket. An empty phase set renders as "-".
func PhasesString(phases []string) string {
	if len(phases) == 0 {
		return "-"
	}
	return strings.Join(phases, ",")
}

// FormatNodes is a comma-joiner that returns "<none>" for empty input so
// pods without an assigned node render predictably.
func FormatNodes(ns []string) string {
	if len(ns) == 0 {
		return "<none>"
	}
	return strings.Join(ns, ",")
}

// nolint:unused // helper retained for future use when rendering rolls up
// per-instance pod details
func formatNumber(n int64) string { return fmt.Sprintf("%d", n) }

// SummarizeComponents flattens a WorkloadView's component tree to its
// leaf components — the ones that actually carry pods — and returns one
// ComponentSummary per leaf in declaration order. This is what the list
// view's COMPONENTS column displays. Logical grouping components (with
// children but no pods of their own) are skipped.
func SummarizeComponents(view WorkloadView) []ComponentSummary {
	var out []ComponentSummary
	for _, c := range view.Components {
		appendLeafSummaries(c, &out)
	}
	return out
}

func appendLeafSummaries(c ComponentView, out *[]ComponentSummary) {
	if len(c.Children) > 0 {
		for _, ch := range c.Children {
			appendLeafSummaries(ch, out)
		}
		return
	}
	*out = append(*out, ComponentSummary{Name: c.Name, CurrentReplicas: c.CurrentReplicas})
}

// TotalGPUs returns the sum of GPUs across the leaf components of a
// WorkloadView. Counting at leaves avoids double-counting when a parent
// aggregator already rolls its children up.
func TotalGPUs(view WorkloadView) int64 {
	var total int64
	for _, c := range view.Components {
		total += leafGPUSum(c)
	}
	return total
}

func leafGPUSum(c ComponentView) int64 {
	if len(c.Children) > 0 {
		var sum int64
		for _, ch := range c.Children {
			sum += leafGPUSum(ch)
		}
		return sum
	}
	return c.GPUs
}
