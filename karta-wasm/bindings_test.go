// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/karta/test/types"
)

func TestJsBuildTree(t *testing.T) {
	definitionJSON := marshal(t, types.ReactorKarta())
	workloadJSON := marshal(t, types.NewReactorObject())

	env := jsBuildTree(js.Value{}, []js.Value{js.ValueOf(definitionJSON), js.ValueOf(workloadJSON)}).(js.Value)
	if !env.Get("error").IsNull() {
		t.Fatalf("unexpected error: %s", env.Get("error").String())
	}

	var tree struct {
		Status   *struct{ Phases []string }
		Children []struct{ Name string }
	}
	if err := json.Unmarshal([]byte(env.Get("data").String()), &tree); err != nil {
		t.Fatalf("failed to unmarshal tree: %v", err)
	}
	if tree.Status == nil || len(tree.Status.Phases) != 1 || tree.Status.Phases[0] != "Running" {
		t.Fatalf("expected Status.Phases = [Running], got %#v", tree.Status)
	}
	if len(tree.Children) != 1 || tree.Children[0].Name != "service" {
		t.Fatalf("expected a single %q component, got %#v", "service", tree.Children)
	}
}

func TestJsBuildTree_WrongArgCount(t *testing.T) {
	env := jsBuildTree(js.Value{}, []js.Value{js.ValueOf("{}")}).(js.Value)

	if env.Get("error").IsNull() {
		t.Fatal("expected an error for a missing argument")
	}
}

func TestJsAttributePods(t *testing.T) {
	definitionJSON := marshal(t, types.PyFlowKarta())
	workloadJSON := marshal(t, types.NewPyFlowObject())
	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "worker-0", Labels: map[string]string{"role": "worker"}}},
	}

	env := jsAttributePods(js.Value{}, []js.Value{
		js.ValueOf(definitionJSON), js.ValueOf(workloadJSON), js.ValueOf(marshal(t, pods)),
	}).(js.Value)
	if !env.Get("error").IsNull() {
		t.Fatalf("unexpected error: %s", env.Get("error").String())
	}

	var attributions []PodAttribution
	if err := json.Unmarshal([]byte(env.Get("data").String()), &attributions); err != nil {
		t.Fatalf("failed to unmarshal attributions: %v", err)
	}
	if len(attributions) != 1 || attributions[0].ComponentName != "worker" {
		t.Fatalf("expected a single attribution to %q, got %#v", "worker", attributions)
	}
}

func TestJsAttributePods_WrongArgCount(t *testing.T) {
	env := jsAttributePods(js.Value{}, []js.Value{js.ValueOf("{}"), js.ValueOf("{}")}).(js.Value)

	if env.Get("error").IsNull() {
		t.Fatal("expected an error for a missing argument")
	}
}

func TestJsEvaluatePhases(t *testing.T) {
	definitionJSON := marshal(t, types.ReactorKarta())
	workloadJSON := marshal(t, types.NewReactorObject())

	env := jsEvaluatePhases(js.Value{}, []js.Value{js.ValueOf(definitionJSON), js.ValueOf(workloadJSON)}).(js.Value)
	if !env.Get("error").IsNull() {
		t.Fatalf("unexpected error: %s", env.Get("error").String())
	}

	var phases []string
	if err := json.Unmarshal([]byte(env.Get("data").String()), &phases); err != nil {
		t.Fatalf("failed to unmarshal phases: %v", err)
	}
	if len(phases) != 1 || phases[0] != "Running" {
		t.Fatalf("expected phases = [Running], got %#v", phases)
	}
}

func TestJsEvaluatePhases_WrongArgCount(t *testing.T) {
	env := jsEvaluatePhases(js.Value{}, nil).(js.Value)

	if env.Get("error").IsNull() {
		t.Fatal("expected an error for a missing argument")
	}
}

func TestJsListCatalog(t *testing.T) {
	env := jsListCatalog(js.Value{}, nil).(js.Value)
	if !env.Get("error").IsNull() {
		t.Fatalf("unexpected error: %s", env.Get("error").String())
	}

	var entries []map[string]any
	if err := json.Unmarshal([]byte(env.Get("data").String()), &entries); err != nil {
		t.Fatalf("failed to unmarshal catalog: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected the embedded catalog to be non-empty")
	}
}
