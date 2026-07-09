// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/karta/pkg/resource"
)

// WorkloadTree is the raw tree produced by Build().
// It contains Karta-extracted data for the workload hierarchy. The root
// component itself is not included; its status is in Status and its metadata
// lives on the workload object.
type WorkloadTree struct {
	// Status is nil when no status was evaluated, e.g. an offline pre-submission
	// tree built from a manifest that has not reached the cluster.
	Status   *WorkloadStatus `json:"status,omitempty"`
	Children []ComponentNode `json:"children,omitempty"`
}

// WorkloadStatus holds the normalized statuses extracted from the root component.
type WorkloadStatus struct {
	// Phases holds all ResourceStatus values that matched the workload's StatusMappings.
	// Status mappings are not mutually exclusive, so there can be multiple matches
	// (e.g. ["Running", "Degraded"]). Empty means no match (Undefined).
	Phases []string `json:"phases,omitempty"`
}

// ComponentNode represents one component in the hierarchy.
//
// Every component always carries at least one InstanceNode, even a
// single-instance component (one without an InstanceIdPath), which has exactly
// one InstanceNode with a nil InstanceKey. The shape is deliberately uniform -
// component -> instance -> component - so consumers can walk the tree without
// special-casing single- versus multi-instance components.
type ComponentNode struct {
	Name string `json:"name"`
	// Kind is nil for logical grouping components (those without a Kind in the definition).
	Kind             *metav1.GroupVersionKind `json:"kind,omitempty"`
	HasPodDefinition bool                     `json:"hasPodDefinition"`
	Instances        []InstanceNode           `json:"instances,omitempty"`
}

// InstanceNode represents one instance of a component.
// InstanceKey is nil for single-instance components (no InstanceIdPath defined).
// ReplicaKey is nil when no ReplicaSelector is defined on the component.
// Both axes are orthogonal and can co-exist.
type InstanceNode struct {
	InstanceKey       *string                     `json:"instanceKey,omitempty"`
	ReplicaKey        *string                     `json:"replicaKey,omitempty"`
	Scale             *resource.Scale             `json:"scale,omitempty"`
	ExtractedInstance *resource.ExtractedInstance `json:"extractedInstance,omitempty"`
	Children          []ComponentNode             `json:"children,omitempty"`
}
