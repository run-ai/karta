// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// decodeFactory decodes a JSON Karta definition and a JSON workload object
// into the ComponentFactory that tree building, pod attribution, and phase
// evaluation are all computed against.
func decodeFactory(definitionJSON, workloadJSON string) (*resource.ComponentFactory, error) {
	var karta v1alpha1.Karta
	if err := json.Unmarshal([]byte(definitionJSON), &karta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal definition: %w", err)
	}

	var workload map[string]interface{}
	if err := json.Unmarshal([]byte(workloadJSON), &workload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workload: %w", err)
	}

	return resource.NewComponentFactoryFromObject(&karta, &unstructured.Unstructured{Object: workload}), nil
}

// encodeEnvelope marshals v (on success) or err (on failure) into the
// {data: string|null, error: string|null} JSON envelope every binding returns,
// so the TS side has one shared contract to unwrap regardless of the call.
func encodeEnvelope(v any, err error) js.Value {
	envelope := map[string]any{"data": nil, "error": nil}
	if err != nil {
		envelope["error"] = err.Error()
		return js.ValueOf(envelope)
	}

	data, marshalErr := json.Marshal(v)
	if marshalErr != nil {
		envelope["error"] = marshalErr.Error()
		return js.ValueOf(envelope)
	}

	envelope["data"] = string(data)
	return js.ValueOf(envelope)
}
