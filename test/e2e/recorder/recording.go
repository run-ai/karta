// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

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
	Succeeded     bool   `json:"succeeded"`
	KartaFile     string `json:"kartaFile"` // repo-relative path to the Karta definition
	Steps         []Step `json:"steps"`
	Path          string `json:"-"` // where the run was written; set by the recorder, not serialized
}

// Step is one observed state: the own-fields State (never from Karta), the CR (full on the first step,
// a merge-patch after), and the action fired here if any.
type Step struct {
	State  string                 `json:"state"`
	CR     map[string]interface{} `json:"cr,omitempty"`
	Patch  map[string]interface{} `json:"patch,omitempty"`
	Action *ActionRecord          `json:"action,omitempty"`
}

// ActionRecord is a mutation fired at a step: the request sent to the apiserver and the object it returned.
// (Distinct from Action in action.go, which is the patch the flow declares.)
type ActionRecord struct {
	Type     ActionType             `json:"type"`
	Request  map[string]interface{} `json:"request"`
	Response map[string]interface{} `json:"response,omitempty"`
}

// States is the ordered own-fields states the recording passed through.
func (r Recording) States() []string {
	out := make([]string, len(r.Steps))
	for i, s := range r.Steps {
		out[i] = s.State
	}
	return out
}

// CRs rebuilds the full CR at each step from the merge-patches.
func (r Recording) CRs() ([]*unstructured.Unstructured, error) {
	if len(r.Steps) == 0 {
		return nil, nil
	}
	if r.Steps[0].CR == nil {
		return nil, fmt.Errorf("recording %s/%s: first step missing its full CR", r.Operator, r.Flow)
	}
	cur := runtime.DeepCopyJSON(r.Steps[0].CR)
	out := []*unstructured.Unstructured{{Object: runtime.DeepCopyJSON(cur)}}
	for _, s := range r.Steps[1:] {
		patch := s.Patch
		if patch == nil {
			patch = map[string]interface{}{}
		}
		applyMergePatch(cur, patch)
		out = append(out, &unstructured.Unstructured{Object: runtime.DeepCopyJSON(cur)})
	}
	return out, nil
}

// RecordingPath is the on-disk path of a flow's recording under fixturesRoot.
func RecordingPath(fixturesRoot string, r Recording) string {
	return filepath.Join(fixturesRoot, r.Operator, r.Version, r.KartaName, r.Flow+".yaml")
}

func WriteRecording(path string, r Recording) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func LoadRecording(path string) (Recording, error) {
	var r Recording
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	if err := yaml.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
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
