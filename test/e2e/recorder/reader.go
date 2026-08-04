// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Reader replays a recording written by the recorder: given a recording file it hands back each
// step (state and action) and the fully reconstructed CR, one Next() at a time.
package recorder

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Reader replays a recording step by step: call Next to advance, then Step and CR for the current step.
type Reader struct {
	rec Recording
	crs []*unstructured.Unstructured
	pos int
}

// OpenRecording loads a recording file and reconstructs every step's CR.
func OpenRecording(path string) (*Reader, error) {
	rec, err := LoadRecording(path)
	if err != nil {
		return nil, err
	}
	return NewReader(rec)
}

// NewReader builds a Reader over an in-memory recording.
func NewReader(rec Recording) (*Reader, error) {
	crs, err := rec.CRs()
	if err != nil {
		return nil, err
	}
	return &Reader{rec: rec, crs: crs, pos: -1}, nil
}

// Next advances to the next step and reports whether one is available.
func (r *Reader) Next() bool {
	r.pos++
	return r.pos < len(r.rec.Steps)
}

// Step is the current step: its state and the action fired there, if any.
func (r *Reader) Step() Step { return r.rec.Steps[r.pos] }

// CR is the current step's fully reconstructed CR.
func (r *Reader) CR() *unstructured.Unstructured { return r.crs[r.pos] }

// Recording is the underlying recording (metadata and all steps).
func (r *Reader) Recording() Recording { return r.rec }
