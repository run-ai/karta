// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"context"
	"fmt"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcile is the per-Karta reconciliation logic. It:
//  1. Validates the Karta spec                    (Story 1.2 — KartaValidated)
//  2. Checks the referenced CRD exists            (Story 1.3 — CRDExists)
//  3. Derives the aggregate Ready condition        (Story 1.4 — Ready)
//  4. Patches Karta status if anything changed
func (r *Reconciler) reconcile(ctx context.Context, karta *kartav1alpha1.Karta) (ctrl.Result, error) {
	original := karta.Status.DeepCopy()

	in := r.computeConditions(ctx, karta)
	setConditions(&karta.Status, in)

	if err := r.patchStatusIfChanged(ctx, karta, original); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status for karta %q: %w", karta.Name, err)
	}
	return ctrl.Result{}, nil
}

// computeConditions evaluates KartaValidated and CRDExists independently.
// Both start as False; each flips to True only if the check passes.
func (r *Reconciler) computeConditions(ctx context.Context, karta *kartav1alpha1.Karta) conditionInputs {
	logger := log.FromContext(ctx).WithValues("karta", karta.Name)

	in := conditionInputs{
		kartaValidated: metav1.ConditionFalse,
		crdExists:      metav1.ConditionFalse,
	}

	// Story 1.2 — KartaValidated
	if err := kartav1alpha1.NewKartaValidator(karta).Validate(); err != nil {
		logger.V(1).Info("Karta spec validation failed", "error", err.Error())
	} else {
		in.kartaValidated = metav1.ConditionTrue
	}

	// Story 1.3 — CRDExists
	gvk := rootGVK(karta)
	if gvk == nil {
		logger.V(1).Info("Karta has no root component kind; leaving CRDExists=False")
		return in
	}

	exists, err := r.crdExistsForGVK(ctx, *gvk)
	if err != nil {
		// Do not return the error: a transient API-server failure should not
		// cause endless requeue storms. The CRD watch will re-trigger this
		// reconcile when the CRD situation resolves.
		logger.Error(err, "Failed to check CRD existence", "gvk", gvk.String())
		return in
	}
	if exists {
		in.crdExists = metav1.ConditionTrue
	}

	return in
}

// crdExistsForGVK reports whether a CRD serving the given group, version and
// kind is present in the cluster. It returns false (not an error) when the
// CRD simply does not exist.
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

// crdMatchesGVK returns true when the CRD covers the given group and kind and
// lists the requested version (regardless of whether it is the storage version).
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
