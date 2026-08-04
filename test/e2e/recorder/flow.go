// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Recorder records the flows of one workload type. Bind it to the type's Karta definition and state
// registry, then build and Run a Flow per case.
type Recorder struct {
	operator  string
	kartaName string
	kartaFile string
	states    []NamedState
	timeout   time.Duration
}

// New starts a recorder for the given operator key, Karta definition name, and Karta YAML path. The path is
// recording metadata (so the replay golden can load the definition); New does not touch the cluster.
func New(operator, kartaName, kartaFile string) *Recorder {
	return &Recorder{operator: operator, kartaName: kartaName, kartaFile: kartaFile, timeout: 3 * time.Minute}
}

// State registers how to recognise a state from the workload's own fields; declare them least to most
// advanced (Classify keeps the furthest match).
func (r *Recorder) State(name kartav1alpha1.ResourceStatus, match StateCheck) *Recorder {
	r.states = append(r.states, NamedState{Name: name, Match: match})
	return r
}

// Timeout overrides the per-flow deadline (default 3m).
func (r *Recorder) Timeout(d time.Duration) *Recorder { r.timeout = d; return r }

// Flow starts a flow named name, seeded from the workload manifest (a path relative to test/e2e).
func (r *Recorder) Flow(name, manifest string) *Flow {
	return &Flow{rec: r, name: name, manifest: manifest}
}

// Flow is a declared journey: a manifest to apply, then the ordered stops the workload must reach. A stop
// carrying an action predicate and/or an action is a checkpoint the recorder drives to.
type Flow struct {
	rec      *Recorder
	name     string
	manifest string
	journey  []journeyStep
}

// Reaches adds a plain stop: the workload must classify as state here.
func (f *Flow) Reaches(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, journeyStep{State: state})
	return f
}

// Maybe adds an optional stop the workload may skip (a transient dip); the order check tolerates it.
func (f *Flow) Maybe(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, journeyStep{State: state, Optional: true})
	return f
}

// At adds a stop to be gated with When/WaitUntil and/or fired with Do.
func (f *Flow) At(state kartav1alpha1.ResourceStatus) *Flow {
	f.journey = append(f.journey, journeyStep{State: state})
	return f
}

// When gates the current stop: it is not reached until this predicate over the workload's own fields holds.
func (f *Flow) When(gate StateCheck) *Flow { f.last().ActionPredicate = gate; return f }

// WaitUntil is When for the terminal stop - the flow finishes once it holds.
func (f *Flow) WaitUntil(gate StateCheck) *Flow { f.last().ActionPredicate = gate; return f }

// Do fires an action when the current stop is reached.
func (f *Flow) Do(action *Action) *Flow { f.last().Action = action; return f }

func (f *Flow) last() *journeyStep { return &f.journey[len(f.journey)-1] }

// want is the flow's terminal state: the last stop's state.
func (f *Flow) want() kartav1alpha1.ResourceStatus { return f.journey[len(f.journey)-1].State }

// StateCheck recognises a state from the workload's own fields, never from Karta. Flows build these with
// the predicate helpers in the flows package.
type StateCheck func(*unstructured.Unstructured) bool

// NamedState pairs a state with the predicate that recognises it.
type NamedState struct {
	Name  kartav1alpha1.ResourceStatus
	Match StateCheck
}

// journeyStep is one stop on a flow's journey: a state to reach, an optional action, and an optional
// ActionPredicate that gates the step. ActionPredicate lets a journey list the same state more than once
// (a scale flow is Running at 1, 3, then 1 replica): the step is not reached until the predicate over the
// workload's own fields holds, and its action fires then. (Distinct from the on-disk Step in recording.go.)
type journeyStep struct {
	State           kartav1alpha1.ResourceStatus
	Action          *Action
	ActionPredicate StateCheck
	// Optional marks a transient step the workload may miss (a scale dip): the order check tolerates it
	// being absent. It is not a drive stop - only checkpoints (steps with an action or predicate) are.
	Optional bool
}

// steps builds a plain journey (states only), used to declare an expected order.
func steps(states ...kartav1alpha1.ResourceStatus) []journeyStep {
	j := make([]journeyStep, len(states))
	for i, s := range states {
		j[i] = journeyStep{State: s}
	}
	return j
}

// Classify returns the furthest-along state the workload matches, judged from its own fields.
func Classify(u *unstructured.Unstructured, states []NamedState) kartav1alpha1.ResourceStatus {
	var name kartav1alpha1.ResourceStatus
	for _, s := range states {
		if s.Match(u) {
			name = s.Name
		}
	}
	return name
}

// ActionType names the mutations a flow can fire to drive a transition the operator will not make itself.
type ActionType string

const (
	ActionResume ActionType = "Resume"
	ActionScale  ActionType = "Scale"
)

// Action is a merge-patch of Type applied to the workload at a step. The recorder sends the patch and
// records the request and response. Flows build these with the action helpers in the flows package.
type Action struct {
	Type  ActionType
	Patch []byte
}
