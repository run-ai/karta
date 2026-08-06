// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"fmt"
	"slices"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// observedOrderErr checks the observed states are a legal walk of the journey: required steps appear in
// order ending at want; Optional or recurring states may be absent; anything else fails.
func observedOrderErr(declared []journeyStep, observed []kartav1alpha1.ResourceStatus, want kartav1alpha1.ResourceStatus) error {
	obs := slices.Compact(slices.Clone(observed)) // collapse consecutive repeats
	if len(obs) == 0 {
		return fmt.Errorf("no states observed")
	}
	skippable := skippableSteps(declared)

	// walk the observed states through the declared journey, skipping steps that may be absent
	declaredIdx := 0
	for _, state := range obs {
		for declaredIdx < len(declared) && declared[declaredIdx].State != state {
			if !skippable[declaredIdx] {
				return fmt.Errorf("required state %q not observed before %q; journey %v, observed %v",
					declared[declaredIdx].State, state, journeyStates(declared), obs)
			}
			declaredIdx++
		}
		if declaredIdx == len(declared) {
			if slices.ContainsFunc(declared, func(s journeyStep) bool { return s.State == state }) {
				return fmt.Errorf("state %q observed out of journey %v; observed %v", state, journeyStates(declared), obs)
			}
			return fmt.Errorf("observed undeclared state %q; journey %v, observed %v", state, journeyStates(declared), obs)
		}
		declaredIdx++
	}

	// any required steps left after the walk were never observed
	for ; declaredIdx < len(declared); declaredIdx++ {
		if !skippable[declaredIdx] {
			return fmt.Errorf("required state %q not observed; journey %v, observed %v",
				declared[declaredIdx].State, journeyStates(declared), obs)
		}
	}

	if last := obs[len(obs)-1]; last != want {
		return fmt.Errorf("terminal must be %q, observed %q; sequence %v", want, last, obs)
	}
	return nil
}

// skippableSteps marks each declared step that may be absent from the observed run: an Optional step, or one
// whose state recurs earlier (compaction collapses a repeated Running from a scale, or an Initializing dip,
// into one, so the later occurrences can never be observed on their own).
func skippableSteps(declared []journeyStep) []bool {
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
	return skippable
}

func journeyStates(steps []journeyStep) []kartav1alpha1.ResourceStatus {
	out := make([]kartav1alpha1.ResourceStatus, len(steps))
	for i, s := range steps {
		out[i] = s.State
	}
	return out
}
