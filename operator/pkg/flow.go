// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// StepFn is a single reconciliation step for a Karta. Steps are chained by
// reconcile(); returning a StepResult with continueReconcile=false short-
// circuits the chain and returns the result directly to the manager.
type StepFn func(ctx context.Context, log logr.Logger, karta *kartav1alpha1.Karta) StepResult

// StepResult is the outcome of a single reconciliation step.
type StepResult struct {
	result            ctrl.Result
	err               error
	continueReconcile bool
}

// Result converts a StepResult to the (ctrl.Result, error) pair expected by
// controller-runtime.
func (r StepResult) Result() (ctrl.Result, error) {
	return r.result, r.err
}

// Continue signals that the step succeeded and the next step should run.
func Continue() StepResult {
	return StepResult{continueReconcile: true}
}

// StopWithError signals that the step failed. The manager will requeue with
// exponential back-off.
func StopWithError(err error) StepResult {
	return StepResult{result: ctrl.Result{Requeue: true}, err: err, continueReconcile: false}
}

// ShortCircuit returns true when the step chain should stop.
func (r StepResult) ShortCircuit() bool {
	return !r.continueReconcile
}
