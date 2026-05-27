// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
