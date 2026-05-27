// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"context"
	"fmt"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcile runs the ordered step chain for one Karta. Every step continues
// to the next so we always produce a complete status picture. The chain only
// short-circuits on hard errors (StopWithError).
func (r *Reconciler) reconcile(ctx context.Context, karta *kartav1alpha1.Karta) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("karta", karta.Name)
	original := karta.Status.DeepCopy()

	steps := []StepFn{
		r.stepValidateKarta,
		r.stepCheckCRDExists,
		r.stepDeriveReady,
	}
	for _, step := range steps {
		if res := step(ctx, logger, karta); shortCircuit(res) {
			return res.Result()
		}
	}

	if err := r.patchStatusIfChanged(ctx, karta, original); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status for karta %q: %w", karta.Name, err)
	}
	return ctrl.Result{}, nil
}

// stepValidateKarta runs the Karta spec validator and writes KartaValidated.
func (r *Reconciler) stepValidateKarta(_ context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) StepResult {
	if err := kartav1alpha1.NewKartaValidator(karta).Validate(); err != nil {
		logger.V(1).Info("Karta spec validation failed", "error", err.Error())
		setKartaValidated(&karta.Status, metav1.ConditionFalse)
	} else {
		setKartaValidated(&karta.Status, metav1.ConditionTrue)
	}
	return Continue()
}

// stepCheckCRDExists looks up the referenced CRD and writes CRDExists.
func (r *Reconciler) stepCheckCRDExists(ctx context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) StepResult {
	gvk := rootGVK(karta)
	if gvk == nil {
		logger.V(1).Info("Karta has no root component kind; leaving CRDExists=False")
		setCRDExists(&karta.Status, metav1.ConditionFalse)
		return Continue()
	}

	exists, err := r.crdExistsForGVK(ctx, *gvk)
	if err != nil {
		// Transient API-server error: log and leave CRDExists=False. The CRD
		// watch will re-trigger this reconcile when the situation resolves, so
		// we do not requeue here to avoid a storm.
		logger.Error(err, "Failed to check CRD existence", "gvk", gvk.String())
		setCRDExists(&karta.Status, metav1.ConditionFalse)
		return Continue()
	}

	if exists {
		setCRDExists(&karta.Status, metav1.ConditionTrue)
	} else {
		setCRDExists(&karta.Status, metav1.ConditionFalse)
	}
	return Continue()
}

// stepDeriveReady sets Ready based on the KartaValidated and CRDExists
// conditions already written to karta.Status by the preceding steps.
func (r *Reconciler) stepDeriveReady(_ context.Context, _ logr.Logger, karta *kartav1alpha1.Karta) StepResult {
	validated := conditionStatus(&karta.Status, kartav1alpha1.ConditionKartaValidated)
	crdExists := conditionStatus(&karta.Status, kartav1alpha1.ConditionCRDExists)
	setReady(&karta.Status, validated, crdExists)
	return Continue()
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

// crdMatchesGVK returns true when the CRD covers the given group, kind and
// version (storage or otherwise).
func crdMatchesGVK(crd *apiextensionsv1.CustomResourceDefinition, gvk schema.GroupVersionKind) bool {
	if crd.Spec.Group != gvk.Group || crd.Spec.Names.Kind != gvk.Kind {
		return false
	}
	for _, v := range crd.Spec.Versions {
		if v.Name == gvk.Version {
			return true
		}
	}
	return false
}

// conditionStatus reads the current Status of the named condition from the
// Karta status, returning ConditionFalse when not found.
func conditionStatus(status *kartav1alpha1.KartaStatus, t kartav1alpha1.ConditionType) metav1.ConditionStatus {
	for _, c := range status.Conditions {
		if c.Type == string(t) {
			return c.Status
		}
	}
	return metav1.ConditionFalse
}
