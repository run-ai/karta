// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
)

// reconcile is the per-Karta hook invoked by Reconcile after the Karta has
// been fetched and lifecycle checks have passed.
func (r *Reconciler) reconcile(_ context.Context, _ *kartav1alpha1.Karta) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
