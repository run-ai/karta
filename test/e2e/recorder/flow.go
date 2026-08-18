// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// NewFlow starts a flow seeded from a manifest (path relative to test/e2e); declare its journey with Through.
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

// Through declares the ordered stops of the journey.
func (f *Flow) Through(steps ...Step) *Flow {
	for _, s := range steps {
		f.journey = append(f.journey, s.step)
	}
	return f
}

// Step is one declared stop, built with Reaches and refined with Optional, When, and Do.
type Step struct {
	step journeyStep
}

// Reaches declares a stop the workload must pass through.
func Reaches(state kartav1alpha1.ResourceStatus) Step {
	return Step{step: journeyStep{State: state}}
}

// Optional marks a stop the workload may skip; the order check tolerates its absence.
func (s Step) Optional() Step { s.step.Optional = true; return s }

// When gates the stop on a predicate over the workload's own fields.
func (s Step) When(gate StateCheck) Step { s.step.ActionPredicate = gate; return s }

// Do attaches an action performed once the stop is reached.
func (s Step) Do(action *Action) Step { s.step.Action = action; return s }

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

// StateCheck recognises a state from the workload's own fields, never from Karta.
type StateCheck func(*unstructured.Unstructured) bool

type namedState struct {
	Name  kartav1alpha1.ResourceStatus
	Match StateCheck
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

// classify returns the furthest-along state the workload matches, judged from its own fields; states are
// declared least- to most-advanced, so the walk runs from the end.
func classify(cr *unstructured.Unstructured, states []namedState) kartav1alpha1.ResourceStatus {
	for i := len(states) - 1; i >= 0; i-- {
		if states[i].Match(cr) {
			return states[i].Name
		}
	}
	return ""
}

func (f *Flow) want() kartav1alpha1.ResourceStatus { return f.journey[len(f.journey)-1].State }

func (f *Flow) client() client.Client { return f.rec.config.Cluster.Client }
func (f *Flow) log() io.Writer        { return f.rec.config.Log }
