// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cases holds the Karta end-to-end workload case definitions: each workload's state
// registry, the flows that drive it, and the field predicates and actions those flows use. The
// e2e harness (package e2e) imports this package, iterates cases.All, and records each flow.
package cases

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// StateCheck recognises a state from the workload's own fields, never from Karta.
type StateCheck func(*unstructured.Unstructured) bool

// StateAction drives a transition the operator will not make itself, e.g. Unsuspend.
type StateAction func(ctx context.Context, obj *unstructured.Unstructured) error

type NamedState struct {
	Name  kartav1alpha1.ResourceStatus
	Ready StateCheck
}

// Step is one stop on a journey: a state to reach and an optional action fired when it does.
// Settle is an optional extra gate on the workload's own fields: the step is only reached once
// Classify returns State AND Settle holds. It lets a journey list the same state twice and capture
// each - a scale flow is Running(1 replica) -> Running(3) -> Running(1), one step per replica count,
// distinguished by Settle even though Karta reads Running throughout.
type Step struct {
	State  kartav1alpha1.ResourceStatus
	Action StateAction
	Settle StateCheck
}

// Flow drives a workload along an ordered journey of states. The observed states must be an in-order
// subsequence of the journey; a workload that revisits a state declares that revisit as its own step.
type Flow struct {
	Name         string
	WorkloadFile string
	Journey      []Step
}

func (f Flow) Want() kartav1alpha1.ResourceStatus { return f.Journey[len(f.Journey)-1].State }

type WorkloadCase struct {
	Name      string
	Operator  string
	KartaFile string
	KartaName string
	States    []NamedState // state registry, ordered least to most advanced
	Flows     []Flow
	Timeout   time.Duration
}

// Steps builds action-less journey stops: Journey: Steps(InitializingStatus, RunningStatus).
func Steps(states ...kartav1alpha1.ResourceStatus) []Step {
	j := make([]Step, len(states))
	for i, s := range states {
		j[i] = Step{State: s}
	}
	return j
}

func (tc WorkloadCase) Validate() error {
	known := map[kartav1alpha1.ResourceStatus]bool{}
	for _, s := range tc.States {
		known[s.Name] = true
	}
	for _, fl := range tc.Flows {
		if len(fl.Journey) == 0 {
			return fmt.Errorf("case %q flow %q: empty journey", tc.Name, fl.Name)
		}
		for _, st := range fl.Journey {
			if !known[st.State] {
				return fmt.Errorf("case %q flow %q: state %q not in registry", tc.Name, fl.Name, st.State)
			}
		}
	}
	return nil
}

// Classify returns the furthest-along state the workload matches, judged from its own fields.
func Classify(u *unstructured.Unstructured, states []NamedState) kartav1alpha1.ResourceStatus {
	var name kartav1alpha1.ResourceStatus
	for _, s := range states {
		if s.Ready(u) {
			name = s.Name
		}
	}
	return name
}
