// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package conformance is the recording format shared by the live recorder and the offline golden: for
// each state a workload passes through it stores the own-fields state, the CR, and Karta's reading, each
// as a first value plus a per-step merge-patch, so a recording holds only what changed between states.
package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// SchemaVersion is bumped when the on-disk format changes incompatibly.
const SchemaVersion = 1

// Recording is one flow: metadata plus the ordered steps a workload passed through.
type Recording struct {
	SchemaVersion int    `json:"schemaVersion"`
	Operator      string `json:"operator"`
	Version       string `json:"version"`
	KartaName     string `json:"kartaName"`
	Flow          string `json:"flow"`
	Want          string `json:"want,omitempty"`
	KartaFile     string `json:"kartaFile"` // repo-relative path to the Karta definition
	Steps         []Step `json:"steps"`
}

// Step is one observed state: State (own-fields, never from Karta), the CR and Karta's reading (full on
// the first step, RFC 7386 merge-patches after), and the Action fired here if any.
type Step struct {
	State         string                 `json:"state"`
	Action        string                 `json:"action,omitempty"`
	CR            map[string]interface{} `json:"cr,omitempty"`
	Patch         map[string]interface{} `json:"patch,omitempty"`
	Expected      map[string]interface{} `json:"expected,omitempty"`
	ExpectedPatch map[string]interface{} `json:"expectedPatch,omitempty"`
}

func (r Recording) States() []string {
	out := make([]string, len(r.Steps))
	for i, s := range r.Steps {
		out[i] = s.State
	}
	return out
}

func (r Recording) CRs() ([]*unstructured.Unstructured, error) {
	series, err := reconstruct(r, func(s Step) (map[string]interface{}, map[string]interface{}) { return s.CR, s.Patch })
	if err != nil {
		return nil, err
	}
	out := make([]*unstructured.Unstructured, len(series))
	for i, m := range series {
		out[i] = &unstructured.Unstructured{Object: m}
	}
	return out, nil
}

func (r Recording) Readings() ([]map[string]interface{}, error) {
	return reconstruct(r, func(s Step) (map[string]interface{}, map[string]interface{}) { return s.Expected, s.ExpectedPatch })
}

// reconstruct rebuilds the full value at each step: the first step's value, then merge-patches in order.
func reconstruct(r Recording, pick func(Step) (first, patch map[string]interface{})) ([]map[string]interface{}, error) {
	if len(r.Steps) == 0 {
		return nil, nil
	}
	first, _ := pick(r.Steps[0])
	if first == nil {
		return nil, fmt.Errorf("recording %s/%s: first step missing its full value", r.Operator, r.Flow)
	}
	cur := runtime.DeepCopyJSON(first)
	out := []map[string]interface{}{runtime.DeepCopyJSON(cur)}
	for _, s := range r.Steps[1:] {
		_, patch := pick(s)
		if patch == nil {
			patch = map[string]interface{}{}
		}
		applyMergePatch(cur, patch)
		out = append(out, runtime.DeepCopyJSON(cur))
	}
	return out, nil
}

func RecordingPath(fixturesRoot string, r Recording) string {
	return filepath.Join(fixturesRoot, r.Operator, r.Version, r.KartaName, r.Flow+".yaml")
}

func WriteRecording(path string, r Recording) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeYAML(path, r)
}

func LoadRecording(path string) (Recording, error) {
	var r Recording
	err := readYAML(path, &r)
	return r, err
}

// MergePatch is the RFC 7386 merge-patch from prev to cur: removed keys nulled, objects recursed, arrays
// and scalars replaced whole.
func MergePatch(prev, cur map[string]interface{}) map[string]interface{} {
	patch := map[string]interface{}{}
	for k := range prev {
		if _, ok := cur[k]; !ok {
			patch[k] = nil
		}
	}
	for k, cv := range cur {
		pv, existed := prev[k]
		if !existed {
			patch[k] = cv
			continue
		}
		cm, cIsMap := cv.(map[string]interface{})
		pm, pIsMap := pv.(map[string]interface{})
		if cIsMap && pIsMap {
			if sub := MergePatch(pm, cm); len(sub) > 0 {
				patch[k] = sub
			}
			continue
		}
		if !reflect.DeepEqual(pv, cv) {
			patch[k] = cv
		}
	}
	return patch
}

func applyMergePatch(target, patch map[string]interface{}) {
	for k, pv := range patch {
		if pv == nil {
			delete(target, k)
			continue
		}
		pm, pIsMap := pv.(map[string]interface{})
		if !pIsMap {
			target[k] = pv
			continue
		}
		tm, tIsMap := target[k].(map[string]interface{})
		if !tIsMap {
			tm = map[string]interface{}{}
		}
		applyMergePatch(tm, pm)
		target[k] = tm
	}
}

func writeYAML(path string, v interface{}) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func readYAML(path string, v interface{}) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
