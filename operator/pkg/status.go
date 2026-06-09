// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"fmt"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ReasonValidationSucceeded = "ValidationSucceeded"
	ReasonValidationFailed    = "ValidationFailed"
	ReasonCRDFound            = "CRDFound"
	ReasonCRDNotFound         = "CRDNotFound"
	ReasonReady               = "Ready"
	ReasonNotReady            = "NotReady"
)

const (
	msgValidationFailed = "Karta validation failed"
	msgCRDNotFound      = "CustomResourceDefinition for the root component GVK does not exist in the cluster or does not serve the referenced version"
	msgNotReady         = "Validated and CRDExists must both be True"
)

// setValidated writes the Validated condition. msg is placed in the condition
// message when s is False; pass an empty string to use the default message.
func setValidated(status *kartav1alpha1.KartaStatus, s metav1.ConditionStatus, msg string) {
	if s == metav1.ConditionFalse && msg == "" {
		msg = msgValidationFailed
	}
	upsertConditions(&status.Conditions, map[kartav1alpha1.ConditionType]metav1.Condition{
		kartav1alpha1.ConditionValidated: buildCondition(
			kartav1alpha1.ConditionValidated,
			s,
			reasonForBool(s, ReasonValidationSucceeded, ReasonValidationFailed),
			msgWhenFalse(s, msg),
		),
	})
}

// setCRDExists writes the CRDExists condition. msg is placed in the condition
// message when s is False; pass an empty string to use the default message.
func setCRDExists(status *kartav1alpha1.KartaStatus, s metav1.ConditionStatus, msg string) {
	if s == metav1.ConditionFalse && msg == "" {
		msg = msgCRDNotFound
	}
	upsertConditions(&status.Conditions, map[kartav1alpha1.ConditionType]metav1.Condition{
		kartav1alpha1.ConditionCRDExists: buildCondition(
			kartav1alpha1.ConditionCRDExists,
			s,
			reasonForBool(s, ReasonCRDFound, ReasonCRDNotFound),
			msgWhenFalse(s, msg),
		),
	})
}

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

// patchStatusIfChanged issues a JSON merge patch on the Karta status
// subresource only when the in-memory status differs from the cluster-side
// snapshot. The patch body is a delta computed by client.MergeFrom against
// base, so fields we did not mutate are not shipped at all in the steady-state case.
func (r *Reconciler) patchStatusIfChanged(
	ctx context.Context,
	karta *kartav1alpha1.Karta,
	base *kartav1alpha1.Karta,
) error {
	if equality.Semantic.DeepEqual(base.Status, karta.Status) {
		return nil
	}
	if err := r.Status().Patch(ctx, karta, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}

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
