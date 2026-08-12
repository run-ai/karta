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

// schemaVersion is bumped on incompatible format changes; v2 is the event stream (STATE and ACTION events).
const schemaVersion = 2

// Recording is one flow: metadata plus the ordered event stream a workload produced.
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

const (
	EventState  = "STATE"
	EventAction = "ACTION"
)

// Event is one entry in the stream: a STATE event carries the full object and its own-fields state; an
// ACTION event carries the mutation the flow performed to drive the next transition. StaleObservedGeneration marks a
// frame captured before the controller observed the spec: recorded, but outside the order-checked walk.
type Event struct {
	Kind                    string          `json:"kind"`
	State                   string          `json:"state,omitempty"`
	StaleObservedGeneration bool            `json:"staleObservedGeneration,omitempty"`
	Object                  map[string]any  `json:"object,omitempty"`
	Action                  *RecordedAction `json:"action,omitempty"`
}

// RecordedAction is a mutation performed between states.
type RecordedAction struct {
	Name      string    `json:"name"`
	Operation Operation `json:"operation"`
}

type Operation struct {
	Verb      string         `json:"verb"`
	PatchType string         `json:"patchType"`
	Payload   map[string]any `json:"payload"`
}

// states is the ordered own-fields states the recording passed through (STATE events only).
func (r Recording) states() []string {
	var out []string
	for _, e := range r.Events {
		if e.Kind == EventState {
			out = append(out, e.State)
		}
	}
	return out
}

func recordingPath(fixturesRoot string, r Recording) string {
	return filepath.Join(fixturesRoot, r.Operator, r.Version, r.KartaName, r.Flow+".yaml")
}

func writeRecording(path string, r Recording) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	b, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal recording %s: %w", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func loadRecording(path string) (Recording, error) {
	var r Recording
	b, err := os.ReadFile(path)
	if err != nil {
		return r, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

// Reader walks a recording's STATE events (Next, then State/Object); ACTION events are skipped.
type Reader struct {
	rec         Recording
	stateEvents []Event
	pos         int
}

func OpenRecording(path string) (*Reader, error) {
	rec, err := loadRecording(path)
	if err != nil {
		return nil, err
	}
	return newReader(rec), nil
}

func newReader(rec Recording) *Reader {
	var stateEvents []Event
	for _, e := range rec.Events {
		if e.Kind == EventState {
			stateEvents = append(stateEvents, e)
		}
	}
	return &Reader{rec: rec, stateEvents: stateEvents, pos: -1}
}

func (r *Reader) Next() bool {
	r.pos++
	return r.pos < len(r.stateEvents)
}

func (r *Reader) State() string { return r.stateEvents[r.pos].State }

func (r *Reader) Object() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: r.stateEvents[r.pos].Object}
}

func (r *Reader) Recording() Recording { return r.rec }
