// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

// Label keys stamped by the OSS Karta operator on every Karta whose root
// component has a GVK defined. They index the root component's
// GroupVersionKind so that consumers can locate a Karta by GVK using a
// label-selector List instead of fetching all Kartas.
//
// The label names match those used by the RunAI EWI controller so that both
// the OSS operator and EWI-produced Kartas share a consistent indexing scheme.
//
// Values are derived directly from
// spec.structureDefinition.rootComponent.kind and are kept in sync by the
// operator on every reconcile. Consumers must not write these labels; the
// operator is the sole owner.
const (
	// LabelRootGroup is the API group of the root component
	// (e.g. "ray.io", "kubeflow.org").
	LabelRootGroup = "run.ai/karta-group"

	// LabelRootVersion is the API version of the root component
	// (e.g. "v1", "v1alpha1").
	LabelRootVersion = "run.ai/karta-version"

	// LabelRootKind is the kind of the root component
	// (e.g. "RayCluster", "PyTorchJob").
	LabelRootKind = "run.ai/karta-kind"
)
