// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package tree builds and represents a workload as a hierarchical tree of
// components, instances, and pods. The tree is the shared data model the
// CLI, web, and MCP layers all consume so each consumer can render its own
// view without re-deriving structure from raw Karta extraction calls.
package tree

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// WorkloadTree is the raw workload tree produced by Build. It carries the
// structure, status, and matched pods extracted from a workload object via
// its Karta definition. Display-time concerns (ready counts, GPU sums,
// rendered tables) live above this layer in the consumer.
type WorkloadTree struct {
	// Status is the normalized workload-level status, taken from the root
	// component's Karta StatusDefinition.
	Status WorkloadStatus

	// Children are the root-level components of the workload.
	Children []ComponentNode
}

// WorkloadStatus captures a workload's normalized phase set. A workload may
// match multiple phases simultaneously (for example "Running" + "Degraded"),
// which is why this is a slice rather than a single string.
type WorkloadStatus struct {
	Phases []string
}

// ComponentNode is one component in the workload tree. A component may have
// one or more instances; for non-multi-instance components there is exactly
// one InstanceNode whose InstanceKey is nil.
type ComponentNode struct {
	// Name is the component's logical name as declared in the Karta definition.
	Name string

	// Kind is the GroupVersionKind of the underlying Kubernetes object backing
	// this component, when one exists. Logical-grouping components (no backing
	// object) leave this nil.
	Kind *v1alpha1.GroupVersionKind

	// Instances always has at least one entry.
	Instances []InstanceNode
}

// InstanceNode is one instance of a component. Multi-instance components
// (for example Dynamo's service component split into Frontend / PrefillWorker
// / DecodeWorker) carry one InstanceNode per instance, each with its own
// InstanceKey, scale, extracted spec, pods, and child components.
type InstanceNode struct {
	// InstanceKey identifies this instance among siblings under the same
	// component. Nil means the component is not multi-instance.
	InstanceKey *string

	// ReplicaKey identifies this instance among ordered replicas (when the
	// component supports replication beyond a simple replica count). Nil
	// otherwise.
	ReplicaKey *string

	// Scale carries the replicas / min / max extracted by the Karta definition.
	Scale *resource.Scale

	// ExtractedInstance carries the pod spec, metadata, and scale that the
	// Karta library extracted for this instance.
	ExtractedInstance *resource.ExtractedInstance

	// Pods are the live pods claimed for this instance by the PodMatcher.
	Pods []*corev1.Pod

	// Children are component nodes nested under this instance, when the
	// Karta definition declares a deeper hierarchy. Empty for leaf components.
	Children []ComponentNode
}
