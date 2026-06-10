// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"encoding/json"
	"fmt"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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
func (r *Reconciler) validateKarta(logger logr.Logger, karta *kartav1alpha1.Karta) {
	if err := kartav1alpha1.NewKartaValidator(karta).Validate(); err != nil {
		logger.Info("Karta spec validation failed", "error", err.Error())
		setValidated(&karta.Status, metav1.ConditionFalse, err.Error())
		return
	}
	logger.V(1).Info("Karta spec validated")
	setValidated(&karta.Status, metav1.ConditionTrue, "")

}

// checkCRDExists looks up the referenced CRD and writes the CRDExists condition.
// It returns an error only on transient API failures; a missing CRD is not an
// error — it sets CRDExists=False and returns nil.
func (r *Reconciler) checkCRDExists(ctx context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) error {
	gvk := rootGVK(karta)
	if gvk == nil {
		logger.V(1).Info("karta has no root component kind")
		setCRDExists(&karta.Status, metav1.ConditionFalse, "")
		return nil
	}

	exists, err := r.crdExistsForGVK(ctx, *gvk)
	if err != nil {
		logger.Error(err, "Failed to check CRD existence", "gvk", gvk.String())
		return fmt.Errorf("check CRD existence for karta %q: %w", karta.Name, err)
	}

	if exists {
		logger.V(1).Info("CRD found", "gvk", gvk.String())
		setCRDExists(&karta.Status, metav1.ConditionTrue, "")
	} else {
		msg := fmt.Sprintf("CRD for %s/%s not found or does not serve version %s",
			gvk.Group, gvk.Kind, gvk.Version)
		logger.V(1).Info("CRD not found", "gvk", gvk.String())
		setCRDExists(&karta.Status, metav1.ConditionFalse, msg)
	}
	return nil
}

// deriveReady sets the Ready condition based on the Validated and CRDExists
// conditions already written to karta.Status by the preceding calls.
func (r *Reconciler) deriveReady(logger logr.Logger, karta *kartav1alpha1.Karta) {
	validated := conditionStatus(&karta.Status, kartav1alpha1.ConditionValidated)
	crdExists := conditionStatus(&karta.Status, kartav1alpha1.ConditionCRDExists)
	setReady(&karta.Status, validated, crdExists)
	ready := conditionStatus(&karta.Status, kartav1alpha1.ConditionReady)
	logger.V(1).Info("Derived Ready condition", "ready", ready)
}

// ensureLabels stamps the karta/gvk index label onto the Karta metadata so
// that consumers can locate a Karta by GVK via a label-selector List.
// The value is encoded as "group__version__kind" (see LabelGVK).
func (r *Reconciler) ensureLabels(ctx context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) error {
	gvk := rootGVK(karta)
	if gvk == nil {
		return r.removeIndexLabel(ctx, logger, karta)
	}

	desired := map[string]string{
		kartav1alpha1.LabelGVK: kartav1alpha1.FormatGVKLabel(gvk.Group, gvk.Version, gvk.Kind),
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

	if err = r.Patch(ctx, karta, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("patch labels for karta %q: %w", karta.Name, err)
	}

	logger.V(1).Info("Stamped GVK index label", "gvk", desired[kartav1alpha1.LabelGVK])
	return nil
}

// removeIndexLabel deletes the karta/gvk label from the Karta metadata via a
// JSON merge-patch. It is a no-op when the label is not present.
func (r *Reconciler) removeIndexLabel(ctx context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) error {
	if _, ok := karta.Labels[kartav1alpha1.LabelGVK]; !ok {
		return nil
	}

	patchBytes, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{kartav1alpha1.LabelGVK: nil},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal label-removal patch for karta %q: %w", karta.Name, err)
	}

	if err = r.Patch(ctx, karta, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("remove stale index label for karta %q: %w", karta.Name, err)
	}

	logger.V(1).Info("Removed stale GVK index label (root kind no longer set)")
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
