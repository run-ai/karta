// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"context"
	"sort"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// Replay runs the Karta library over a single CR snapshot and projects the result
// into Expected. It is the ONE read path shared by the live recorder (test/e2e) and
// the offline golden test (golden_test.go), so what the two check can never diverge:
// the recorder stores Replay's output as the expected, and the offline test asserts
// the current library still produces it.
func Replay(karta *v1alpha1.Karta, obj resource.KubernetesObject) (Expected, error) {
	ctx := context.Background()
	factory := resource.NewComponentFactoryFromObject(karta, obj)

	root, err := factory.GetRootComponent()
	if err != nil {
		return Expected{}, err
	}
	status, err := root.GetStatus(ctx)
	if err != nil {
		return Expected{}, err
	}

	exp := Expected{}
	if status != nil {
		exp.MatchedStatuses = status.MatchedStatuses
		exp.Phase = status.Phase
	}

	children, err := factory.GetChildComponents()
	if err != nil {
		return Expected{}, err
	}
	components := append([]*resource.Component{root}, children...)

	comps := map[string]ComponentReading{}
	for _, c := range components {
		instances, err := c.GetExtractedInstances(ctx)
		if err != nil {
			// This component cannot extract from this CR. Extraction is
			// deterministic, so record and replay skip it identically; a
			// regression that makes a previously extractable component fail shows
			// up as its keys disappearing from the diff.
			continue
		}
		if len(instances) == 0 {
			continue
		}
		keys := make([]string, 0, len(instances))
		scale := map[string]Scale{}
		for k, inst := range instances {
			keys = append(keys, k)
			if inst.Scale != nil {
				scale[k] = Scale{
					Replicas:    inst.Scale.Replicas,
					MinReplicas: inst.Scale.MinReplicas,
					MaxReplicas: inst.Scale.MaxReplicas,
				}
			}
		}
		sort.Strings(keys)
		reading := ComponentReading{Instances: keys}
		if len(scale) > 0 {
			reading.Scale = scale
		}
		comps[c.Name()] = reading
	}
	if len(comps) > 0 {
		exp.Components = comps
	}
	return exp, nil
}
