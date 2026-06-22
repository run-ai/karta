// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type conditionInputs struct {
	validated metav1.ConditionStatus
	crdExists metav1.ConditionStatus
}

var _ = Describe("status.go test", func() {
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

func setConditions(status *kartav1alpha1.KartaStatus, in conditionInputs) {
	setValidated(status, 0, in.validated, "")
	setCRDExists(status, 0, in.crdExists, "")
	setReady(status, 0)
}

func findCondition(conds []metav1.Condition, t kartav1alpha1.ConditionType) metav1.Condition {
	GinkgoHelper()
	c := apimeta.FindStatusCondition(conds, string(t))
	if c == nil {
		Fail("condition not found: " + string(t))
	}
	return *c
}
