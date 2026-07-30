// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cases holds the Karta end-to-end workload definitions: each workload's state registry, the
// flows that drive it, and the predicates and actions those flows use.
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

type StateAction func(ctx context.Context, obj *unstructured.Unstructured) error

type NamedState struct {
	Name  kartav1alpha1.ResourceStatus
	Match StateCheck
}

// Step is one stop on a journey: a state to reach, an optional action, and an optional Settle gate that
// lets a journey list the same state more than once (a scale flow is Running at 1, 3, then 1 replica).
type Step struct {
	State  kartav1alpha1.ResourceStatus
	Action StateAction
	Settle StateCheck
	// Optional marks a transient step the workload may miss (a scale dip): the order check tolerates it
	// being absent, and driveByPosition does not stop at it.
	Optional bool
}

// Flow drives a workload along an ordered journey; the observed states must be an in-order subsequence.
type Flow struct {
	Name         string
	WorkloadFile string
	Journey      []Step
}

func (f Flow) DesiredFinalStatus() kartav1alpha1.ResourceStatus {
	return f.Journey[len(f.Journey)-1].State
}

type WorkloadCase struct {
	Name      string
	Operator  string
	KartaFile string
	KartaName string
	States    []NamedState // state registry, ordered least to most advanced
	Flows     []Flow
	Timeout   time.Duration
}

func Steps(states ...kartav1alpha1.ResourceStatus) []Step {
	j := make([]Step, len(states))
	for i, s := range states {
		j[i] = Step{State: s}
	}
	return j
}

func (wc WorkloadCase) Validate() error {
	if wc.Operator == "" || wc.KartaFile == "" || wc.KartaName == "" {
		return fmt.Errorf("case %q: Operator, KartaFile or KartaName are required", wc.Name)
	}
	known := map[kartav1alpha1.ResourceStatus]bool{}
	for _, s := range wc.States {
		known[s.Name] = true
	}
	for _, fl := range wc.Flows {
		if len(fl.Journey) == 0 {
			return fmt.Errorf("case %q flow %q: empty journey", wc.Name, fl.Name)
		}
		if fl.WorkloadFile == "" {
			return fmt.Errorf("case %q flow %q: empty WorkloadFile", wc.Name, fl.Name)
		}
		// A terminal Optional step would make DesiredFinalStatus() a dip that driveByPosition
		// skips, so the run could finish without the last real settle firing.
		if last := fl.Journey[len(fl.Journey)-1]; last.Optional {
			return fmt.Errorf("case %q flow %q: journey ends on an Optional step %q; the last step must be a real settle", wc.Name, fl.Name, last.State)
		}
		for _, st := range fl.Journey {
			if !known[st.State] {
				return fmt.Errorf("case %q flow %q: state %q not in registry", wc.Name, fl.Name, st.State)
			}
		}
	}
	return nil
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
