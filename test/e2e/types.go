// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package e2e runs the Karta end-to-end suite against a real cluster provisioned by
// hack/e2e/up.sh. It is its own Go module so the cluster deps stay out of the library.
package e2e

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// stateCheck recognises a state from the workload's own fields, never from Karta.
type stateCheck func(*unstructured.Unstructured) bool

// stateAction drives a transition the operator will not make itself, e.g. unsuspend.
type stateAction func(ctx context.Context, obj *unstructured.Unstructured) error

type namedState struct {
	name  kartav1alpha1.ResourceStatus
	ready stateCheck
}

// step is one stop on a journey: a state to reach and an optional action fired when it does.
type step struct {
	state  kartav1alpha1.ResourceStatus
	action stateAction
}

// flow drives a workload along an ordered journey of states. mayGoBackwards waives the
// strict-order check for an operator whose observed order Karta cannot make reliable.
type flow struct {
	name           string
	workloadFile   string
	journey        []step
	mayGoBackwards bool
}

func (f flow) want() kartav1alpha1.ResourceStatus { return f.journey[len(f.journey)-1].state }

type workloadCase struct {
	name      string
	operator  string
	kartaFile string
	kartaName string
	states    []namedState // state registry, ordered least to most advanced
	flows     []flow
	timeout   time.Duration
}

// steps builds action-less journey stops: journey: steps(InitializingStatus, RunningStatus).
func steps(states ...kartav1alpha1.ResourceStatus) []step {
	j := make([]step, len(states))
	for i, s := range states {
		j[i] = step{state: s}
	}
	return j
}

func (tc workloadCase) validate() error {
	known := map[kartav1alpha1.ResourceStatus]bool{}
	for _, s := range tc.states {
		known[s.name] = true
	}
	for _, fl := range tc.flows {
		if len(fl.journey) == 0 {
			return fmt.Errorf("case %q flow %q: empty journey", tc.name, fl.name)
		}
		for _, st := range fl.journey {
			if !known[st.state] {
				return fmt.Errorf("case %q flow %q: state %q not in registry", tc.name, fl.name, st.state)
			}
		}
	}
	return nil
}
