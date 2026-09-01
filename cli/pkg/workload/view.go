// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package workload resolves a workload object into the view the CLI renders,
// reading it through its Karta definition without contacting the cluster.
package workload

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/cli/pkg/definitions"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

const gpuResourceName = corev1.ResourceName("nvidia.com/gpu")

// undefinedPhase covers both an absent status mapping and one that matched nothing.
const undefinedPhase = "Undefined"

// View is one workload as Karta resolves it: the root object's identity plus its
// semantic component breakdown.
type View struct {
	UID        string          `json:"-"`
	Name       string          `json:"name"`
	Namespace  string          `json:"namespace"`
	Kind       string          `json:"kind"`
	APIVersion string          `json:"apiVersion"`
	CreatedAt  time.Time       `json:"createdAt"`
	Definition string          `json:"definition"`
	Origin     string          `json:"origin"`
	Phases     []string        `json:"phases"`
	GPUs       int64           `json:"gpus"`
	Components []ComponentView `json:"components"`
	// PodStats is populated by the caller after Resolve, once live pods have
	// been attributed to this workload. Zero value means it was not computed.
	PodStats PodStats `json:"podStats,omitempty"`
}

// ComponentView is one pod-bearing component. Name is the instance key for a
// multi-instance component and the component name otherwise.
type ComponentView struct {
	Name     string `json:"name"`
	Kind     string `json:"kind,omitempty"`
	Replicas int32  `json:"replicas"`
	GPUs     int64  `json:"gpus"`
}

// Resolve reads obj through def and returns its view.
func Resolve(ctx context.Context, obj *unstructured.Unstructured, def definitions.Definition) (*View, error) {
	factory := resource.NewComponentFactoryFromObject(def.Karta, obj)

	workloadTree, err := tree.Build(ctx, factory)
	if err != nil {
		return nil, fmt.Errorf("build tree: %w", err)
	}

	view := &View{
		UID:        string(obj.GetUID()),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Kind:       obj.GetKind(),
		APIVersion: obj.GetAPIVersion(),
		CreatedAt:  obj.GetCreationTimestamp().Time,
		Definition: def.Karta.Name,
		Origin:     string(def.Origin),
		Phases:     phases(workloadTree),
	}

	// tree.Build drops the root, but Deployment, StatefulSet, Pod and Job carry
	// their pod template on it.
	root, err := factory.GetRootComponent()
	if err != nil {
		return nil, fmt.Errorf("get root component: %w", err)
	}
	if root.HasPodDefinition() {
		instances, err := root.GetExtractedInstances(ctx)
		if err != nil {
			return nil, fmt.Errorf("extract root instances: %w", err)
		}
		for _, id := range slices.Sorted(maps.Keys(instances)) {
			name := root.Name()
			if id != "" {
				name = id
			}
			view.Components = append(view.Components, componentViews(name, kindOf(root.Kind()), instances[id])...)
		}
	}

	view.Components = append(view.Components, walk(workloadTree.Children)...)
	if view.Components == nil {
		view.Components = []ComponentView{}
	}
	for _, component := range view.Components {
		view.GPUs += component.GPUs
	}
	return view, nil
}

// phases folds "no status mapping" and "mapping matched nothing" into one value.
func phases(t *tree.WorkloadTree) []string {
	if t.Status == nil || len(t.Status.Phases) == 0 {
		return []string{undefinedPhase}
	}
	return t.Status.Phases
}

// walk flattens the tree into one entry per pod-bearing component instance.
func walk(nodes []tree.ComponentNode) []ComponentView {
	var out []ComponentView
	for _, node := range nodes {
		for _, instance := range node.Instances {
			if !node.HasPodDefinition || instance.ExtractedInstance == nil {
				continue
			}
			name := node.Name
			if instance.InstanceKey != nil {
				name = *instance.InstanceKey
			}
			out = append(out, componentViews(name, kindOf(node.Kind), *instance.ExtractedInstance)...)
		}

		// Every instance carries an identical clone of the subtree, and a child's
		// replicas are already an absolute total, so descend exactly once.
		if len(node.Instances) > 0 {
			out = append(out, walk(node.Instances[0].Children)...)
		}
	}
	return out
}

// componentViews returns the entry for one instance, or nothing when the
// workload does not use it. A jq null becomes a zero struct, never nil.
func componentViews(name, kind string, instance resource.ExtractedInstance) []ComponentView {
	replicas, scaled := replicasOf(instance.Scale)
	gpus, specified := gpusOf(instance)
	if !scaled && !specified {
		return nil
	}
	return []ComponentView{{
		Name:     name,
		Kind:     kind,
		Replicas: replicas,
		GPUs:     gpus * int64(replicas),
	}}
}

// replicasOf reports the desired replica count and whether any scale was
// extracted. Components declaring only a minimum fall back to it.
func replicasOf(scale *resource.Scale) (replicas int32, scaled bool) {
	switch {
	case scale == nil:
		return 1, false
	case scale.Replicas != nil:
		return *scale.Replicas, true
	case scale.MinReplicas != nil:
		return *scale.MinReplicas, true
	case scale.MaxReplicas != nil:
		return 1, true
	default:
		return 1, false
	}
}

// gpusOf sums the GPUs one replica requests. Shapes are mutually exclusive, so
// a definition naming both a container and a resources path counts them once.
func gpusOf(instance resource.ExtractedInstance) (gpus int64, specified bool) {
	switch {
	case instance.PodTemplateSpec != nil && len(instance.PodTemplateSpec.Spec.Containers) > 0:
		return podGPUs(instance.PodTemplateSpec.Spec), true
	case instance.PodSpec != nil && len(instance.PodSpec.Containers) > 0:
		return podGPUs(*instance.PodSpec), true
	case instance.FragmentedPodSpec != nil:
		return sumFragmented(*instance.FragmentedPodSpec)
	default:
		return 0, false
	}
}

// podGPUs mirrors the effective pod request Kubernetes schedules against:
// max(largest init container, running containers plus sidecars).
func podGPUs(spec corev1.PodSpec) int64 {
	running := sumContainers(spec.Containers)

	var largestInit int64
	for _, container := range spec.InitContainers {
		gpus := gpusIn(container.Resources)
		// A sidecar never exits, so it accumulates rather than peaking.
		if container.RestartPolicy != nil && *container.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			running += gpus
			continue
		}
		largestInit = max(largestInit, gpus)
	}

	return max(largestInit, running)
}

func sumFragmented(spec resource.FragmentedPodSpec) (gpus int64, specified bool) {
	switch {
	case len(spec.Containers) > 0:
		return sumContainers(spec.Containers), true
	case spec.Container != nil:
		return sumContainers([]corev1.Container{*spec.Container}), true
	case spec.Resources != nil:
		return gpusIn(*spec.Resources), true
	default:
		return 0, false
	}
}

func sumContainers(containers []corev1.Container) int64 {
	var gpus int64
	for _, container := range containers {
		gpus += gpusIn(container.Resources)
	}
	return gpus
}

// gpusIn falls back to limits: extended resources are often declared there only.
func gpusIn(requirements corev1.ResourceRequirements) int64 {
	if quantity, ok := requirements.Requests[gpuResourceName]; ok {
		return quantity.Value()
	}
	if quantity, ok := requirements.Limits[gpuResourceName]; ok {
		return quantity.Value()
	}
	return 0
}

func kindOf(gvk *metav1.GroupVersionKind) string {
	if gvk == nil {
		return ""
	}
	return gvk.Kind
}
