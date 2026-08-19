// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package attribute

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/instructions"
	"github.com/run-ai/karta/pkg/resource"

	"github.com/run-ai/karta/exporter/pkg/collector"
	"github.com/run-ai/karta/exporter/pkg/registry"
)

// Result is the pod-to-component attribution outcome. Reason is empty on a
// clean attribution; a failed step degrades its field to the sentinel and
// records why, so workload-level aggregation keeps working (the pod is never
// dropped).
type Result struct {
	Component string
	Instance  string
	Replica   string
	Reason    string
}

// Attribute resolves which component, component instance, and replica of a
// workload the given pod belongs to, using the entry's Karta definition.
// The workload object is the live root object the pod's owner chain reached.
func Attribute(ctx context.Context, pod *corev1.Pod, entry *registry.Entry, workload resource.KubernetesObject) Result {
	factory := resource.NewComponentFactoryFromObject(entry.Karta, workload)
	querier := resource.NewPodQuerier(pod)

	componentName, err := instructions.InferPodComponent(ctx, querier, entry.Summary)
	if err != nil {
		return Result{
			Component: collector.SentinelUnknown,
			Instance:  collector.SentinelUnknown,
			Reason:    collector.ReasonJQError,
		}
	}

	result := Result{Component: componentName}

	instance, err := instructions.InferPodComponentInstance(ctx, querier, componentName, factory)
	switch {
	case err == nil:
		if instance != nil {
			result.Instance = *instance
		}
	case errors.As(err, new(resource.InstanceNotFoundError)):
		result.Instance = collector.SentinelUnknown
		result.Reason = collector.ReasonUnknownInstance
	default:
		result.Instance = collector.SentinelUnknown
		result.Reason = collector.ReasonJQError
	}

	replica, err := extractReplica(ctx, querier, factory, componentName)
	if err != nil && result.Reason == "" {
		result.Reason = collector.ReasonJQError
	}
	result.Replica = replica

	return result
}

// extractReplica finds the nearest ReplicaSelector on the component or its
// ancestors (descendants inherit the replica context from the ancestor that
// defines it) and evaluates it against the pod.
func extractReplica(ctx context.Context, querier *resource.PodQuerier, factory *resource.ComponentFactory, componentName string) (string, error) {
	current := componentName
	for current != "" {
		component, err := factory.GetComponent(current)
		if err != nil {
			return "", err
		}

		selector := component.GetPodSelector()
		if selector != nil && selector.ReplicaSelector != nil {
			replica, found, err := querier.ExtractReplicaKey(ctx, selector.ReplicaSelector)
			if err != nil {
				return "", err
			}
			if found {
				return replica, nil
			}
			return "", nil
		}

		definition := component.Definition()
		if definition.OwnerRef == nil {
			return "", nil
		}
		current = *definition.OwnerRef
	}
	return "", nil
}
