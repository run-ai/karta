// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package internal

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

// reconcile runs the ordered step chain for one Karta.
//
// Status is the user-facing API contract, so the condition-computing steps
// run first and the status patch happens before the best-effort label
// stamping. If label stamping fails (e.g., transient API error or missing
// RBAC) the user still gets correct status conditions; controller-runtime
// will requeue and retry the label patch on the next reconcile.
//
// Steps are otherwise independent — each writes its own slice of state and
// returns Continue so the rest of the chain still runs. Only a hard error
// (StopWithError) short-circuits the chain.
func (r *Reconciler) reconcile(ctx context.Context, karta *kartav1alpha1.Karta) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("karta", karta.Name)
	original := karta.Status.DeepCopy()

	steps := []StepFn{
		r.stepValidateKarta,
		r.stepCheckCRDExists,
		r.stepDeriveReady,
		stepPatchStatusWith(r, original),
		r.stepEnsureLabels,
	}
	for _, step := range steps {
		if res := step(ctx, logger, karta); shortCircuit(res) {
			return res.Result()
		}
	}
	return ctrl.Result{}, nil
}

// stepPatchStatusWith returns a step that flushes status to the cluster,
// closing over the snapshot taken at the start of reconcile so we only
// patch when something actually changed.
func stepPatchStatusWith(r *Reconciler, original *kartav1alpha1.KartaStatus) StepFn {
	return func(ctx context.Context, _ logr.Logger, karta *kartav1alpha1.Karta) StepResult {
		if err := r.patchStatusIfChanged(ctx, karta, original); err != nil {
			return StopWithError(fmt.Errorf("update status for karta %q: %w", karta.Name, err))
		}
		return Continue()
	}
}

// stepValidateKarta runs the Karta spec validator and writes Validated.
func (r *Reconciler) stepValidateKarta(_ context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) StepResult {
	if err := kartav1alpha1.NewKartaValidator(karta).Validate(); err != nil {
		logger.V(1).Info("Karta spec validation failed", "error", err.Error())
		setValidated(&karta.Status, metav1.ConditionFalse)
	} else {
		setValidated(&karta.Status, metav1.ConditionTrue)
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

// stepDeriveReady sets Ready based on the Validated and CRDExists conditions
// already written to karta.Status by the preceding steps.
func (r *Reconciler) stepDeriveReady(_ context.Context, _ logr.Logger, karta *kartav1alpha1.Karta) StepResult {
	validated := conditionStatus(&karta.Status, kartav1alpha1.ConditionValidated)
	crdExists := conditionStatus(&karta.Status, kartav1alpha1.ConditionCRDExists)
	setReady(&karta.Status, validated, crdExists)
	return Continue()
}

// stepEnsureLabels stamps the GVK-derived index labels (karta/group,
// karta/version, karta/kind) onto the Karta metadata so that consumers and
// the CRD event mapper can locate a Karta by GVK via a label-selector List
// instead of fetching all Kartas.
//
// This step runs last so that label-patch failures (transient API errors,
// missing RBAC) never block the status patch — the user still sees correct
// Validated / CRDExists / Ready conditions. On failure we return
// StopWithError so controller-runtime requeues; the next reconcile retries
// the label patch idempotently.
func (r *Reconciler) stepEnsureLabels(ctx context.Context, logger logr.Logger, karta *kartav1alpha1.Karta) StepResult {
	gvk := rootGVK(karta)
	if gvk == nil {
		return Continue()
	}

	desired := map[string]string{
		kartav1alpha1.LabelRootGroup:   gvk.Group,
		kartav1alpha1.LabelRootVersion: gvk.Version,
		kartav1alpha1.LabelRootKind:    gvk.Kind,
	}

	current := karta.Labels
	if labelsMatch(current, desired) {
		return Continue()
	}

	// Merge our labels into whatever labels already exist on the Karta.
	merged := make(map[string]string, len(current)+len(desired))
	for k, v := range current {
		merged[k] = v
	}
	for k, v := range desired {
		merged[k] = v
	}

	patchBytes, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"labels": merged},
	})
	if err != nil {
		return StopWithError(fmt.Errorf("marshal label patch for karta %q: %w", karta.Name, err))
	}

	if err = r.Patch(ctx, karta, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return StopWithError(fmt.Errorf("patch labels for karta %q: %w", karta.Name, err))
	}

	logger.V(1).Info("Stamped GVK index labels", "group", gvk.Group, "version", gvk.Version, "kind", gvk.Kind)
	return Continue()
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
