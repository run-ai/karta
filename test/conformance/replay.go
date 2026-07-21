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

// Replay reads one CR snapshot through the Karta library and projects the result into
// Expected. It is the single read path shared by the live recorder and the offline golden
// test, so the two can never diverge. It keeps everything Karta reads (status plus the full
// per-instance extraction) with per-run volatile fields stripped, so the frozen reading
// changes only when Karta's extraction changes.
func Replay(karta *v1alpha1.Karta, obj resource.KubernetesObject) (Expected, error) {
	ctx := context.Background()
	factory := resource.NewComponentFactoryFromObject(karta, obj)

	root, err := factory.GetRootComponent()
	if err != nil {
		return Expected{}, fmt.Errorf("root component: %w", err)
	}
	status, err := root.GetStatus(ctx)
	if err != nil {
		return Expected{}, fmt.Errorf("status: %w", err)
	}

	exp := Expected{}
	if status != nil {
		exp.MatchedStatuses = status.MatchedStatuses
		exp.Phase = status.Phase
		if len(status.Conditions) > 0 {
			conds, err := strippedCopy[[]interface{}](status.Conditions)
			if err != nil {
				return Expected{}, fmt.Errorf("conditions: %w", err)
			}
			exp.Conditions = conds
		}
	}

	children, err := factory.GetChildComponents()
	if err != nil {
		return Expected{}, fmt.Errorf("child components: %w", err)
	}
	components := append([]*resource.Component{root}, children...)

	comps := map[string]ComponentReading{}
	for _, c := range components {
		instances, err := c.GetExtractedInstances(ctx)
		if err != nil {
			// A component that cannot extract from this CR is skipped the same way by
			// record and replay; a regression shows up as its keys leaving the diff.
			continue
		}
		if len(instances) == 0 {
			continue
		}
		insts := map[string]interface{}{}
		for k, inst := range instances {
			m, err := strippedCopy[map[string]interface{}](inst)
			if err != nil {
				return Expected{}, fmt.Errorf("extract %s: %w", c.Name(), err)
			}
			insts[k] = m
		}
		comps[c.Name()] = ComponentReading{Instances: insts}
	}
	if len(comps) > 0 {
		exp.Components = comps
	}
	return exp, nil
}

// strippedCopy marshals a Karta-extracted value to generic JSON, removes the per-run
// volatile fields (see sanitize.go), and drops empty leaves so the frozen reading holds
// only what Karta read, not noise like "reason: null" or "securityContext: {}". T is one of
// the two shapes stripVolatile understands: a map for an instance, a slice for conditions.
func strippedCopy[T map[string]interface{} | []interface{}](v interface{}) (T, error) {
	var out T
	b, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	stripVolatile(out)
	pruneEmpty(out)
	return out, nil
}

// pruneEmpty recursively drops map keys whose value is nil, an empty map, or an empty
// slice. It runs only on the reading, never on the CR, whose empty/null fields the library
// may read (a null status.lastScheduleTime is how a CronJob reads as initializing).
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
