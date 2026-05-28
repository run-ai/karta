// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package internal

import (
	"context"
	"encoding/json"
	"fmt"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reason constants reported on Karta status conditions. They are
// machine-readable and may be matched by consumers (EWI, dashboards, etc.).
const (
	ReasonKartaValidationSucceeded = "KartaValidationSucceeded"
	ReasonKartaValidationFailed    = "KartaValidationFailed"
	ReasonCRDFound                 = "CRDFound"
	ReasonCRDNotFound              = "CRDNotFound"
	ReasonReady                    = "Ready"
	ReasonNotReady                 = "NotReady"
)

// condition message strings shown when a condition is False.
const (
	msgKartaValidationFailed = "Karta spec validation failed"
	msgCRDNotFound           = "CustomResourceDefinition for the root component GVK does not exist in the cluster or does not serve the referenced version"
	msgNotReady              = "KartaValidated and CRDExists must both be True"
)

// setKartaValidated writes the KartaValidated condition into status.
func setKartaValidated(status *kartav1alpha1.KartaStatus, s metav1.ConditionStatus) {
	upsertConditions(&status.Conditions, map[kartav1alpha1.ConditionType]metav1.Condition{
		kartav1alpha1.ConditionKartaValidated: buildCondition(
			kartav1alpha1.ConditionKartaValidated,
			s,
			reasonForBool(s, ReasonKartaValidationSucceeded, ReasonKartaValidationFailed),
			msgWhenFalse(s, msgKartaValidationFailed),
		),
	})
}

// setCRDExists writes the CRDExists condition into status.
func setCRDExists(status *kartav1alpha1.KartaStatus, s metav1.ConditionStatus) {
	upsertConditions(&status.Conditions, map[kartav1alpha1.ConditionType]metav1.Condition{
		kartav1alpha1.ConditionCRDExists: buildCondition(
			kartav1alpha1.ConditionCRDExists,
			s,
			reasonForBool(s, ReasonCRDFound, ReasonCRDNotFound),
			msgWhenFalse(s, msgCRDNotFound),
		),
	})
}

// setReady derives and writes the Ready condition from the current values of
// KartaValidated and CRDExists already in status.
func setReady(status *kartav1alpha1.KartaStatus, validated, crdExists metav1.ConditionStatus) {
	readyStatus := metav1.ConditionFalse
	readyReason := ReasonNotReady
	readyMsg := msgNotReady
	if validated == metav1.ConditionTrue && crdExists == metav1.ConditionTrue {
		readyStatus = metav1.ConditionTrue
		readyReason = ReasonReady
		readyMsg = ""
	}
	upsertConditions(&status.Conditions, map[kartav1alpha1.ConditionType]metav1.Condition{
		kartav1alpha1.ConditionReady: buildCondition(
			kartav1alpha1.ConditionReady, readyStatus, readyReason, readyMsg,
		),
	})
}

// conditionInputs is kept for use by status_test.go which tests all three
// conditions together through the helper below.
type conditionInputs struct {
	kartaValidated metav1.ConditionStatus
	crdExists      metav1.ConditionStatus
}

// setConditions writes all three owned conditions at once. Used by tests.
func setConditions(status *kartav1alpha1.KartaStatus, in conditionInputs) {
	setKartaValidated(status, in.kartaValidated)
	setCRDExists(status, in.crdExists)
	setReady(status, in.kartaValidated, in.crdExists)
}

// upsertConditions merges desired into the existing condition list.
//
// Rules:
//   - Existing conditions owned by this operator are updated in-place,
//     preserving list order.
//   - LastTransitionTime is bumped only when Status actually changes.
//   - Conditions present in the existing list but absent from desired are left
//     completely untouched — this is how foreign conditions owned by other
//     controllers (e.g. EWI's RBACReady) survive across our reconciles.
//   - New desired conditions not yet in the list are appended.
func upsertConditions(current *[]metav1.Condition, desired map[kartav1alpha1.ConditionType]metav1.Condition) {
	if current == nil {
		return
	}

	now := metav1.Now()
	out := make([]metav1.Condition, 0, len(*current)+len(desired))

	for _, existing := range *current {
		incoming, owned := desired[kartav1alpha1.ConditionType(existing.Type)]
		if !owned {
			out = append(out, existing)
			continue
		}
		if incoming.Status != existing.Status {
			incoming.LastTransitionTime = now
		} else {
			incoming.LastTransitionTime = existing.LastTransitionTime
		}
		out = append(out, incoming)
		delete(desired, kartav1alpha1.ConditionType(existing.Type))
	}

	// append any desired conditions that were not already in the list
	for _, incoming := range desired {
		incoming.LastTransitionTime = now
		out = append(out, incoming)
	}

	*current = out
}

// patchStatusIfChanged issues a merge-patch on the Karta status subresource
// only when something meaningful changed compared to the snapshot taken before
// reconciliation began.
func (r *Reconciler) patchStatusIfChanged(
	ctx context.Context,
	karta *kartav1alpha1.Karta,
	original *kartav1alpha1.KartaStatus,
) error {
	if !statusChanged(original, &karta.Status) {
		return nil
	}

	patchBytes, err := json.Marshal(map[string]any{
		"status": map[string]any{"conditions": karta.Status.Conditions},
	})
	if err != nil {
		return fmt.Errorf("marshal status patch: %w", err)
	}

	if err := r.Status().Patch(ctx, karta, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}

// statusChanged reports whether conditions differ in any meaningful field
// (Type, Status, Reason, Message). LastTransitionTime is intentionally ignored
// so we don't patch on every reconcile when nothing actually changed.
func statusChanged(original, current *kartav1alpha1.KartaStatus) bool {
	if len(original.Conditions) != len(current.Conditions) {
		return true
	}
	for i := range original.Conditions {
		o, c := original.Conditions[i], current.Conditions[i]
		if o.Type != c.Type || o.Status != c.Status || o.Reason != c.Reason || o.Message != c.Message {
			return true
		}
	}
	return false
}

// buildCondition assembles a metav1.Condition.
func buildCondition(t kartav1alpha1.ConditionType, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    string(t),
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

// reasonForBool returns trueReason when status is True, falseReason otherwise.
func reasonForBool(status metav1.ConditionStatus, trueReason, falseReason string) string {
	if status == metav1.ConditionTrue {
		return trueReason
	}
	return falseReason
}

// msgWhenFalse returns the fallback message when status is False, empty otherwise.
func msgWhenFalse(status metav1.ConditionStatus, fallback string) string {
	if status == metav1.ConditionTrue {
		return ""
	}
	return fallback
}
