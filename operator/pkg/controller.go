// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
	ControllerName = "karta-controller"

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

// Reconcile is the main reconciliation entry point.
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

// reconcile runs the reconciliation logic for one Karta.
func (r *Reconciler) reconcile(ctx context.Context, karta *kartav1alpha1.Karta) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx).WithValues("karta", karta.Name)
	base := karta.DeepCopy()

	defer func() {
		if patchErr := r.patchStatusIfChanged(ctx, karta, base); patchErr != nil {
			patchErr = fmt.Errorf("update status for karta %q: %w", karta.Name, patchErr)
			if err == nil {
				err = patchErr
			} else {
				logger.Error(patchErr, "failed to patch status after reconcile error")
			}
		}
	}()

	r.validateKarta(logger, karta)
	if err = r.checkCRDExists(ctx, logger, karta); err != nil {
		return
	}
	r.deriveReady(logger, karta)
	err = r.ensureLabels(ctx, logger, karta)
	return
}

// validateKarta runs the Karta spec validator and writes the Validated condition.
// A Warning event is emitted on every reconcile where validation fails so that
// `kubectl describe karta` always shows a fresh, counted event.
func (r *Reconciler) validateKarta(logger logr.Logger, karta *kartav1alpha1.Karta) {
	if err := kartav1alpha1.NewKartaValidator(karta).Validate(); err != nil {
		logger.Info("Karta spec validation failed", "error", err.Error())
		r.recorder.Event(karta, corev1.EventTypeWarning, ReasonValidationFailed, err.Error())
		setValidated(&karta.Status, karta.Generation, metav1.ConditionFalse, err.Error())
		return
	}
	logger.V(1).Info("Karta spec validated")
	setValidated(&karta.Status, karta.Generation, metav1.ConditionTrue, "")
}

// checkCRDExists looks up the referenced CRD and writes the CRDExists condition.
// It returns an error only on transient API failures; a missing CRD is not an
// error — it sets CRDExists=False and returns nil.
func (r *Reconciler) checkCRDExists(ctx context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) error {
	gvk := rootGVK(karta)
	if gvk == nil {
		logger.V(1).Info("karta has no root component kind")
		setCRDExists(&karta.Status, karta.Generation, metav1.ConditionFalse, "")
		return nil
	}

	exists, err := r.crdExistsForGVK(ctx, *gvk)
	if err != nil {
		logger.Error(err, "Failed to check CRD existence", "gvk", gvk.String())
		return fmt.Errorf("check CRD existence for karta %q: %w", karta.Name, err)
	}

	if exists {
		logger.V(1).Info("CRD found", "gvk", gvk.String())
		setCRDExists(&karta.Status, karta.Generation, metav1.ConditionTrue, "")
	} else {
		msg := fmt.Sprintf("CRD for %s/%s not found or does not serve version %s",
			gvk.Group, gvk.Kind, gvk.Version)
		logger.V(1).Info("CRD not found", "gvk", gvk.String())
		r.recorder.Event(karta, corev1.EventTypeWarning, ReasonCRDNotFound, msg)
		setCRDExists(&karta.Status, karta.Generation, metav1.ConditionFalse, msg)
	}
	return nil
}

// deriveReady sets the Ready condition based on the Validated and CRDExists
// conditions already written to karta.Status by the preceding calls.
func (r *Reconciler) deriveReady(logger logr.Logger, karta *kartav1alpha1.Karta) {
	statusOf := func(t kartav1alpha1.ConditionType) metav1.ConditionStatus {
		if c := apimeta.FindStatusCondition(karta.Status.Conditions, string(t)); c != nil {
			return c.Status
		}
		return metav1.ConditionFalse
	}
	setReady(&karta.Status, karta.Generation, statusOf(kartav1alpha1.ConditionValidated), statusOf(kartav1alpha1.ConditionCRDExists))
	logger.V(1).Info("Derived Ready condition", "ready", statusOf(kartav1alpha1.ConditionReady))
}

