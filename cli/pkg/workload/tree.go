// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package workload

import (
	"context"
	"fmt"
	"slices"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/cli/pkg/definitions"
	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

// TreeView is a workload rendered as a hierarchical tree, with live pods
// attributed to the component and instance they actually belong to, using
// the Karta definition's own PodSelector rather than a generic heuristic.
type TreeView struct {
	Kind      string
	Name      string
	Namespace string
	Phases    []string
	Nodes     []TreeNode
}

// TreeNode is one component (or, for a multi-instance component, one
// instance) in the tree. A pure grouping component has no pod-bearing
// content of its own: Replicas is 0 and Pods is empty, only Children matter.
type TreeNode struct {
	Name            string
	Kind            string
	DesiredReplicas int32
	CurrentReplicas int32
	ReadyReplicas   int32
	GPUs            int64
	NodeNames       []string
	Pods            []PodNode
	Children        []TreeNode
}

// PodNode is one live pod attributed to a TreeNode.
type PodNode struct {
	Name  string
	Phase string
	Ready bool
	Node  string
	GPUs  int64
}

// ResolveTree builds obj's tree through def, attributing pods (already
// filtered to the workload, e.g. by PodAttributor's owner-chain walk) to the
// component and instance the Karta definition says they belong to.
func ResolveTree(
	ctx context.Context, obj *unstructured.Unstructured, def definitions.Definition, pods []corev1.Pod,
) (*TreeView, error) {
	factory := resource.NewComponentFactoryFromObject(def.Karta, obj)

	workloadTree, err := tree.Build(ctx, factory)
	if err != nil {
		return nil, fmt.Errorf("build tree: %w", err)
	}

	view := &TreeView{
		Kind:      obj.GetKind(),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Phases:    phases(workloadTree),
	}

	componentDefs := indexComponentDefs(def.Karta)

	// tree.Build drops the root, but Deployment, StatefulSet, Pod and Job
	// carry their pod template on it, so the root itself can be pod-bearing.
	root, err := factory.GetRootComponent()
	if err != nil {
		return nil, fmt.Errorf("get root component: %w", err)
	}
	if root.HasPodDefinition() {
		rootNode, err := buildLeafNode(ctx, root.Name(), kindOf(root.Kind()), componentDefs[root.Name()], pods)
		if err != nil {
			return nil, fmt.Errorf("build root node: %w", err)
		}
		view.Nodes = append(view.Nodes, rootNode)
	}

	for _, node := range workloadTree.Children {
		child, err := buildTreeNode(ctx, node, componentDefs, pods)
		if err != nil {
			return nil, fmt.Errorf("build component %q: %w", node.Name, err)
		}
		view.Nodes = append(view.Nodes, child)
	}
	return view, nil
}

func indexComponentDefs(karta *v1alpha1.Karta) map[string]v1alpha1.ComponentDefinition {
	defs := make(map[string]v1alpha1.ComponentDefinition, len(karta.Spec.StructureDefinition.ChildComponents)+1)
	defs[karta.Spec.StructureDefinition.RootComponent.Name] = karta.Spec.StructureDefinition.RootComponent
	for _, child := range karta.Spec.StructureDefinition.ChildComponents {
		defs[child.Name] = child
	}
	return defs
}

// buildTreeNode renders one ComponentNode. A grouping component (no pod
// definition) only recurses; a pod-bearing one matches live pods against its
// PodSelector and, when multi-instance, splits them per instance.
func buildTreeNode(
	ctx context.Context, node tree.ComponentNode, defs map[string]v1alpha1.ComponentDefinition, pods []corev1.Pod,
) (TreeNode, error) {
	kind := kindOf(node.Kind)

	if !node.HasPodDefinition {
		tn := TreeNode{Name: node.Name, Kind: kind}
		for _, inst := range node.Instances {
			for _, child := range inst.Children {
				grandchild, err := buildTreeNode(ctx, child, defs, pods)
				if err != nil {
					return TreeNode{}, err
				}
				tn.Children = append(tn.Children, grandchild)
			}
		}
		return tn, nil
	}

	def := defs[node.Name]
	matched, err := matchComponentType(ctx, def.PodSelector, pods)
	if err != nil {
		return TreeNode{}, fmt.Errorf("match pods to component %q: %w", node.Name, err)
	}

	if !isMultiInstance(node.Instances) {
		return buildInstanceNode(ctx, node.Name, kind, def, node.Instances, matched, defs)
	}

	// Multi-instance: each InstanceNode becomes its own labeled child, its
	// pods narrowed by ComponentInstanceSelector or ReplicaSelector.
	parent := TreeNode{Name: node.Name, Kind: kind}
	for _, inst := range node.Instances {
		instPods, err := filterByInstance(ctx, def.PodSelector, inst, matched)
		if err != nil {
			return TreeNode{}, fmt.Errorf("match pods to instance of %q: %w", node.Name, err)
		}
		child, err := buildInstanceNode(ctx, instanceLabel(node.Name, inst), kind, def, []tree.InstanceNode{inst}, instPods, defs)
		if err != nil {
			return TreeNode{}, err
		}
		parent.Children = append(parent.Children, child)
		parent.DesiredReplicas += child.DesiredReplicas
		parent.CurrentReplicas += child.CurrentReplicas
		parent.ReadyReplicas += child.ReadyReplicas
		parent.GPUs += child.GPUs
		parent.NodeNames = mergeNodeNames(parent.NodeNames, child.NodeNames)
	}
	return parent, nil
}

// buildInstanceNode renders the single instance of a pod-bearing component:
// its own pods (leaf) plus any descendants.
func buildInstanceNode(
	ctx context.Context,
	name, kind string,
	def v1alpha1.ComponentDefinition,
	instances []tree.InstanceNode,
	pods []corev1.Pod,
	defs map[string]v1alpha1.ComponentDefinition,
) (TreeNode, error) {
	tn, err := buildLeafNode(ctx, name, kind, def, pods)
	if err != nil {
		return TreeNode{}, err
	}
	for _, inst := range instances {
		if inst.Scale != nil && inst.Scale.Replicas != nil {
			tn.DesiredReplicas = *inst.Scale.Replicas
		}
		for _, child := range inst.Children {
			grandchild, err := buildTreeNode(ctx, child, defs, pods)
			if err != nil {
				return TreeNode{}, err
			}
			tn.Children = append(tn.Children, grandchild)
		}
	}
	return tn, nil
}

// buildLeafNode turns a pod set already matched to one component (or one
// component's PodSelector, when called for the root) into its TreeNode.
func buildLeafNode(ctx context.Context, name, kind string, def v1alpha1.ComponentDefinition, pods []corev1.Pod) (TreeNode, error) {
	matched := pods
	if len(matched) == 0 {
		matched, _ = matchComponentType(ctx, def.PodSelector, pods)
	}

	tn := TreeNode{Name: name, Kind: kind, DesiredReplicas: int32(len(matched))}
	for i := range matched {
		pod := &matched[i]
		tn.CurrentReplicas++
		podGPUs := podGPUs(pod.Spec)
		tn.GPUs += podGPUs
		ready := isPodReady(pod)
		if ready {
			tn.ReadyReplicas++
		}
		if pod.Spec.NodeName != "" {
			tn.NodeNames = mergeNodeNames(tn.NodeNames, []string{pod.Spec.NodeName})
		}
		tn.Pods = append(tn.Pods, PodNode{
			Name: pod.Name, Phase: string(pod.Status.Phase), Ready: ready, Node: pod.Spec.NodeName, GPUs: podGPUs,
		})
	}
	sort.Slice(tn.Pods, func(i, j int) bool { return tn.Pods[i].Name < tn.Pods[j].Name })
	return tn, nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func mergeNodeNames(existing, add []string) []string {
	merged := append(slices.Clone(existing), add...)
	slices.Sort(merged)
	return slices.Compact(merged)
}

// isMultiInstance mirrors the CLI table view's rule: more than one instance,
// with at least one carrying an InstanceKey or ReplicaKey.
func isMultiInstance(instances []tree.InstanceNode) bool {
	if len(instances) <= 1 {
		return false
	}
	for _, inst := range instances {
		if inst.InstanceKey != nil || inst.ReplicaKey != nil {
			return true
		}
	}
	return false
}

func instanceLabel(componentName string, inst tree.InstanceNode) string {
	switch {
	case inst.InstanceKey != nil:
		return *inst.InstanceKey
	case inst.ReplicaKey != nil:
		return fmt.Sprintf("%s[%s]", componentName, *inst.ReplicaKey)
	default:
		return componentName
	}
}

// matchComponentType filters pods to those the component's ComponentTypeSelector
// accepts. A nil selector means the component does not discriminate by pod
// content, so every candidate pod is assumed to belong to it (correct for a
// workload's only pod-bearing component, e.g. a Deployment's root).
func matchComponentType(ctx context.Context, selector *v1alpha1.PodSelector, pods []corev1.Pod) ([]corev1.Pod, error) {
	if selector == nil || selector.ComponentTypeSelector == nil {
		return pods, nil
	}
	var matched []corev1.Pod
	for i := range pods {
		ok, err := resource.NewPodQuerier(&pods[i]).MatchesComponentType(ctx, selector.ComponentTypeSelector)
		if err != nil {
			return nil, err
		}
		if ok {
			matched = append(matched, pods[i])
		}
	}
	return matched, nil
}

// filterByInstance narrows pods already matched to a component down to the
// ones belonging to one specific instance, via ComponentInstanceSelector or
// ReplicaSelector. Neither selector set means the instances cannot be told
// apart from pod content alone, so every pod is handed to every instance;
// buildTreeNode only takes this path when at least one axis is present.
func filterByInstance(
	ctx context.Context, selector *v1alpha1.PodSelector, inst tree.InstanceNode, pods []corev1.Pod,
) ([]corev1.Pod, error) {
	if selector == nil {
		return pods, nil
	}
	var matched []corev1.Pod
	for i := range pods {
		querier := resource.NewPodQuerier(&pods[i])
		if inst.InstanceKey != nil {
			id, found, err := querier.ExtractInstanceId(ctx, selector.ComponentInstanceSelector)
			if err != nil {
				return nil, err
			}
			if !found || id != *inst.InstanceKey {
				continue
			}
		}
		if inst.ReplicaKey != nil {
			key, found, err := querier.ExtractReplicaKey(ctx, selector.ReplicaSelector)
			if err != nil {
				return nil, err
			}
			if !found || key != *inst.ReplicaKey {
				continue
			}
		}
		matched = append(matched, pods[i])
	}
	return matched, nil
}
