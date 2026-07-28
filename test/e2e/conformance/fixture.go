// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package conformance is the shared seam between the live e2e recorder (test/e2e) and the offline
// golden test. A recording is one flow of one operator: for each state the workload passed through
// it stores the workload's own-fields state, the CR, and what Karta read - the CR and the reading
// both as a first value plus a merge-patch per state change, so the file holds only what changed.
// The recorder writes recordings; the offline test rebuilds each CR, re-reads it through the current
// Karta, and diffs against the stored reading. Recordings live under
// test/e2e/conformance/fixtures/<operator>/<version>/<kartaName>/<flow>.yaml.
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
// v6: one <flow>.yaml per flow; each step carries the own-fields state, the CR (first + merge-patch),
// and Karta's reading (first + merge-patch). No sanitize.
const SchemaVersion = 6

// Recording is one flow: metadata plus an ordered list of steps, one per state the workload passed
// through. The set of recordings under test/e2e/conformance/fixtures/ is the tested operator matrix.
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

// Step is one observed state. State is the workload's own-fields state (never from Karta). CR and
// Expected are the full values on the first step; Patch and ExpectedPatch are RFC 7386 merge-patches
// from the previous step on every later step. Action names the mutation fired at this state, if any.
type Step struct {
	State         string                 `json:"state"`
	Action        string                 `json:"action,omitempty"`
	CR            map[string]interface{} `json:"cr,omitempty"`
	Patch         map[string]interface{} `json:"patch,omitempty"`
	Expected      map[string]interface{} `json:"expected,omitempty"`
	ExpectedPatch map[string]interface{} `json:"expectedPatch,omitempty"`
}

// States is the ordered list of own-fields states the recording passed through.
func (r Recording) States() []string {
	out := make([]string, len(r.Steps))
	for i, s := range r.Steps {
		out[i] = s.State
	}
	return out
}

// CRs reconstructs the full CR at each step by applying the CR merge-patches in order.
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

// Readings reconstructs Karta's stored reading at each step by applying the reading merge-patches.
func (r Recording) Readings() ([]map[string]interface{}, error) {
	return reconstruct(r, func(s Step) (map[string]interface{}, map[string]interface{}) { return s.Expected, s.ExpectedPatch })
}

// reconstruct walks the steps applying pick(step)'s first value (step 0) then merge-patch (later
// steps), returning the full value at each step.
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

// RecordingPath is the on-disk path of a flow's recording, relative to fixturesRoot.
func RecordingPath(fixturesRoot string, r Recording) string {
	return filepath.Join(fixturesRoot, r.Operator, r.Version, r.KartaName, r.Flow+".yaml")
}

// WriteRecording writes a recording to a single <flow>.yaml file, creating parent dirs.
func WriteRecording(path string, r Recording) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeYAML(path, r)
}

// LoadRecording reads a <flow>.yaml recording.
func LoadRecording(path string) (Recording, error) {
	var r Recording
	err := readYAML(path, &r)
	return r, err
}

// MergePatch computes the RFC 7386 merge-patch that turns prev into cur: keys only in prev are
// nulled, keys added or changed are set, nested objects recurse, and arrays or scalars that differ
// are replaced whole.
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

// applyMergePatch applies an RFC 7386 merge-patch onto target in place.
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
