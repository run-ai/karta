// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Q4: verify GenerationChangedPredicate semantics on CRDs.
// Create and Delete must pass through; Update only when generation changes.
var _ = Describe("CRD watch predicate (Q4)", func() {
	pred := predicate.GenerationChangedPredicate{}

	crd := func(gen int64) *apiextensionsv1.CustomResourceDefinition {
		return &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "foos.test.run.ai", Generation: gen},
		}
	}

	It("passes Create events", func() {
		Expect(pred.Create(event.CreateEvent{Object: crd(1)})).To(BeTrue())
	})

	It("passes Delete events", func() {
		Expect(pred.Delete(event.DeleteEvent{Object: crd(1)})).To(BeTrue())
	})

	It("passes Update events when generation changes (spec changed)", func() {
		Expect(pred.Update(event.UpdateEvent{ObjectOld: crd(1), ObjectNew: crd(2)})).To(BeTrue())
	})

	It("drops Update events when generation is unchanged (status-only update)", func() {
		Expect(pred.Update(event.UpdateEvent{ObjectOld: crd(3), ObjectNew: crd(3)})).To(BeFalse())
	})
})
