// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package loader resolves a workload object and its candidate pods from the
// live cluster, given a name and namespace, so the tree builder can run.
//
// The loader is deliberately Karta-aware: it iterates the registry of known
// community definitions, lists each definition's target GVK in the namespace,
// and matches by name. This means the user can run `karta workload tree
// my-job` without having to remember whether it's a PyTorchJob or a
// RayCluster — the CLI figures it out.
package loader

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/run-ai/karta/cmd/karta/internal/definitions"
	"github.com/run-ai/karta/cmd/karta/internal/kube"
	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Resolved is the input set the tree builder needs.
type Resolved struct {
	Karta    *v1alpha1.Karta
	Workload *unstructured.Unstructured
	Pods     []corev1.Pod
}

// Discovered is one Karta-aware workload found while walking a namespace.
// It carries everything needed to assemble the workload's tree without a
// second cluster round-trip per workload.
type Discovered struct {
	Karta    *v1alpha1.Karta
	Workload *unstructured.Unstructured
}

// FindWorkload locates a workload by name across all GVKs the registry knows
// about. It errors if the name is ambiguous across multiple kinds in the
// namespace, or if no Karta-aware workload matches.
func FindWorkload(ctx context.Context, client *kube.Client, registry *definitions.Registry, namespace, name string) (*Resolved, error) {
	type hit struct {
		gvk      schema.GroupVersionKind
		obj      *unstructured.Unstructured
		karta    *v1alpha1.Karta
	}

	var hits []hit

	for _, k := range registry.All() {
		root := k.Spec.StructureDefinition.RootComponent
		if root.Kind == nil {
			continue
		}
		gvk := schema.GroupVersionKind{Group: root.Kind.Group, Version: root.Kind.Version, Kind: root.Kind.Kind}
		mapping, err := client.Mapper().RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			// Kind not installed in cluster; skip this definition silently.
			continue
		}
		obj, err := client.Dynamic().Resource(mapping.Resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("get %s/%s: %w", gvk.Kind, name, err)
		}
		hits = append(hits, hit{gvk: gvk, obj: obj, karta: k})
	}

	switch len(hits) {
	case 0:
		return nil, fmt.Errorf("no Karta-aware workload named %q in namespace %q (try kubectl get to list raw resources)", name, namespace)
	case 1:
		// fall through
	default:
		kinds := make([]string, 0, len(hits))
		for _, h := range hits {
			kinds = append(kinds, h.gvk.Kind)
		}
		return nil, fmt.Errorf("workload name %q is ambiguous across kinds: %v — disambiguate by deleting one or specifying kind explicitly (not yet supported)", name, kinds)
	}

	h := hits[0]
	pods, err := client.Core().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods in %q: %w", namespace, err)
	}

	return &Resolved{Karta: h.karta, Workload: h.obj, Pods: pods.Items}, nil
}

// ListWorkloads returns every Karta-aware workload in the namespace, plus
// the namespace's full pod list (so the caller can build trees per workload
// without re-listing pods). GVKs whose CRDs aren't installed in the cluster
// are silently skipped, mirroring FindWorkload.
func ListWorkloads(ctx context.Context, client *kube.Client, registry *definitions.Registry, namespace string) ([]Discovered, []corev1.Pod, error) {
	var found []Discovered

	for _, k := range registry.All() {
		root := k.Spec.StructureDefinition.RootComponent
		if root.Kind == nil {
			continue
		}
		gvk := schema.GroupVersionKind{Group: root.Kind.Group, Version: root.Kind.Version, Kind: root.Kind.Kind}
		mapping, err := client.Mapper().RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			continue
		}
		list, err := client.Dynamic().Resource(mapping.Resource).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, nil, fmt.Errorf("list %s in %q: %w", gvk.Kind, namespace, err)
		}
		for i := range list.Items {
			obj := list.Items[i]
			found = append(found, Discovered{Karta: k, Workload: &obj})
		}
	}

	pods, err := client.Core().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("list pods in %q: %w", namespace, err)
	}

	return found, pods.Items, nil
}
