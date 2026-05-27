// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Reconciler.MapCRDToKartaEvent", func() {
	var (
		ctx context.Context
		k8s client.WithWatch
		r   *Reconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		k8s = fake.NewClientBuilder().WithScheme(buildScheme()).Build()
		r = NewReconciler(k8s)
	})

	It("returns nil when the object is not a CRD", func() {
		Expect(r.MapCRDToKartaEvent(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p"}})).To(BeNil())
	})

	It("returns no requests when there are no Kartas", func() {
		crd := newCRD("foos.test.run.ai", "test.run.ai", "Foo", "v1")
		Expect(r.MapCRDToKartaEvent(ctx, crd)).To(BeEmpty())
	})

	It("returns one request per Karta whose root GVK matches a served CRD version", func() {
		gvkV1 := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		gvkV2 := schema.GroupVersionKind{Group: "test.run.ai", Version: "v2", Kind: "Foo"}
		other := schema.GroupVersionKind{Group: "other.run.ai", Version: "v1", Kind: "Bar"}

		Expect(k8s.Create(ctx, newKarta("karta-v1", &gvkV1))).To(Succeed())
		Expect(k8s.Create(ctx, newKarta("karta-v2", &gvkV2))).To(Succeed())
		Expect(k8s.Create(ctx, newKarta("karta-other", &other))).To(Succeed())
		Expect(k8s.Create(ctx, newKarta("karta-no-gvk", nil))).To(Succeed())

		crd := newCRD("foos.test.run.ai", "test.run.ai", "Foo", "v1", "v2")
		reqs := r.MapCRDToKartaEvent(ctx, crd)

		names := make([]string, 0, len(reqs))
		for _, req := range reqs {
			names = append(names, req.Name)
		}
		Expect(names).To(ConsistOf("karta-v1", "karta-v2"))
	})

	It("ignores CRD versions that are not served", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		Expect(k8s.Create(ctx, newKarta("karta-v1", &gvk))).To(Succeed())

		crd := newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version)
		crd.Spec.Versions[0].Served = false
		Expect(r.MapCRDToKartaEvent(ctx, crd)).To(BeEmpty())
	})
})

var _ = Describe("rootGVK helper", func() {
	It("returns nil when no root component kind is set", func() {
		Expect(rootGVK(newKarta("k", nil))).To(BeNil())
	})

	It("converts the embedded GroupVersionKind", func() {
		gvk := schema.GroupVersionKind{Group: "g", Version: "v", Kind: "K"}
		got := rootGVK(newKarta("k", &gvk))
		Expect(got).NotTo(BeNil())
		Expect(*got).To(Equal(gvk))
	})
})
