// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

// Command wasm exposes the Karta tree builder to the browser. The Headlamp
// plugin instantiates the compiled module and calls kartaBuildTree with the
// Karta definition and workload object as JSON strings. Building is pure
// computation on those two documents, so no Kubernetes access is needed.
package main

import (
	"context"
	"encoding/json"
	"syscall/js"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/pkg/tree"
)

func errorResult(message string) js.Value {
	return js.ValueOf(map[string]any{"error": message})
}

// buildTree implements kartaBuildTree(kartaJSON, workloadJSON). It returns
// {tree: <JSON string>} on success and {error: <message>} on failure.
func buildTree(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return errorResult("kartaBuildTree expects (kartaJSON, workloadJSON)")
	}

	karta := &v1alpha1.Karta{}
	if err := json.Unmarshal([]byte(args[0].String()), karta); err != nil {
		return errorResult("decode karta: " + err.Error())
	}
	workload := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(args[1].String()), workload); err != nil {
		return errorResult("decode workload: " + err.Error())
	}
	if err := v1alpha1.NewKartaValidator(karta).Validate(); err != nil {
		return errorResult("invalid karta: " + err.Error())
	}

	factory := resource.NewComponentFactoryFromObject(karta, workload)
	workloadTree, err := tree.Build(context.Background(), factory)
	if err != nil {
		return errorResult("build tree: " + err.Error())
	}

	out, err := json.Marshal(workloadTree)
	if err != nil {
		return errorResult("encode tree: " + err.Error())
	}
	return js.ValueOf(map[string]any{"tree": string(out)})
}

// matchPods implements kartaMatchPods(kartaJSON, podListJSON). It classifies
// pods by the definition's componentTypeSelector and, when a
// componentInstanceSelector is defined, extracts the instance key. It returns
// {matches: {<pod name>: {component: <name>, instance: <key>}}} or
// {error: <message>}. Selectors identify component membership only, not
// workload identity; the caller must scope the pod list to one workload.
func matchPods(_ js.Value, args []js.Value) any {
	if len(args) != 2 {
		return errorResult("kartaMatchPods expects (kartaJSON, podListJSON)")
	}

	karta := &v1alpha1.Karta{}
	if err := json.Unmarshal([]byte(args[0].String()), karta); err != nil {
		return errorResult("decode karta: " + err.Error())
	}
	var pods []corev1.Pod
	if err := json.Unmarshal([]byte(args[1].String()), &pods); err != nil {
		return errorResult("decode pods: " + err.Error())
	}

	structure := karta.Spec.StructureDefinition
	components := append([]v1alpha1.ComponentDefinition{structure.RootComponent}, structure.ChildComponents...)

	ctx := context.Background()
	matches := map[string]any{}
	for i := range pods {
		querier := resource.NewPodQuerier(&pods[i])
		for _, component := range components {
			selector := component.PodSelector
			if selector == nil {
				continue
			}

			// Membership is established by the componentTypeSelector when
			// defined; otherwise a pod carrying the instance key of a
			// componentInstanceSelector also proves membership (e.g. JobSet
			// definitions select purely by replicated-job label).
			if selector.ComponentTypeSelector != nil {
				matched, err := querier.MatchesComponentType(ctx, selector.ComponentTypeSelector)
				if err != nil {
					return errorResult("match pod " + pods[i].Name + " against component " + component.Name + ": " + err.Error())
				}
				if !matched {
					continue
				}
			} else if selector.ComponentInstanceSelector == nil {
				continue
			}

			match := map[string]any{"component": component.Name}
			if selector.ComponentInstanceSelector != nil {
				// Extraction fails when the pod lacks the instance key, which
				// simply means the pod does not carry this selector.
				instance, found, err := querier.ExtractInstanceId(ctx, selector.ComponentInstanceSelector)
				if err != nil || !found {
					if selector.ComponentTypeSelector == nil {
						// Instance-only selector: the key is required for membership.
						continue
					}
				} else {
					match["instance"] = instance
				}
			}
			matches[pods[i].Name] = match
			break
		}
	}

	return js.ValueOf(map[string]any{"matches": matches})
}

func main() {
	js.Global().Set("kartaBuildTree", js.FuncOf(buildTree))
	js.Global().Set("kartaMatchPods", js.FuncOf(matchPods))
	// Keep the Go runtime alive so the exported functions stay callable.
	select {}
}
