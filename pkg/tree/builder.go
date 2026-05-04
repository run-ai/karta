// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/jq/execution"
	"github.com/run-ai/karta/pkg/resource"
)

// Build constructs a WorkloadTree by walking the Karta definition's
// component hierarchy against a workload object and a list of candidate pods.
//
// The walk is top-down: the root component sets the workload status; each
// direct child of the root becomes a top-level ComponentNode in the tree.
// At each component the matcher is asked which pods belong to it, and the
// claimed pods are attached to the corresponding instance.
//
// Scope note (PoC): single-instance components are fully supported. Multi-
// instance (ComponentInstanceSelector) and ReplicaSelector flows are wired
// through but produce a single InstanceNode per component for now; richer
// instance splitting is a follow-up.
func Build(ctx context.Context, karta *v1alpha1.Karta, workload client.Object, pods []corev1.Pod, matcher PodMatcher) (*WorkloadTree, error) {
	if karta == nil {
		return nil, fmt.Errorf("karta definition must not be nil")
	}
	if workload == nil {
		return nil, fmt.Errorf("workload object must not be nil")
	}
	if matcher == nil {
		matcher = JQMatcher{}
	}

	runner := execution.NewDefaultRunner(workload)
	accessor := resource.NewAccessor(runner)
	factory := resource.NewComponentFactory(karta, accessor)

	root, err := factory.GetRootComponent()
	if err != nil {
		return nil, fmt.Errorf("get root component: %w", err)
	}

	tree := &WorkloadTree{}

	if status, err := root.GetStatus(ctx); err == nil && status != nil {
		tree.Status = workloadStatusFromResource(status)
	}

	rootDef := root.Definition()
	rootChildren, err := childDefinitionsOf(karta, rootDef.Name)
	if err != nil {
		return nil, err
	}

	for _, childDef := range rootChildren {
		comp, err := factory.GetComponent(childDef.Name)
		if err != nil {
			return nil, fmt.Errorf("get component %q: %w", childDef.Name, err)
		}
		node, err := buildComponentNode(ctx, factory, karta, comp, childDef, pods, matcher)
		if err != nil {
			return nil, err
		}
		tree.Children = append(tree.Children, node)
	}

	return tree, nil
}

// buildComponentNode produces a ComponentNode for a single component
// definition, attaching its extracted instances, claiming pods the matcher
// associates with this component, and recursing into any child components
// the Karta definition declares under this component.
func buildComponentNode(ctx context.Context, factory *resource.ComponentFactory, karta *v1alpha1.Karta, comp *resource.Component, def v1alpha1.ComponentDefinition, pods []corev1.Pod, matcher PodMatcher) (ComponentNode, error) {
	node := ComponentNode{
		Name: def.Name,
		Kind: def.Kind,
	}

	matched := make([]*corev1.Pod, 0, len(pods))
	for i := range pods {
		ok, err := matcher.Matches(ctx, &pods[i], &def)
		if err != nil {
			return ComponentNode{}, fmt.Errorf("match pod %q against component %q: %w", pods[i].Name, def.Name, err)
		}
		if ok {
			matched = append(matched, &pods[i])
		}
	}

	childDefs, err := childDefinitionsOf(karta, def.Name)
	if err != nil {
		return ComponentNode{}, err
	}

	children, err := buildChildren(ctx, factory, karta, childDefs, matched, matcher)
	if err != nil {
		return ComponentNode{}, err
	}

	instances, err := extractedInstancesOrEmpty(ctx, comp)
	if err != nil {
		return ComponentNode{}, err
	}

	if len(instances) == 0 {
		// Components with no extracted instances (logical groupings, or
		// components without a SpecDefinition) still get a single instance
		// node so callers can attach matched pods and child components.
		node.Instances = []InstanceNode{{Pods: matched, Children: children}}
		return node, nil
	}

	// PoC simplification: collapse all extracted instances into one
	// InstanceNode and attach all matched pods + recursed children to it.
	// Splitting via ComponentInstanceSelector is the follow-up step.
	first := pickFirstInstance(instances)
	scaleCopy := first.Scale
	instCopy := first
	node.Instances = []InstanceNode{{
		Scale:             scaleCopy,
		ExtractedInstance: &instCopy,
		Pods:              matched,
		Children:          children,
	}}

	return node, nil
}

// buildChildren recurses into the child definitions of a component, narrowing
// the candidate pod set so each child only sees pods its parent already
// claimed.
func buildChildren(ctx context.Context, factory *resource.ComponentFactory, karta *v1alpha1.Karta, childDefs []v1alpha1.ComponentDefinition, parentPods []*corev1.Pod, matcher PodMatcher) ([]ComponentNode, error) {
	if len(childDefs) == 0 {
		return nil, nil
	}
	candidatePods := make([]corev1.Pod, len(parentPods))
	for i, p := range parentPods {
		candidatePods[i] = *p
	}
	out := make([]ComponentNode, 0, len(childDefs))
	for _, def := range childDefs {
		comp, err := factory.GetComponent(def.Name)
		if err != nil {
			return nil, fmt.Errorf("get component %q: %w", def.Name, err)
		}
		child, err := buildComponentNode(ctx, factory, karta, comp, def, candidatePods, matcher)
		if err != nil {
			return nil, err
		}
		out = append(out, child)
	}
	return out, nil
}

// childDefinitionsOf returns the direct children of the named component
// from the Karta structure definition, in declaration order.
func childDefinitionsOf(karta *v1alpha1.Karta, parentName string) ([]v1alpha1.ComponentDefinition, error) {
	if karta == nil {
		return nil, fmt.Errorf("karta definition must not be nil")
	}
	var children []v1alpha1.ComponentDefinition
	for _, c := range karta.Spec.StructureDefinition.ChildComponents {
		if c.OwnerRef != nil && *c.OwnerRef == parentName {
			children = append(children, c)
		}
	}
	return children, nil
}

// extractedInstancesOrEmpty returns the extracted instances for a component,
// or an empty map when the component has no spec definition (in which case
// extraction is undefined and not an error).
func extractedInstancesOrEmpty(ctx context.Context, comp *resource.Component) (map[string]resource.ExtractedInstance, error) {
	if !comp.HasPodDefinition() {
		return nil, nil
	}
	return comp.GetExtractedInstances(ctx)
}

func pickFirstInstance(instances map[string]resource.ExtractedInstance) resource.ExtractedInstance {
	for _, v := range instances {
		return v
	}
	return resource.ExtractedInstance{}
}

func workloadStatusFromResource(s *resource.Status) WorkloadStatus {
	out := WorkloadStatus{}
	for _, m := range s.MatchedStatuses {
		out.Phases = append(out.Phases, string(m))
	}
	return out
}
