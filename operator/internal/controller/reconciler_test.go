// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Reconciler — lifecycle", func() {
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

	reconcile := func(name string) (ctrl.Result, error) {
		return r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: name}})
	}

	get := func(name string) *kartav1alpha1.Karta {
		out := &kartav1alpha1.Karta{}
		ExpectWithOffset(1, k8s.Get(ctx, client.ObjectKey{Name: name}, out)).To(Succeed())
		return out
	}

	It("returns no error when Karta does not exist", func() {
		_, err := reconcile("missing")
		Expect(err).NotTo(HaveOccurred())
	})

	It("skips reconciliation and leaves status empty while Karta is being deleted", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		k := newValidKarta("karta-deleting", &gvk)
		k.Finalizers = []string{"keep.test/finalizer"}
		Expect(k8s.Create(ctx, k)).To(Succeed())
		Expect(k8s.Delete(ctx, k)).To(Succeed())

		_, err := reconcile(k.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(get(k.Name).Status.Conditions).To(BeEmpty())
	})
})

var _ = Describe("Reconciler — condition logic", func() {
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

	reconcileAndGet := func(name string) *kartav1alpha1.Karta {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: name}})
		Expect(err).NotTo(HaveOccurred())
		out := &kartav1alpha1.Karta{}
		Expect(k8s.Get(ctx, client.ObjectKey{Name: name}, out)).To(Succeed())
		return out
	}

	Context("Story 1.2 — KartaValidated", func() {
		It("sets KartaValidated=False when root component has no StatusDefinition", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newKarta("karta-invalid", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-invalid")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionKartaValidated).Status).
				To(Equal(metav1.ConditionFalse))
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionKartaValidated).Reason).
				To(Equal(ReasonKartaValidationFailed))
		})

		It("sets KartaValidated=True when spec is valid", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newValidKarta("karta-valid", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-valid")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionKartaValidated).Status).
				To(Equal(metav1.ConditionTrue))
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionKartaValidated).Reason).
				To(Equal(ReasonKartaValidationSucceeded))
		})
	})

	Context("Story 1.3 — CRDExists", func() {
		It("sets CRDExists=False when Karta has no root kind", func() {
			Expect(k8s.Create(ctx, newKarta("karta-no-gvk", nil))).To(Succeed())

			got := reconcileAndGet("karta-no-gvk")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Status).
				To(Equal(metav1.ConditionFalse))
		})

		It("sets CRDExists=False when referenced CRD does not exist", func() {
			gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
			Expect(k8s.Create(ctx, newValidKarta("karta-no-crd", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-no-crd")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Status).
				To(Equal(metav1.ConditionFalse))
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Reason).
				To(Equal(ReasonCRDNotFound))
		})

		It("sets CRDExists=False when CRD exists but does not list the referenced version", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, "v2"))).To(Succeed())
			Expect(k8s.Create(ctx, newValidKarta("karta-wrong-ver", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-wrong-ver")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Status).
				To(Equal(metav1.ConditionFalse))
		})

		It("sets CRDExists=True when the CRD lists the referenced version", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newValidKarta("karta-crd-ok", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-crd-ok")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Status).
				To(Equal(metav1.ConditionTrue))
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Reason).
				To(Equal(ReasonCRDFound))
		})

		It("sets CRDExists=True even when the version is not the storage version", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			// v2 is storage, v1 is not — but Karta references v1
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, "v2", gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newValidKarta("karta-non-storage-ver", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-non-storage-ver")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Status).
				To(Equal(metav1.ConditionTrue))
		})
	})

	Context("Story 1.4 — Ready (derived)", func() {
		It("sets Ready=False when KartaValidated=False regardless of CRDExists", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newKarta("karta-invalid-crd-ok", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-invalid-crd-ok")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionReady).Status).
				To(Equal(metav1.ConditionFalse))
		})

		It("sets Ready=False when CRDExists=False regardless of KartaValidated", func() {
			gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
			Expect(k8s.Create(ctx, newValidKarta("karta-valid-no-crd", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-valid-no-crd")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionReady).Status).
				To(Equal(metav1.ConditionFalse))
		})

		It("sets Ready=True when both KartaValidated and CRDExists are True", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newValidKarta("karta-all-good", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-all-good")
			ready := findCondition(got.Status.Conditions, kartav1alpha1.ConditionReady)
			Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			Expect(ready.Reason).To(Equal(ReasonReady))
			Expect(ready.Message).To(BeEmpty())
		})
	})

	Context("Idempotency", func() {
		It("does not change LastTransitionTime on a second reconcile when nothing changed", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newValidKarta("karta-stable", &gvk))).To(Succeed())

			first := reconcileAndGet("karta-stable")
			firstTimes := transitionTimes(first.Status.Conditions)

			second := reconcileAndGet("karta-stable")
			secondTimes := transitionTimes(second.Status.Conditions)

			Expect(secondTimes).To(Equal(firstTimes))
		})
	})

	Context("Foreign conditions", func() {
		It("preserves conditions set by other controllers across reconciles", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())

			k := newValidKarta("karta-foreign", &gvk)
			k.Status.Conditions = []metav1.Condition{
				{Type: "RBACReady", Status: metav1.ConditionTrue, Reason: "EWI", LastTransitionTime: metav1.Now()},
			}
			Expect(k8s.Create(ctx, k)).To(Succeed())

			got := reconcileAndGet("karta-foreign")
			rbac := findCondition(got.Status.Conditions, "RBACReady")
			Expect(rbac.Status).To(Equal(metav1.ConditionTrue))
			Expect(rbac.Reason).To(Equal("EWI"))
		})
	})
})

func transitionTimes(conds []metav1.Condition) map[string]metav1.Time {
	out := map[string]metav1.Time{}
	for _, c := range conds {
		out[c.Type] = c.LastTransitionTime
	}
	return out
}
