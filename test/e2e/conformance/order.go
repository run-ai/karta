// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"fmt"
	"slices"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// ObservedOrderErr checks the observed states are an in-order subsequence of the declared journey,
// ending on want. A skipped declared step is fine - a fast cluster may miss a transient dip - but an
// out-of-order or undeclared state is a regression. A genuine revisit (a Job dips Running to
// Initializing as its pod terminates) is declared as its own step. The live recorder runs this on the
// states it observes, and the offline golden runs it on the recorded steps, so a corrupted or
// journey-drifted fixture fails offline too.
func ObservedOrderErr(declared, observed []v1alpha1.ResourceStatus, want v1alpha1.ResourceStatus) error {
	obs := slices.Compact(slices.Clone(observed)) // collapse consecutive repeats
	if len(obs) == 0 {
		return fmt.Errorf("no states observed")
	}

	// Walk declared forward, matching each observed state to the next declared one (subsequence check).
	di := 0
	for _, state := range obs {
		for di < len(declared) && declared[di] != state {
			di++
		}
		if di == len(declared) {
			if slices.Contains(declared, state) {
				return fmt.Errorf("state %q observed out of journey %v; observed %v", state, declared, obs)
			}
			return fmt.Errorf("observed undeclared state %q; journey is %v; observed %v", state, declared, obs)
		}
		di++
	}
	if last := obs[len(obs)-1]; last != want {
		return fmt.Errorf("terminal must be %q, observed %q; sequence %v", want, last, obs)
	}
	return nil
}
