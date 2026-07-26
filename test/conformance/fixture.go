// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package conformance is the shared seam between the live e2e recorder (test/e2e) and the offline
// transition test. A recording is one flow of one operator: the workload's first CR in full, then a
// merge-patch per state change, each tagged with the state it reaches. Both sides read a CR through
// the same Read helper, so what Karta reads can never drift between record and replay. Recordings
// live under test/conformance/fixtures/<operator>/<version>/<kartaName>/<flow>/recording.yaml.
package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// SchemaVersion is bumped when the on-disk format changes incompatibly.
// v5: a recording is first-CR + per-state merge-patch + the state each CR reaches, replacing the
// sanitized full CR per snapshot. There is no sanitize; the test asserts states, not raw bytes.
const SchemaVersion = 5

const recordingFile = "recording.yaml"

// Recording is one flow: metadata plus an ordered list of steps, one per state the workload passed
// through. Step 0 carries the full first CR; every later step carries a merge-patch from the CR
// before it. The set of recordings under test/conformance/fixtures/ is the tested operator matrix.
type Recording struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Operator      string                  `json:"operator"`
	Version       string                  `json:"version"`
	KartaName     string                  `json:"kartaName"`
	Flow          string                  `json:"flow"`
	Want          v1alpha1.ResourceStatus `json:"want,omitempty"`
	KartaFile     string                  `json:"kartaFile"` // repo-relative path to the Karta definition
	Steps         []Step                  `json:"steps"`
}

// Step is one observed state. State is judged from the workload's own fields, never from Karta. The
// first step carries CR (the full object); every later step carries Patch (an RFC 7386 merge-patch
// from the previous CR). Action names the mutation fired when this state was reached, if any.
type Step struct {
	State  string                 `json:"state"`
	Action string                 `json:"action,omitempty"`
	CR     map[string]interface{} `json:"cr,omitempty"`
	Patch  map[string]interface{} `json:"patch,omitempty"`
}

// States is the ordered list of states the recording passed through.
func (r Recording) States() []string {
	out := make([]string, len(r.Steps))
	for i, s := range r.Steps {
		out[i] = s.State
	}
	return out
}

// CRs reconstructs the full CR at each step by applying the merge-patches in order onto the first CR.
func (r Recording) CRs() ([]*unstructured.Unstructured, error) {
	if len(r.Steps) == 0 {
		return nil, nil
	}
	if r.Steps[0].CR == nil {
		return nil, fmt.Errorf("recording %s/%s: first step has no cr", r.Operator, r.Flow)
	}
	cur := runtime.DeepCopyJSON(r.Steps[0].CR)
	out := []*unstructured.Unstructured{{Object: runtime.DeepCopyJSON(cur)}}
	for i, s := range r.Steps[1:] {
		if s.Patch == nil {
			return nil, fmt.Errorf("recording %s/%s: step %d has no patch", r.Operator, r.Flow, i+1)
		}
		applyMergePatch(cur, s.Patch)
		out = append(out, &unstructured.Unstructured{Object: runtime.DeepCopyJSON(cur)})
	}
	return out, nil
}

// WriteRecording persists a recording as a single recording.yaml under root, replacing any prior one.
func WriteRecording(root string, r Recording) error {
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	return writeYAML(filepath.Join(root, recordingFile), r)
}

// LoadRecording reads the recording.yaml under root.
func LoadRecording(root string) (Recording, error) {
	var r Recording
	err := readYAML(filepath.Join(root, recordingFile), &r)
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
