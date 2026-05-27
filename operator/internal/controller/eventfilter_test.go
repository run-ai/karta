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

	It("enqueues all Kartas that share the CRD group+kind, regardless of version", func() {
		gvkV1 := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		gvkV2 := schema.GroupVersionKind{Group: "test.run.ai", Version: "v2", Kind: "Foo"}
		other := schema.GroupVersionKind{Group: "other.run.ai", Version: "v1", Kind: "Bar"}

		Expect(k8s.Create(ctx, newKarta("karta-v1", &gvkV1))).To(Succeed())
		Expect(k8s.Create(ctx, newKarta("karta-v2", &gvkV2))).To(Succeed())
		Expect(k8s.Create(ctx, newKarta("karta-other", &other))).To(Succeed())
		Expect(k8s.Create(ctx, newKarta("karta-no-gvk", nil))).To(Succeed())

		// CRD only serves v1 now; karta-v2 still gets enqueued because we match
		// on group+kind only. The reconciler decides whether v2 actually exists.
		crd := newCRD("foos.test.run.ai", "test.run.ai", "Foo", "v1")
		reqs := r.MapCRDToKartaEvent(ctx, crd)

		names := make([]string, 0, len(reqs))
		for _, req := range reqs {
			names = append(names, req.Name)
		}
		Expect(names).To(ConsistOf("karta-v1", "karta-v2"))
	})

	It("enqueues a Karta even when its referenced version was just removed from the CRD", func() {
		// This is the gap the group+kind approach fixes: if the CRD drops v1
		// from its served set, the version-based mapper would miss karta-v1.
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		Expect(k8s.Create(ctx, newKarta("karta-v1", &gvk))).To(Succeed())

		// CRD now only serves v2 - v1 is gone.
		crd := newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, "v2")
		reqs := r.MapCRDToKartaEvent(ctx, crd)

		names := make([]string, 0, len(reqs))
		for _, req := range reqs {
			names = append(names, req.Name)
		}
		Expect(names).To(ConsistOf("karta-v1"))
	})

	It("skips Kartas whose group or kind does not match", func() {
		differentGroup := schema.GroupVersionKind{Group: "other.run.ai", Version: "v1", Kind: "Foo"}
		differentKind := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Bar"}

		Expect(k8s.Create(ctx, newKarta("karta-diff-group", &differentGroup))).To(Succeed())
		Expect(k8s.Create(ctx, newKarta("karta-diff-kind", &differentKind))).To(Succeed())

		crd := newCRD("foos.test.run.ai", "test.run.ai", "Foo", "v1")
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
