// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cases holds the vocabulary for the Karta end-to-end flow tests: the state-registry primitives,
// the predicates and actions flows use, and the exported state constants.
package cases

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// StateCheck recognises a state from the workload's own fields, never from Karta.
type StateCheck func(*unstructured.Unstructured) bool

// NamedState pairs a state with the predicate that recognises it.
type NamedState struct {
	Name  kartav1alpha1.ResourceStatus
	Match StateCheck
}

// Step is one stop on a flow's journey: a state to reach, an optional action, and an optional
// ActionPredicate that gates the step. ActionPredicate lets a journey list the same state more than once
// (a scale flow is Running at 1, 3, then 1 replica): the step is not reached until the predicate over the
// workload's own fields holds, and its action fires then.
type Step struct {
	State           kartav1alpha1.ResourceStatus
	Action          *Action
	ActionPredicate StateCheck
	// Optional marks a transient step the workload may miss (a scale dip): the order check tolerates it
	// being absent. It is not a drive stop - only checkpoints (steps with an action or predicate) are.
	Optional bool
}

// Steps builds a plain journey (states only), used to declare an expected order.
func Steps(states ...kartav1alpha1.ResourceStatus) []Step {
	j := make([]Step, len(states))
	for i, s := range states {
		j[i] = Step{State: s}
	}
	return j
}

// Classify returns the furthest-along state the workload matches, judged from its own fields.
func Classify(u *unstructured.Unstructured, states []NamedState) kartav1alpha1.ResourceStatus {
	var name kartav1alpha1.ResourceStatus
	for _, s := range states {
		if s.Match(u) {
			name = s.Name
		}
	}
	return name
}
