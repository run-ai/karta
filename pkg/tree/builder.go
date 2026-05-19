// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"

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
func Build(ctx context.Context, karta *v1alpha1.Karta, workload resource.KubernetesObject, pods []corev1.Pod, matcher PodMatcher) (*WorkloadTree, error) {
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
//
// Three instance-shaping paths exist, evaluated in order:
//
//  1. ReplicaSelector — split pods into one InstanceNode per replica index
//     and recurse children per-replica so descendants stay replica-scoped
//     (LWS `group` does this).
//  2. ComponentInstanceSelector — one InstanceNode per extracted instance,
//     routing pods by the selector's idPath (Dynamo `service` does this).
//  3. Single instance — attach every matched pod and the (shared) recursed
//     children to one InstanceNode.
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

	instances, err := extractedInstancesOrEmpty(ctx, comp)
	if err != nil {
		return ComponentNode{}, err
	}

	// Path 1: ReplicaSelector splits the component into per-replica subtrees.
	// Each replica's children are rebuilt against only that replica's pods,
	// keeping leaf attribution scoped (LWS group-0's leader doesn't see
	// group-1's pods). When componentInstanceSelector is also present (the
	// Dynamo case where replicaSelector is wired through grove for future
	// use), we prefer the instance split — replica-within-instance is a
	// follow-up.
	if repSel := replicaSelector(def); repSel != nil && componentInstanceSelector(def) == nil {
		node.Instances, err = buildReplicaScoped(ctx, factory, karta, def, matched, childDefs, instances, repSel, matcher)
		if err != nil {
			return ComponentNode{}, err
		}
		return node, nil
	}

	children, err := buildChildren(ctx, factory, karta, childDefs, matched, matcher)
	if err != nil {
		return ComponentNode{}, err
	}

	// For non-leaf components with no componentTypeSelector, the matcher's
	// permissive fallback would over-claim pods that belong to unrelated
	// workloads sharing the same namespace. Re-narrow to the union of pods
	// any descendant claimed: a parent only owns what its children own.
	if len(children) > 0 && !hasComponentTypeSelector(def) {
		matched = collectDescendantPods(children)
	}

	if len(instances) == 0 {
		node.Instances = []InstanceNode{{Pods: matched, Children: children}}
		return node, nil
	}

	// Path 2: ComponentInstanceSelector — one InstanceNode per extracted
	// instance, routed by the selector's idPath.
	if instSel := componentInstanceSelector(def); instSel != nil && len(instances) > 1 {
		node.Instances, err = buildMultiInstance(ctx, instances, matched, instSel)
		if err != nil {
			return ComponentNode{}, err
		}
		return node, nil
	}

	// Path 3: Single instance.
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

// replicaSelector returns the selector responsible for splitting pods across
// replicas of a component, when the definition declares one.
func replicaSelector(def v1alpha1.ComponentDefinition) *v1alpha1.ReplicaSelector {
	if def.PodSelector == nil {
		return nil
	}
	return def.PodSelector.ReplicaSelector
}

// buildReplicaScoped groups the parent's matched pods by replica key and
// rebuilds the children subtree per replica, so descendant pod-attribution
// stays scoped within each replica. The result is one InstanceNode per
// replica with ReplicaKey set, its own children, and any pods that didn't
// fall into a child.
func buildReplicaScoped(ctx context.Context, factory *resource.ComponentFactory, karta *v1alpha1.Karta, def v1alpha1.ComponentDefinition, matched []*corev1.Pod, childDefs []v1alpha1.ComponentDefinition, instances map[string]resource.ExtractedInstance, repSel *v1alpha1.ReplicaSelector, matcher PodMatcher) ([]InstanceNode, error) {
	podsByReplica := make(map[string][]*corev1.Pod)
	for _, p := range matched {
		key, found, err := resource.NewPodQuerier(p).ExtractReplicaKey(ctx, repSel)
		if err != nil || !found {
			// Pods missing the replica-key label belong to no replica; skip.
			continue
		}
		podsByReplica[key] = append(podsByReplica[key], p)
	}

	keys := make([]string, 0, len(podsByReplica))
	for k := range podsByReplica {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Pick a single shared template instance (LWS group has one template
	// regardless of replica count). When the Karta definition exposes
	// per-replica instances by key, prefer those.
	var sharedInst *resource.ExtractedInstance
	if len(instances) == 1 {
		for _, v := range instances {
			vCopy := v
			sharedInst = &vCopy
			break
		}
	}

	out := make([]InstanceNode, 0, len(keys))
	for _, k := range keys {
		replicaPods := podsByReplica[k]
		children, err := buildChildren(ctx, factory, karta, childDefs, replicaPods, matcher)
		if err != nil {
			return nil, err
		}
		if len(children) > 0 && !hasComponentTypeSelector(def) {
			replicaPods = collectDescendantPods(children)
		}

		var inst *resource.ExtractedInstance
		var scale *resource.Scale
		if perKey, ok := instances[k]; ok {
			perKeyCopy := perKey
			inst = &perKeyCopy
			scale = perKey.Scale
		} else if sharedInst != nil {
			inst = sharedInst
			scale = sharedInst.Scale
		}

		keyCopy := k
		out = append(out, InstanceNode{
			ReplicaKey:        &keyCopy,
			Scale:             scale,
			ExtractedInstance: inst,
			Pods:              replicaPods,
			Children:          children,
		})
	}
	return out, nil
}

// componentInstanceSelector returns the selector responsible for splitting
// pods across instances of a single component, when the definition declares
// one.
func componentInstanceSelector(def v1alpha1.ComponentDefinition) *v1alpha1.ComponentInstanceSelector {
	if def.PodSelector == nil {
		return nil
	}
	return def.PodSelector.ComponentInstanceSelector
}

// buildMultiInstance produces one InstanceNode per extracted instance,
// routing pods by the result of the ComponentInstanceSelector. Pods whose
// instance ID doesn't match any extracted instance are dropped silently —
// they belong to a deleted or transitioning instance.
func buildMultiInstance(ctx context.Context, instances map[string]resource.ExtractedInstance, pods []*corev1.Pod, instSel *v1alpha1.ComponentInstanceSelector) ([]InstanceNode, error) {
	podsByID := make(map[string][]*corev1.Pod, len(instances))
	for _, p := range pods {
		id, found, err := resource.NewPodQuerier(p).ExtractInstanceId(ctx, instSel)
		if err != nil || !found {
			// Pods that don't carry the instance-id label belong to no
			// extracted instance: skip silently.
			continue
		}
		podsByID[id] = append(podsByID[id], p)
	}

	keys := make([]string, 0, len(instances))
	for k := range instances {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]InstanceNode, 0, len(keys))
	for _, k := range keys {
		inst := instances[k]
		instCopy := inst
		keyCopy := k
		out = append(out, InstanceNode{
			InstanceKey:       &keyCopy,
			Scale:             inst.Scale,
			ExtractedInstance: &instCopy,
			Pods:              podsByID[k],
		})
	}
	return out, nil
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

// hasComponentTypeSelector reports whether the definition has an explicit
// pod-level discriminator. Components without one are treated as logical
// groupings whose pod set is the union of their descendants'.
func hasComponentTypeSelector(def v1alpha1.ComponentDefinition) bool {
	return def.PodSelector != nil && def.PodSelector.ComponentTypeSelector != nil
}

// collectDescendantPods returns every pod claimed anywhere in the subtree,
// deduplicated by pointer identity so a pod claimed by multiple descendants
// is counted once.
func collectDescendantPods(nodes []ComponentNode) []*corev1.Pod {
	seen := make(map[*corev1.Pod]struct{})
	var out []*corev1.Pod
	var walk func([]ComponentNode)
	walk = func(ns []ComponentNode) {
		for _, n := range ns {
			for _, inst := range n.Instances {
				for _, p := range inst.Pods {
					if _, ok := seen[p]; ok {
						continue
					}
					seen[p] = struct{}{}
					out = append(out, p)
				}
				walk(inst.Children)
			}
		}
	}
	walk(nodes)
	return out
}

func workloadStatusFromResource(s *resource.Status) WorkloadStatus {
	out := WorkloadStatus{}
	for _, m := range s.MatchedStatuses {
		out.Phases = append(out.Phases, string(m))
	}
	return out
}
