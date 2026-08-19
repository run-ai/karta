// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package state

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"

	"github.com/run-ai/karta/exporter/pkg/registry"
	"github.com/run-ai/karta/exporter/pkg/store"
)

// Build computes the stored state of a workload from its live object:
// matched normalized statuses of the root, and per component instance the
// desired replicas. A root without a StatusDefinition yields HasStatus false,
// which is a different fact from Undefined (mappings present, none matched).
func Build(ctx context.Context, entry *registry.Entry, workload resource.KubernetesObject, uid types.UID, ref store.WorkloadRef) (store.WorkloadRecord, error) {
	factory := resource.NewComponentFactoryFromObject(entry.Karta, workload)

	record := store.WorkloadRecord{
		UID:   uid,
		Ref:   ref,
		Karta: entry.Karta.Name,
	}

	root, err := factory.GetRootComponent()
	if err != nil {
		return record, fmt.Errorf("failed to get root component: %w", err)
	}

	var statusErr error
	status, err := root.GetStatus(ctx)
	switch {
	case err != nil:
		statusErr = fmt.Errorf("failed to evaluate status: %w", err)
	case status != nil:
		record.HasStatus = true
		record.Phases = status.MatchedStatuses
		if len(record.Phases) == 0 {
			record.Phases = []v1alpha1.ResourceStatus{v1alpha1.UndefinedStatus}
		}
	}

	components, err := factory.GetChildComponents()
	if err != nil {
		return record, fmt.Errorf("failed to get child components: %w", err)
	}
	components = append(components, root)

	for _, component := range components {
		states, err := componentStates(ctx, component)
		if err != nil {
			return record, fmt.Errorf("failed to extract component %s: %w", component.Name(), err)
		}
		record.Components = append(record.Components, states...)
	}

	return record, statusErr
}

// componentStates lists the instances of one component with their desired
// replicas. Components without a pod definition and without a scale are
// skipped: they produce no pods and no replica expectation.
func componentStates(ctx context.Context, component *resource.Component) ([]store.ComponentState, error) {
	scales, err := component.GetScale(ctx)
	if err != nil {
		return nil, err
	}
	if scales == nil && !component.HasPodDefinition() {
		return nil, nil
	}

	instanceIds, err := component.GetInstanceIds(ctx)
	if err != nil {
		return nil, err
	}

	states := make([]store.ComponentState, 0, len(instanceIds))
	for _, instanceID := range instanceIds {
		componentState := store.ComponentState{
			Component: component.Name(),
			Instance:  instanceID,
		}
		if scale, ok := scales[instanceID]; ok {
			componentState.Replicas = scale.Replicas
		}
		states = append(states, componentState)
	}
	return states, nil
}
