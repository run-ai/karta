// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cases holds the Karta end-to-end workload definitions: each workload's state registry, the
// flows that drive it, and the predicates and actions those flows use.
package cases

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// StateCheck recognises a state from the workload's own fields, never from Karta.
type StateCheck func(*unstructured.Unstructured) bool

type NamedState struct {
	Name  kartav1alpha1.ResourceStatus
	Match StateCheck
}

// Step is one stop on a journey: a state to reach, an optional action, and an optional ActionPredicate
// that gates the step. ActionPredicate lets a journey list the same state more than once (a scale flow is
// Running at 1, 3, then 1 replica): the step is not reached until the predicate over the workload's own
// fields holds, and its action fires then.
type Step struct {
	State           kartav1alpha1.ResourceStatus
	Action          *Action
	ActionPredicate StateCheck
	// Optional marks a transient step the workload may miss (a scale dip): the order check tolerates it
	// being absent. It is not a drive stop - only checkpoints (steps with an action or predicate) are.
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
		// A terminal Optional step would make DesiredFinalStatus() a dip the driver skips, so the run
		// could finish without reaching the last real step.
		last := fl.Journey[len(fl.Journey)-1]
		if last.Optional {
			return fmt.Errorf("case %q flow %q: journey ends on an Optional step %q; the last step must be a real settle", wc.Name, fl.Name, last.State)
		}
		lastIsCheckpoint := last.Action != nil || last.ActionPredicate != nil
		for _, st := range fl.Journey {
			if !known[st.State] {
				return fmt.Errorf("case %q flow %q: state %q not in registry", wc.Name, fl.Name, st.State)
			}
			// A checkpoint (action or predicate) on an Optional step would fire even though the step is a
			// skippable dip.
			if st.Optional && (st.Action != nil || st.ActionPredicate != nil) {
				return fmt.Errorf("case %q flow %q: Optional step %q must not carry an action or predicate", wc.Name, fl.Name, st.State)
			}
			// If a checkpoint shares the final state, the last step must be that checkpoint; otherwise the
			// run stops when the checkpoint pops and drops the final settle.
			if !lastIsCheckpoint && st.State == last.State && (st.Action != nil || st.ActionPredicate != nil) {
				return fmt.Errorf("case %q flow %q: a checkpoint has the final state %q but the last step is plain; make the last step a checkpoint", wc.Name, fl.Name, last.State)
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
