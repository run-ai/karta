// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package controller implements the OSS Karta operator reconciliation logic.
//
// The operator watches Karta CRs and CustomResourceDefinitions and maintains
// three status conditions on every Karta:
//   - KartaValidated — spec is structurally valid (Story 1.2)
//   - CRDExists      — referenced CRD is present in the cluster (Story 1.3)
//   - Ready          — derived: True iff both above are True (Story 1.4)
//
// The operator is intentionally stateless and idempotent. It does not manage
// RBAC, finalizers, or any consumer-specific concerns — those are left to
// downstream consumers (e.g., RunAI EWI, anyworkload-controller).
package controller

import (
	"context"
	"fmt"
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// ControllerName is the name used to identify this controller in
	// controller-runtime metrics and logs.
	ControllerName = "karta-operator"

	rateLimiterBaseDelay = 500 * time.Millisecond
	rateLimiterMaxDelay  = 60 * time.Second
)

// Reconciler reconciles Karta CRs.
//
// It watches Karta and CustomResourceDefinition objects. The per-Karta
// reconciliation logic lives in the (currently empty) reconcile() hook,
// which subsequent stories populate. It does not own any RBAC, finalizer,
// or consumer-specific behavior.
type Reconciler struct {
	client.Client
}

// NewReconciler constructs a new Reconciler.
func NewReconciler(c client.Client) *Reconciler {
	return &Reconciler{Client: c}
}

// SetupWithManager registers the reconciler with the given manager.
//
// Two watches are registered:
//  1. Karta — all create/update/delete events trigger Reconcile.
//  2. CustomResourceDefinition — events are mapped to the Kartas that
//     reference the same group+kind. A GenerationChangedPredicate is applied
//     so that only CRD spec changes (versions added/removed, served flag
//     flipped) and create/delete events fire the mapping. Noisy CRD status
//     updates (e.g. "Established" condition ticks) are dropped.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&kartav1alpha1.Karta{}).
		Watches(
			&apiextensionsv1.CustomResourceDefinition{},
			handler.EnqueueRequestsFromMapFunc(r.MapCRDToKartaEvent),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](
				rateLimiterBaseDelay, rateLimiterMaxDelay,
			),
		}).
		Complete(r)
}

// Reconcile is the main reconciliation entry point invoked by the
// controller-runtime manager whenever a watched object changes.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("karta", req.Name)

	karta := &kartav1alpha1.Karta{}
	if err := r.Get(ctx, req.NamespacedName, karta); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("Karta not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get karta %q: %w", req.Name, err)
	}

	if !karta.DeletionTimestamp.IsZero() {
		logger.V(1).Info("Karta is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	logger.V(1).Info("Reconciling Karta")
	return r.reconcile(ctx, karta)
}
