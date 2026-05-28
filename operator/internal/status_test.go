// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package internal

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// conditionInputs is kept for use by status_test.go which tests all three
// conditions together through the helper below.
type conditionInputs struct {
	validated metav1.ConditionStatus
	crdExists metav1.ConditionStatus
}

// setConditions writes all three owned conditions at once. Used by tests.
func setConditions(status *kartav1alpha1.KartaStatus, in conditionInputs) {
	setValidated(status, in.validated)
	setCRDExists(status, in.crdExists)
	setReady(status, in.validated, in.crdExists)
}

var _ = Describe("setConditions", func() {
	allFalse := conditionInputs{metav1.ConditionFalse, metav1.ConditionFalse}
	allTrue := conditionInputs{metav1.ConditionTrue, metav1.ConditionTrue}
	validatedOnly := conditionInputs{metav1.ConditionTrue, metav1.ConditionFalse}
	crdOnly := conditionInputs{metav1.ConditionFalse, metav1.ConditionTrue}

	It("derives Ready=True only when both inputs are True", func() {
		for _, tc := range []struct {
			in    conditionInputs
			ready metav1.ConditionStatus
		}{
			{allTrue, metav1.ConditionTrue},
			{validatedOnly, metav1.ConditionFalse},
			{crdOnly, metav1.ConditionFalse},
			{allFalse, metav1.ConditionFalse},
		} {
			status := &kartav1alpha1.KartaStatus{}
			setConditions(status, tc.in)
			Expect(findCondition(status.Conditions, kartav1alpha1.ConditionReady).Status).
				To(Equal(tc.ready))
		}
	})

	It("sets non-empty messages on False conditions and clears them on True", func() {
		status := &kartav1alpha1.KartaStatus{}
		setConditions(status, allFalse)
		Expect(findCondition(status.Conditions, kartav1alpha1.ConditionValidated).Message).NotTo(BeEmpty())
		Expect(findCondition(status.Conditions, kartav1alpha1.ConditionCRDExists).Message).NotTo(BeEmpty())
		Expect(findCondition(status.Conditions, kartav1alpha1.ConditionReady).Message).NotTo(BeEmpty())

		setConditions(status, allTrue)
		Expect(findCondition(status.Conditions, kartav1alpha1.ConditionValidated).Message).To(BeEmpty())
		Expect(findCondition(status.Conditions, kartav1alpha1.ConditionCRDExists).Message).To(BeEmpty())
		Expect(findCondition(status.Conditions, kartav1alpha1.ConditionReady).Message).To(BeEmpty())
	})

	It("preserves LastTransitionTime when status does not change", func() {
		status := &kartav1alpha1.KartaStatus{}
		setConditions(status, allTrue)
		before := transitionTimes(status.Conditions)

		time.Sleep(2 * time.Millisecond)
		setConditions(status, allTrue)
		Expect(transitionTimes(status.Conditions)).To(Equal(before))
	})

	It("bumps LastTransitionTime when status changes", func() {
		status := &kartav1alpha1.KartaStatus{}
		setConditions(status, allFalse)
		beforeTime := findCondition(status.Conditions, kartav1alpha1.ConditionValidated).LastTransitionTime

		time.Sleep(2 * time.Millisecond)
		setConditions(status, allTrue)
		afterTime := findCondition(status.Conditions, kartav1alpha1.ConditionValidated).LastTransitionTime

		Expect(afterTime.After(beforeTime.Time)).To(BeTrue())
	})

	It("leaves foreign conditions untouched", func() {
		status := &kartav1alpha1.KartaStatus{Conditions: []metav1.Condition{
			{Type: "RBACReady", Status: metav1.ConditionTrue, Reason: "EWI"},
		}}
		setConditions(status, allTrue)

		foreign := findCondition(status.Conditions, "RBACReady")
		Expect(foreign.Status).To(Equal(metav1.ConditionTrue))
		Expect(foreign.Reason).To(Equal("EWI"))
	})
})
