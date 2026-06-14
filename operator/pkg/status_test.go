// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
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
// Generation 0 is used since test objects don't have a real generation.
func setConditions(status *kartav1alpha1.KartaStatus, in conditionInputs) {
	setValidated(status, 0, in.validated, "")
	setCRDExists(status, 0, in.crdExists, "")
	setReady(status, 0, in.validated, in.crdExists)
}

var _ = Describe("setConditions", func() {
	It("derives Ready=True only when both Validated and CRDExists are True", func() {
		for _, tc := range []struct {
			in    conditionInputs
			ready metav1.ConditionStatus
		}{
			{conditionInputs{metav1.ConditionTrue, metav1.ConditionTrue}, metav1.ConditionTrue},
			{conditionInputs{metav1.ConditionTrue, metav1.ConditionFalse}, metav1.ConditionFalse},
			{conditionInputs{metav1.ConditionFalse, metav1.ConditionTrue}, metav1.ConditionFalse},
			{conditionInputs{metav1.ConditionFalse, metav1.ConditionFalse}, metav1.ConditionFalse},
		} {
			status := &kartav1alpha1.KartaStatus{}
			setConditions(status, tc.in)
			Expect(findCondition(status.Conditions, kartav1alpha1.ConditionReady).Status).
				To(Equal(tc.ready))
		}
	})
})
