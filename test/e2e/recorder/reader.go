// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
