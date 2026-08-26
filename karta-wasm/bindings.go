// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog"
	"github.com/run-ai/karta/pkg/instructions"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

type PodAttribution struct {
	PodIndex      int     `json:"podIndex"`
	ComponentName string  `json:"componentName"`
	InstanceKey   *string `json:"instanceKey,omitempty"`
}

func jsBuildTree(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return encodeEnvelope(nil, fmt.Errorf("kartaBuildTree: expected 2 arguments, got %d", len(args)))
	}
	definitionJSON := args[0].String()
	workloadJSON := args[1].String()

	factory, err := decodeFactory(definitionJSON, workloadJSON)
	if err != nil {
		return encodeEnvelope(nil, err)
	}

	workloadTree, err := tree.Build(context.Background(), factory)
	return encodeEnvelope(workloadTree, err)
}

func jsAttributePods(_ js.Value, args []js.Value) any {
	if len(args) != 3 {
		return encodeEnvelope(nil, fmt.Errorf("kartaAttributePods: expected 3 arguments, got %d", len(args)))
	}
	definitionJSON := args[0].String()
	workloadJSON := args[1].String()
	podsJSON := args[2].String()
	ctx := context.Background()

	var karta v1alpha1.Karta
	if err := json.Unmarshal([]byte(definitionJSON), &karta); err != nil {
		return encodeEnvelope(nil, fmt.Errorf("failed to unmarshal definition: %w", err))
	}

	factory, err := decodeFactory(definitionJSON, workloadJSON)
	if err != nil {
		return encodeEnvelope(nil, err)
	}

	summary, err := instructions.NewStructureSummary(&karta)
	if err != nil {
		return encodeEnvelope(nil, fmt.Errorf("failed to build structure summary: %w", err))
	}

	var pods []corev1.Pod
	if err := json.Unmarshal([]byte(podsJSON), &pods); err != nil {
		return encodeEnvelope(nil, fmt.Errorf("failed to unmarshal pods: %w", err))
	}

	attributions := make([]PodAttribution, 0, len(pods))
	for i := range pods {
		querier := resource.NewPodQuerier(&pods[i])

		componentName, err := instructions.InferPodComponent(ctx, querier, summary)
		if err != nil {
			continue
		}

		instanceKey, err := instructions.InferPodComponentInstance(ctx, querier, componentName, factory)
		if err != nil {
			continue
		}

		attributions = append(attributions, PodAttribution{PodIndex: i, ComponentName: componentName, InstanceKey: instanceKey})
	}

	return encodeEnvelope(attributions, nil)
}

func jsEvaluatePhases(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return encodeEnvelope(nil, fmt.Errorf("kartaEvaluatePhases: expected 2 arguments, got %d", len(args)))
	}
	definitionJSON := args[0].String()
	workloadJSON := args[1].String()

	factory, err := decodeFactory(definitionJSON, workloadJSON)
	if err != nil {
		return encodeEnvelope(nil, err)
	}

	workloadTree, err := tree.Build(context.Background(), factory)
	if err != nil {
		return encodeEnvelope(nil, err)
	}
	if workloadTree.Status == nil {
		return encodeEnvelope([]string{}, nil)
	}
	return encodeEnvelope(workloadTree.Status.Phases, nil)
}

func jsListCatalog(js.Value, []js.Value) any {
	return encodeEnvelope(catalog.List(), nil)
}
