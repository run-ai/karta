// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// labeledKarta returns a Karta pre-stamped with the GVK index labels, as if
// ensureLabels had already run. Used to set up the label-selector tests.
func labeledKarta(name string, gvk schema.GroupVersionKind) *kartav1alpha1.Karta {
	k := newKarta(name, &gvk)
	k.Labels = kartaLabels(gvk)
	return k
}

var _ = Describe("Reconciler.MapCRDToKartaEvent", func() {
	var (
		ctx context.Context
		k8s client.WithWatch
		r   *Reconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		k8s = fake.NewClientBuilder().WithScheme(buildScheme()).Build()
		r = newReconciler(k8s)
	})

	It("returns nil when the object is not a CRD", func() {
		Expect(r.MapCRDToKartaEvent(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p"}})).To(BeNil())
	})

	It("returns no requests when there are no Kartas", func() {
		crd := newCRD("fooTestRunai", "test.run.ai", "Foo", "v1")
		Expect(r.MapCRDToKartaEvent(ctx, crd)).To(BeEmpty())
	})

	It("returns no requests when Kartas exist but have no index labels (not yet reconciled)", func() {
		// The mapper uses label-selector; a freshly created Karta without labels
		// is not found here. Its own create event triggers the first reconcile
		// which stamps the labels for future CRD events.
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		Expect(k8s.Create(ctx, newKarta("karta-unlabeled", &gvk))).To(Succeed())

		crd := newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version)
		Expect(r.MapCRDToKartaEvent(ctx, crd)).To(BeEmpty())
	})

	It("enqueues all Kartas sharing the CRD group+kind, regardless of version", func() {
		gvkV1 := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		gvkV2 := schema.GroupVersionKind{Group: "test.run.ai", Version: "v2", Kind: "Foo"}
		other := schema.GroupVersionKind{Group: "other.run.ai", Version: "v1", Kind: "Bar"}

		Expect(k8s.Create(ctx, labeledKarta("karta-v1", gvkV1))).To(Succeed())
		Expect(k8s.Create(ctx, labeledKarta("karta-v2", gvkV2))).To(Succeed())
		Expect(k8s.Create(ctx, labeledKarta("karta-other", other))).To(Succeed())
		Expect(k8s.Create(ctx, newKarta("karta-no-gvk", nil))).To(Succeed())

		// CRD only serves v1, but karta-v2 still gets enqueued because the
		// label-selector matches on group+kind (not version).
		crd := newCRD("foos.test.run.ai", "test.run.ai", "Foo", "v1")
		reqs := r.MapCRDToKartaEvent(ctx, crd)

		names := make([]string, 0, len(reqs))
		for _, req := range reqs {
			names = append(names, req.Name)
		}
		Expect(names).To(ConsistOf("karta-v1", "karta-v2"))
	})

	It("enqueues a Karta even when its referenced version was just removed from the CRD", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		Expect(k8s.Create(ctx, labeledKarta("karta-v1", gvk))).To(Succeed())

		// enqueued so the reconciler can set CRDExists=False.
		crd := newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, "v2")
		reqs := r.MapCRDToKartaEvent(ctx, crd)

		names := make([]string, 0, len(reqs))
		for _, req := range reqs {
			names = append(names, req.Name)
		}
		Expect(names).To(ConsistOf("karta-v1"))
	})

	It("skips Kartas whose group or kind label does not match", func() {
		differentGroup := schema.GroupVersionKind{Group: "other.run.ai", Version: "v1", Kind: "Foo"}
		differentKind := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Bar"}

		Expect(k8s.Create(ctx, labeledKarta("karta-diff-group", differentGroup))).To(Succeed())
		Expect(k8s.Create(ctx, labeledKarta("karta-diff-kind", differentKind))).To(Succeed())

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

var _ = Describe("kartaLabels helper", func() {
	It("produces the expected three-label map for a GVK", func() {
		gvk := schema.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayCluster"}
		labels := kartaLabels(gvk)
		Expect(labels[kartav1alpha1.LabelRootGroup]).To(Equal("ray.io"))
		Expect(labels[kartav1alpha1.LabelRootVersion]).To(Equal("v1"))
		Expect(labels[kartav1alpha1.LabelRootKind]).To(Equal("RayCluster"))
	})
})
