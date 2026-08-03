// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"fmt"
	"slices"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

type JourneyStep struct {
	State    v1alpha1.ResourceStatus
	Optional bool
}

// ObservedOrderErr checks the observed states are a legal walk of the journey: required steps appear in
// order ending at want; Optional or recurring states may be absent; anything else fails.
func ObservedOrderErr(declared []JourneyStep, observed []v1alpha1.ResourceStatus, want v1alpha1.ResourceStatus) error {
	obs := slices.Compact(slices.Clone(observed)) // collapse consecutive repeats
	if len(obs) == 0 {
		return fmt.Errorf("no states observed")
	}

	// A step is skippable if Optional, or if its state recurs earlier: compaction collapses a repeated
	// Running (scale) or an Initializing dip into one, so the later occurrences can never be observed.
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
