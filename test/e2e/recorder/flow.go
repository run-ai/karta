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

// Flow is a workload journey recorded for testing: the workload manifest plus the ordered stops the workload
// must reach.
type Flow struct {
	rec      *Recorder
	name     string
	manifest string
	journey  []journeyStep
}

// journeyStep is one stop on a journey. State borrows Karta's ResourceStatus as the shared vocabulary so the
// replay compares one-to-one; the value is judged from the workload's own fields, never from Karta.
// ActionPredicate lets the same state appear more than once (a scale flow is Running at 1, 3, then 1): the
// step is reached only once the predicate holds, performing its action.
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

// Reaches adds a stop the workload must pass through.
func (f *Flow) Reaches(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, journeyStep{State: state})
	return f
}

// OptionalReaches adds a stop the workload may skip; the order check tolerates its absence.
func (f *Flow) OptionalReaches(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, journeyStep{State: state, Optional: true})
	return f
}

// At adds a stop to gate with When, or attach an action with Do.
func (f *Flow) At(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, journeyStep{State: state})
	return f
}

// When gates the current stop on a predicate over the workload's own fields.
func (f *Flow) When(gate StateCheck) *Flow { f.last().ActionPredicate = gate; return f }

// Do attaches an action performed once the current stop is reached.
func (f *Flow) Do(action *Action) *Flow { f.last().Action = action; return f }

func (f *Flow) last() *journeyStep { return &f.journey[len(f.journey)-1] }

func (f *Flow) want() kartav1alpha1.ResourceStatus { return f.journey[len(f.journey)-1].State }

func (f *Flow) client() client.Client { return f.rec.config.Cluster.Client }
func (f *Flow) log() io.Writer        { return f.rec.config.Log }

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
