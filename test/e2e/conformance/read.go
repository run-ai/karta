// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// Reading runs the Karta library on one CR and returns everything it reads (matched statuses, phase,
// conditions, per-component extraction) as a generic map with empty leaves pruned. No denylist: the
// golden rebuilds the exact CR, so Karta reads the same bytes back and the diff is stable.
func Reading(ctx context.Context, karta *v1alpha1.Karta, obj resource.KubernetesObject) (map[string]interface{}, error) {
	factory := resource.NewComponentFactoryFromObject(karta, obj)

	root, err := factory.GetRootComponent()
	if err != nil {
		return nil, fmt.Errorf("root component: %w", err)
	}
	status, err := root.GetStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}

	reading := map[string]interface{}{}
	if status != nil {
		reading["matchedStatuses"] = status.MatchedStatuses
		if status.Phase != nil {
			reading["phase"] = *status.Phase
		}
		if len(status.Conditions) > 0 {
			reading["conditions"] = status.Conditions
		}
	}

	children, err := factory.GetChildComponents()
	if err != nil {
		return nil, fmt.Errorf("child components: %w", err)
	}
	components := map[string]interface{}{}
	for _, c := range append([]*resource.Component{root}, children...) {
		instances, err := c.GetExtractedInstances(ctx)
		// A component that cannot extract is skipped; a regression shows up as its keys leaving the diff.
		if err != nil || len(instances) == 0 {
			continue
		}
		components[c.Name()] = map[string]interface{}{"instances": instances}
	}
	if len(components) > 0 {
		reading["components"] = components
	}

	// Round-trip through JSON to plain scalars, maps, and slices, then drop empty leaves so the reading
	// holds what Karta read, not "reason: null" noise.
	generic, err := toGeneric(reading)
	if err != nil {
		return nil, err
	}
	pruneEmpty(generic)
	return generic, nil
}

func matchedStatuses(reading map[string]interface{}) []string {
	raw, _ := reading["matchedStatuses"].([]interface{})
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func toGeneric(v interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// pruneEmpty drops nil/empty map keys. It runs only on the reading, never on the CR, whose null fields
// the library may read (a null status.lastScheduleTime is how a CronJob reads as initializing).
func pruneEmpty(v interface{}) {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, val := range x {
			pruneEmpty(val)
			switch inner := val.(type) {
			case nil:
				delete(x, k)
			case map[string]interface{}:
				if len(inner) == 0 {
					delete(x, k)
				}
			case []interface{}:
				if len(inner) == 0 {
					delete(x, k)
				}
			}
		}
	case []interface{}:
		for _, val := range x {
			pruneEmpty(val)
		}
	}
}
