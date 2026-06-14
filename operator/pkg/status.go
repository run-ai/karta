// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"fmt"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	"k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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
func setValidated(status *kartav1alpha1.KartaStatus, generation int64, s metav1.ConditionStatus, msg string) {
	if s == metav1.ConditionFalse && msg == "" {
		msg = msgValidationFailed
	}
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(kartav1alpha1.ConditionValidated),
		Status:             s,
		Reason:             reasonForBool(s, ReasonValidationSucceeded, ReasonValidationFailed),
		Message:            msgWhenFalse(s, msg),
		ObservedGeneration: generation,
	})
}

// setCRDExists writes the CRDExists condition. msg is placed in the condition
// message when s is False; pass an empty string to use the default message.
func setCRDExists(status *kartav1alpha1.KartaStatus, generation int64, s metav1.ConditionStatus, msg string) {
	if s == metav1.ConditionFalse && msg == "" {
		msg = msgCRDNotFound
	}
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(kartav1alpha1.ConditionCRDExists),
		Status:             s,
		Reason:             reasonForBool(s, ReasonCRDFound, ReasonCRDNotFound),
		Message:            msgWhenFalse(s, msg),
		ObservedGeneration: generation,
	})
}

func setReady(status *kartav1alpha1.KartaStatus, generation int64, validated, crdExists metav1.ConditionStatus) {
	readyStatus := metav1.ConditionFalse
	readyReason := ReasonNotReady
	readyMsg := msgNotReady
	if validated == metav1.ConditionTrue && crdExists == metav1.ConditionTrue {
		readyStatus = metav1.ConditionTrue
		readyReason = ReasonReady
		readyMsg = ""
	}
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(kartav1alpha1.ConditionReady),
		Status:             readyStatus,
		Reason:             readyReason,
		Message:            readyMsg,
		ObservedGeneration: generation,
	})
}

// patchStatusIfChanged issues a JSON merge patch on the Karta status
// subresource only when the in-memory status differs from the cluster-side
// snapshot.
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