// ensureLabels stamps the three GVK index labels (run.ai/karta-group,
// run.ai/karta-version, run.ai/karta-kind) onto the Karta metadata so that
// consumers can locate a Karta by GVK via a label-selector List.
func (r *Reconciler) ensureLabels(ctx context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) error {
	gvk := rootGVK(karta)
	if gvk == nil {
		return r.removeIndexLabels(ctx, logger, karta)
	}

	desired := map[string]string{
		kartav1alpha1.LabelRootGroup:   gvk.Group,
		kartav1alpha1.LabelRootVersion: gvk.Version,
		kartav1alpha1.LabelRootKind:    gvk.Kind,
	}

	if labelsMatch(karta.Labels, desired) {
		return nil
	}

	patchBytes, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"labels": desired},
	})
	if err != nil {
		return fmt.Errorf("marshal label patch for karta %q: %w", karta.Name, err)
	}

	patchTarget := karta.DeepCopy()
	if err = r.Patch(ctx, patchTarget, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("patch labels for karta %q: %w", karta.Name, err)
	}
	karta.Labels = patchTarget.Labels
	karta.ResourceVersion = patchTarget.ResourceVersion

	logger.V(1).Info("Stamped GVK index labels",
		"group", gvk.Group, "version", gvk.Version, "kind", gvk.Kind)
	return nil
}

// removeIndexLabels deletes the three GVK index labels from the Karta metadata
// via a JSON merge-patch (setting each key to null removes it).
// It is a no-op when none of the labels are present.
func (r *Reconciler) removeIndexLabels(ctx context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) error {
	if !hasAnyIndexLabel(karta.Labels) {
		return nil
	}

	patchBytes, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				kartav1alpha1.LabelRootGroup:   nil,
				kartav1alpha1.LabelRootVersion: nil,
				kartav1alpha1.LabelRootKind:    nil,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal label-removal patch for karta %q: %w", karta.Name, err)
	}

	// Patch a copy — see note in ensureLabels.
	patchTarget := karta.DeepCopy()
	if err = r.Patch(ctx, patchTarget, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("remove stale index labels for karta %q: %w", karta.Name, err)
	}
	karta.Labels = patchTarget.Labels
	karta.ResourceVersion = patchTarget.ResourceVersion

	logger.V(1).Info("Removed stale GVK index labels (root kind no longer set)")
	return nil
}

// labelsMatch returns true when current already contains all desired key/value pairs.
func labelsMatch(current, desired map[string]string) bool {
	for k, v := range desired {
		if current[k] != v {
			return false
		}
	}
	return true
}

// hasAnyIndexLabel reports whether the Karta has any of the three GVK index labels.
func hasAnyIndexLabel(labels map[string]string) bool {
	if len(labels) == 0 {
		return false
	}
	for _, k := range []string{
		kartav1alpha1.LabelRootGroup,
		kartav1alpha1.LabelRootVersion,
		kartav1alpha1.LabelRootKind,
	} {
		if _, ok := labels[k]; ok {
			return true
		}
	}
	return false
}

// rootGVK extracts the GroupVersionKind of the Karta root component, or nil
// when the Karta has no root component kind defined.
func rootGVK(karta *kartav1alpha1.Karta) *schema.GroupVersionKind {
	kind := karta.Spec.StructureDefinition.RootComponent.Kind
	if kind == nil {
		return nil
	}
	return &schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}
}

// crdExistsForGVK reports whether a CRD serving the given group, version and
// kind is present in the cluster.
func (r *Reconciler) crdExistsForGVK(ctx context.Context, gvk schema.GroupVersionKind) (bool, error) {
	crds := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := r.List(ctx, crds); err != nil {
		return false, fmt.Errorf("list CRDs: %w", err)
	}

	for i := range crds.Items {
		if crdMatchesGVK(&crds.Items[i], gvk) {
			return true, nil
		}
	}
	return false, nil
}

func crdMatchesGVK(crd *apiextensionsv1.CustomResourceDefinition, gvk schema.GroupVersionKind) bool {
	if crd.Spec.Group != gvk.Group || crd.Spec.Names.Kind != gvk.Kind {
		return false
	}
	for _, v := range crd.Spec.Versions {
		if v.Name == gvk.Version && v.Served {
			return true
		}
	}
	return false
}
