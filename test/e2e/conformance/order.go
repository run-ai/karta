// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"fmt"
	"slices"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// JourneyStep is one declared step for the order check: a state, and whether the workload may skip it.
// A required step (Optional false) must appear in the observed order; an Optional step - a transient dip
// a fast cluster may miss - may be absent, but if present must be in its declared position.
type JourneyStep struct {
	State    v1alpha1.ResourceStatus
	Optional bool
}

// ObservedOrderErr checks the observed states against the declared journey. Every required step must be
// observed, in order. An Optional step may be skipped, but if seen must be in place. A state not in the
// journey, or one out of order, is a regression. The live recorder runs this on the states it observes,
// and the offline golden runs it on the recorded steps, so a fixture that skipped a required transition
// or drifted from its case fails offline too.
func ObservedOrderErr(declared []JourneyStep, observed []v1alpha1.ResourceStatus, want v1alpha1.ResourceStatus) error {
	obs := slices.Compact(slices.Clone(observed)) // collapse consecutive repeats
	if len(obs) == 0 {
		return fmt.Errorf("no states observed")
	}

	// A step may be absent from the observed order if it is Optional, or if the same state appears
	// earlier in the journey - a recurring state (a scale flow's repeated Running, or a completed
	// flow's Initializing dip) collapses into one in the compacted order, so the golden can never see
	// the later occurrences. Every other step is required and must appear, in order.
	skippable := make([]bool, len(declared))
	for i, s := range declared {
		skippable[i] = s.Optional
		for j := 0; j < i; j++ {
			if declared[j].State == s.State {
				skippable[i] = true
				break
			}
		}
	}

	di := 0
	for _, state := range obs {
		// Advance to the next declared step that matches. A skippable step may be passed over; a required
		// one that does not match means it was skipped or this state does not belong here.
		for di < len(declared) && declared[di].State != state {
			if !skippable[di] {
				return fmt.Errorf("required state %q not observed before %q; journey %v, observed %v",
					declared[di].State, state, journeyStates(declared), obs)
			}
			di++
		}
		if di == len(declared) {
			if slices.ContainsFunc(declared, func(s JourneyStep) bool { return s.State == state }) {
				return fmt.Errorf("state %q observed out of journey %v; observed %v", state, journeyStates(declared), obs)
			}
			return fmt.Errorf("observed undeclared state %q; journey %v, observed %v", state, journeyStates(declared), obs)
		}
		di++
	}

	// Any required step after the last observed one never happened.
	for ; di < len(declared); di++ {
		if !skippable[di] {
			return fmt.Errorf("required state %q not observed; journey %v, observed %v",
				declared[di].State, journeyStates(declared), obs)
		}
	}

	if last := obs[len(obs)-1]; last != want {
		return fmt.Errorf("terminal must be %q, observed %q; sequence %v", want, last, obs)
	}
	return nil
}

func journeyStates(steps []JourneyStep) []v1alpha1.ResourceStatus {
	out := make([]v1alpha1.ResourceStatus, len(steps))
	for i, s := range steps {
		out[i] = s.State
	}
	return out
}
