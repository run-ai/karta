// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package v1alpha1

// ConditionType is a typed constant for Karta status condition types.
//
// These conditions are managed by the OSS Karta operator and form the public
// API surface that consumers (e.g., schedulers, controllers, platforms) rely
// on to decide whether a Karta is usable.
type ConditionType string

const (
	// ConditionKartaValidated indicates that the Karta spec was validated by
	// the operator. When True, the spec is structurally valid (component
	// hierarchy, ownership graph, instructions) and all JQ expressions parse
	// successfully.
	ConditionKartaValidated ConditionType = "KartaValidated"

	// ConditionCRDExists indicates that the CustomResourceDefinition for the
	// root component GroupVersionKind exists in the cluster and serves the
	// referenced version.
	ConditionCRDExists ConditionType = "CRDExists"

	// ConditionReady is the aggregate condition signaling that a Karta is
	// ready to be used by consumers. It is True iff ConditionKartaValidated
	// and ConditionCRDExists are both True.
	ConditionReady ConditionType = "Ready"
)
