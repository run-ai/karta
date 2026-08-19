// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"fmt"
	"slices"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// validateObservedOrder checks that observed is a legal walk of the journey:
//
//  1. Consecutive duplicate observations collapse to one visit; an empty
//     observation list always fails.
//  2. The last observed state must equal want, the declared terminal.
//  3. The collapsed walk must be the journey with zero or more absent-allowed
//     steps removed, in order, nothing extra. A step is allowed to be absent
//     when it is Optional, or when its state already appears earlier in the
//     journey: the recorder collapses duplicates, so a declared revisit can
//     merge into the earlier visit and can never be demanded.
//
// Example: journey [Init, Running, Init, Completed] accepts observed
// [Init, Running, Completed], the declared dip back to Init may be missed;
// journey [Init, Running, Completed] rejects it, the dip was never declared.
func validateObservedOrder(journey []journeyStep, observed []kartav1alpha1.ResourceStatus, wantTerminal kartav1alpha1.ResourceStatus) error {
	journeyStates := make([]kartav1alpha1.ResourceStatus, len(journey))
	for i, step := range journey {
		journeyStates[i] = step.State
	}

	// Collapse consecutive duplicate observations: dwelling in a state is one visit.
	var visits []kartav1alpha1.ResourceStatus
	for _, state := range observed {
		if len(visits) == 0 || visits[len(visits)-1] != state {
			visits = append(visits, state)
		}
	}
	if len(visits) == 0 {
		return fmt.Errorf("no states observed, want journey %v", journeyStates)
	}
	if lastVisit := visits[len(visits)-1]; lastVisit != wantTerminal {
		return fmt.Errorf("last observed state is %q, want terminal %q (journey %v, observed %v)", lastVisit, wantTerminal, journeyStates, visits)
	}

	// Match the journey against the visits, in order: every journey step either matches the next
	// unmatched visit, or must be allowed to be absent from the walk.
	nextUnmatchedVisit := 0
	for stepIndex, step := range journey {
		stepMatchesNextVisit := nextUnmatchedVisit < len(visits) && visits[nextUnmatchedVisit] == step.State
		stateDeclaredEarlier := slices.Contains(journeyStates[:stepIndex], step.State)

		switch {
		case stepMatchesNextVisit:
			nextUnmatchedVisit++
		case step.Optional || stateDeclaredEarlier:
			// Allowed to be absent: a declared revisit merges into the earlier visit.
		default:
			return fmt.Errorf("required state %q missing or out of order (journey %v, observed %v)", step.State, journeyStates, visits)
		}
	}
	if nextUnmatchedVisit < len(visits) {
		return fmt.Errorf("observed state %q is not part of the journey here (journey %v, observed %v)", visits[nextUnmatchedVisit], journeyStates, visits)
	}
	return nil
}
