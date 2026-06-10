// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"fmt"
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
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

	// Default rate-limiter delays. Override via KARTA_RATE_LIMITER_BASE_DELAY
	// and KARTA_RATE_LIMITER_MAX_DELAY environment variables.
	DefaultRateLimiterBaseDelay = 500 * time.Millisecond
	DefaultRateLimiterMaxDelay  = 60 * time.Second
)

// RateLimiterConfig holds the exponential back-off parameters for the
// reconcile queue.
type RateLimiterConfig struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// Reconciler reconciles Karta CRs.
type Reconciler struct {
	client.Client
	rateLimiter RateLimiterConfig
	recorder    record.EventRecorder
}

// NewReconciler constructs a new Reconciler.
func NewReconciler(c client.Client, rl RateLimiterConfig, recorder record.EventRecorder) *Reconciler {
	return &Reconciler{Client: c, rateLimiter: rl, recorder: recorder}
}

// SetupWithManager registers the reconciler with the given manager.
//
// Two watches are registered:
//  1. Karta — all create/update/delete events trigger Reconcile.
//  2. CustomResourceDefinition — events are mapped to the Kartas that
//     reference the same group+kind.
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
				r.rateLimiter.BaseDelay, r.rateLimiter.MaxDelay,
			),
		}).
		Complete(r)
}

// Reconcile is the main reconciliation entry point .
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

	logger.Info("Reconciling Karta")
	return r.reconcile(ctx, karta)
}
