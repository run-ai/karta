// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// SchemaVersion is bumped when the on-disk format changes incompatibly. v2 is the event stream (STATE and
// ACTION events, each STATE carrying the full object); v1 was a merge-patch diff with the action coupled
// onto the step.
const SchemaVersion = 2

// Recording is one flow: metadata plus the ordered event stream a workload produced - the states it passed
// through and the actions the flow fired between them.
type Recording struct {
	SchemaVersion int     `json:"schemaVersion"`
	Operator      string  `json:"operator"`
	Version       string  `json:"version"`
	KartaName     string  `json:"kartaName"`
	Flow          string  `json:"flow"`
	Want          string  `json:"want,omitempty"`
	Succeeded     bool    `json:"succeeded"`
	KartaFile     string  `json:"kartaFile"` // repo-relative path to the Karta definition
	Events        []Event `json:"events"`
	Path          string  `json:"-"` // where the run was written; set by the recorder, not serialized
}

// Event kinds.
const (
	EventState  = "STATE"
	EventAction = "ACTION"
)

// Event is one entry in the recording stream, decoupling observed states from fired actions. A STATE event
// carries the full object and the state read from its own fields (never from Karta); an ACTION event
// carries the mutation the flow fired to drive the next transition.
type Event struct {
	Kind   string                 `json:"kind"`
	State  string                 `json:"state,omitempty"`
	Object map[string]interface{} `json:"object,omitempty"`
	Action *RecordedAction        `json:"action,omitempty"`
}

// RecordedAction is a mutation fired between states: a named apiserver operation.
type RecordedAction struct {
	Name      string    `json:"name"`
	Operation Operation `json:"operation"`
}

// Operation is the apiserver call an action made.
type Operation struct {
	Verb      string                 `json:"verb"`
	PatchType string                 `json:"patchType"`
	Payload   map[string]interface{} `json:"payload"`
}

// States is the ordered own-fields states the recording passed through (STATE events only).
func (r Recording) States() []string {
	var out []string
	for _, e := range r.Events {
		if e.Kind == EventState {
			out = append(out, e.State)
		}
	}
	return out
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

// Reader replays a recording's STATE events one at a time: call Next to advance, then State and Object for
// the current state. ACTION events are metadata for the flow and are skipped by the walk.
type Reader struct {
	rec    Recording
	states []Event
	pos    int
}

// OpenRecording loads a recording file and prepares to walk its states.
func OpenRecording(path string) (*Reader, error) {
	rec, err := LoadRecording(path)
	if err != nil {
		return nil, err
	}
	return NewReader(rec), nil
}

// NewReader walks an in-memory recording's STATE events.
func NewReader(rec Recording) *Reader {
	var states []Event
	for _, e := range rec.Events {
		if e.Kind == EventState {
			states = append(states, e)
		}
	}
	return &Reader{rec: rec, states: states, pos: -1}
}

// Next advances to the next STATE event and reports whether one is available.
func (r *Reader) Next() bool {
	r.pos++
	return r.pos < len(r.states)
}

// State is the current STATE event's own-fields state (never from Karta).
func (r *Reader) State() string { return r.states[r.pos].State }

// Object is the current STATE event's full CR.
func (r *Reader) Object() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: r.states[r.pos].Object}
}

// Recording is the underlying recording (metadata and all events).
func (r *Reader) Recording() Recording { return r.rec }
