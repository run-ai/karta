// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// NewFlow starts a flow seeded from a manifest (path relative to test/e2e).
func NewFlow(r *Recorder, name, manifest string) *Flow {
	return &Flow{rec: r, name: name, manifest: manifest}
}

// Flow is a declared journey: a manifest plus the ordered stops the workload must reach.
type Flow struct {
	rec      *Recorder
	name     string
	manifest string
	journey  []journeyStep
}

func (f *Flow) Reaches(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, journeyStep{State: state})
	return f
}

// Maybe adds an optional stop the workload may skip; the order check tolerates its absence.
func (f *Flow) Maybe(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, journeyStep{State: state, Optional: true})
	return f
}

// At adds a stop to gate with When/WaitUntil, or attach an action with Do.
func (f *Flow) At(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, journeyStep{State: state})
	return f
}

// When gates the current stop on a predicate over the workload's own fields.
func (f *Flow) When(gate StateCheck) *Flow { f.last().ActionPredicate = gate; return f }

// WaitUntil is When for the terminal stop.
func (f *Flow) WaitUntil(gate StateCheck) *Flow { f.last().ActionPredicate = gate; return f }

func (f *Flow) Do(action *Action) *Flow { f.last().Action = action; return f }

func (f *Flow) last() *journeyStep { return &f.journey[len(f.journey)-1] }

func (f *Flow) want() kartav1alpha1.ResourceStatus { return f.journey[len(f.journey)-1].State }

// client and log are shortcuts to the recorder's bound cluster access, so the driving code reads cleanly.
func (f *Flow) client() client.Client { return f.rec.cluster.Client }
func (f *Flow) log() io.Writer        { return f.rec.log }

// StateCheck recognises a state from the workload's own fields, never from Karta.
type StateCheck func(*unstructured.Unstructured) bool

type namedState struct {
	Name  kartav1alpha1.ResourceStatus
	Match StateCheck
}

// classify returns the furthest-along state the workload matches, judged from its own fields.
func classify(cr *unstructured.Unstructured, states []namedState) kartav1alpha1.ResourceStatus {
	var name kartav1alpha1.ResourceStatus
	for _, s := range states {
		if s.Match(cr) {
			name = s.Name
		}
	}
	return name
}

// journeyStep is one stop on a journey. ActionPredicate lets the same state appear more than once (a scale
// flow is Running at 1, 3, then 1): the step is reached only once the predicate holds, firing its action.
type journeyStep struct {
	State           kartav1alpha1.ResourceStatus
	Action          *Action
	ActionPredicate StateCheck
	Optional        bool // a transient step the workload may miss; the order check tolerates its absence
}

type ActionType string

const (
	ActionResume ActionType = "Resume"
	ActionScale  ActionType = "Scale"
)

// Action is a merge-patch applied to the workload to drive a transition.
type Action struct {
	Type  ActionType
	Patch []byte
}
