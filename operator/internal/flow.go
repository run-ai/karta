// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package internal

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// StepFn is a single reconciliation step for a Karta. Steps are chained by
// reconcile(); returning a StepResult with continueReconcile=false short-
// circuits the chain and returns the result directly to the manager.
//
// Inspired by Grove's ReconcileStepFn pattern. Generics are omitted because
// the operator reconciles exactly one CR type (Karta). If a second CR is ever
// added, the signature can be genericized at that point.
type StepFn func(ctx context.Context, log logr.Logger, karta *kartav1alpha1.Karta) StepResult

// StepResult is the outcome of a single reconciliation step.
type StepResult struct {
	result    ctrl.Result
	err       error
	continue_ bool
}

// Result converts a StepResult to the (ctrl.Result, error) pair expected by
// controller-runtime.
func (r StepResult) Result() (ctrl.Result, error) {
	return r.result, r.err
}

// Continue signals that the step succeeded and the next step should run.
func Continue() StepResult {
	return StepResult{continue_: true}
}

// Stop signals that reconciliation is done (no error, no requeue).
func Stop() StepResult {
	return StepResult{continue_: false}
}

// StopWithError signals that the step failed. The manager will requeue with
// exponential back-off.
func StopWithError(err error) StepResult {
	return StepResult{result: ctrl.Result{Requeue: true}, err: err, continue_: false}
}

// shortCircuit returns true when the step chain should stop.
func shortCircuit(r StepResult) bool {
	return !r.continue_
}
