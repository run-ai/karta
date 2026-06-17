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
	setReady(status, 0)
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

// findCondition returns the condition with the given type, failing the test
// when not found.
func findCondition(conds []metav1.Condition, t kartav1alpha1.ConditionType) metav1.Condition {
	GinkgoHelper()
	c, found := findConditionOpt(conds, t)
	if !found {
		Fail("condition not found: " + string(t))
	}
	return c
}

// findConditionOpt returns the condition with the given type and whether it was
// found, without failing the test.
func findConditionOpt(conds []metav1.Condition, t kartav1alpha1.ConditionType) (metav1.Condition, bool) {
	for _, c := range conds {
		if c.Type == string(t) {
			return c, true
		}
	}
	return metav1.Condition{}, false
}
