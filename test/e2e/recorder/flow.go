// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"io"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Cluster is the access a recorder needs: the clients, the Kubernetes/server version (recorded as the
// version for built-in workloads no operator provides), the throwaway namespace workloads are created in,
// and a writer for progress lines (nil discards). The suite builds one and passes it to New.
type Cluster struct {
	Client    client.Client
	Dynamic   dynamic.Interface
	Version   string
	Namespace string
	Progress  io.Writer
}

// Recorder records the flows of one workload type: build and Run a Flow per case.
type Recorder struct {
	cluster   Cluster
	operator  string
	kartaName string
	kartaFile string
	states    []NamedState
	timeout   time.Duration
}

// New starts a recorder bound to cluster; kartaFile is recorded as metadata for the replay golden, not read
// here.
func New(cluster Cluster, operator, kartaName, kartaFile string) *Recorder {
	if cluster.Progress == nil {
		cluster.Progress = io.Discard
	}
	return &Recorder{cluster: cluster, operator: operator, kartaName: kartaName, kartaFile: kartaFile, timeout: 3 * time.Minute}
}

// AddState registers a state predicate; declare states least- to most-advanced (Classify keeps the furthest match).
func (r *Recorder) AddState(name kartav1alpha1.ResourceStatus, match StateCheck) *Recorder {
	r.states = append(r.states, NamedState{Name: name, Match: match})
	return r
}

// SetTimeout overrides the per-flow deadline (default 3m).
func (r *Recorder) SetTimeout(d time.Duration) *Recorder { r.timeout = d; return r }

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

// At adds a stop to gate with When/WaitUntil or fire with Do.
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

// StateCheck recognises a state from the workload's own fields, never from Karta.
type StateCheck func(*unstructured.Unstructured) bool

type NamedState struct {
	Name  kartav1alpha1.ResourceStatus
	Match StateCheck
}

// journeyStep is one stop on a journey. ActionPredicate lets the same state appear more than once (a scale
// flow is Running at 1, 3, then 1): the step is reached only once the predicate holds, firing its action.
type journeyStep struct {
	State           kartav1alpha1.ResourceStatus
	Action          *Action
	ActionPredicate StateCheck
	Optional        bool // a transient step the workload may miss; the order check tolerates its absence
}

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
