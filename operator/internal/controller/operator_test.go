// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ = Describe("Reconciler.Reconcile (lifecycle)", func() {
	var (
		ctx context.Context
		k8s client.WithWatch
		r   *Reconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		k8s = fake.NewClientBuilder().
			WithScheme(buildScheme()).
			WithStatusSubresource(&kartav1alpha1.Karta{}).
			Build()
		r = NewReconciler(k8s)
	})

	reconcileByName := func(name string) (ctrl.Result, error) {
		return r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: name}})
	}

	It("returns no error when the Karta does not exist", func() {
		res, err := reconcileByName("missing")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("skips reconciliation when the Karta is being deleted", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		k := newKarta("karta-deleting", &gvk)
		// Finalizer required so the fake client retains the object after Delete.
		k.Finalizers = []string{"keep.test/finalizer"}
		Expect(k8s.Create(ctx, k)).To(Succeed())
		Expect(k8s.Delete(ctx, k)).To(Succeed())

		res, err := reconcileByName(k.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})

	It("reconciles an existing Karta without error", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		Expect(k8s.Create(ctx, newKarta("karta-existing", &gvk))).To(Succeed())

		res, err := reconcileByName("karta-existing")
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())
	})
})

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
