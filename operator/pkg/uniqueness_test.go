// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func reconcilerWith(objs ...client.Object) *Reconciler {
	s := runtime.NewScheme()
	utilruntime.Must(kartav1alpha1.AddToScheme(s))
	return &Reconciler{Client: fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()}
}

var _ = Describe("Reconciler GVK uniqueness backstop", func() {
	ctx := context.Background()
	ray := &kartav1alpha1.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayCluster"}
	pytorch := &kartav1alpha1.GroupVersionKind{Group: "kubeflow.org", Version: "v1", Kind: "PyTorchJob"}

	Describe("gvkOwnerConflict", func() {
		It("returns empty when no other Karta claims the GVK", func() {
			a := namedKarta("a", ray)
			Expect(reconcilerWith(a).gvkOwnerConflict(ctx, a)).To(BeEmpty())
		})

		It("names the owner (name tiebreak) and clears the owner itself", func() {
			a, b := namedKarta("a-first", ray), namedKarta("b-second", ray)
			r := reconcilerWith(a, b)
			Expect(r.gvkOwnerConflict(ctx, b)).To(Equal("a-first"))
			Expect(r.gvkOwnerConflict(ctx, a)).To(BeEmpty())
		})

		It("ignores Kartas that target a different GVK", func() {
			a, b := namedKarta("a", ray), namedKarta("b", pytorch)
			Expect(reconcilerWith(a, b).gvkOwnerConflict(ctx, b)).To(BeEmpty())
		})

		It("is a no-op for a Karta with no root component kind", func() {
			Expect(reconcilerWith().gvkOwnerConflict(ctx, kartaWithRootKind(nil))).To(BeEmpty())
		})
	})

	Describe("MapKartaToSiblings", func() {
		It("enqueues same-GVK siblings, excluding the changed Karta and other GVKs", func() {
			a, b, c := namedKarta("a", ray), namedKarta("b", ray), namedKarta("c", pytorch)
			reqs := reconcilerWith(a, b, c).MapKartaToSiblings(ctx, a)
			Expect(reqs).To(ConsistOf(reconcile.Request{NamespacedName: client.ObjectKey{Name: "b"}}))
		})
	})
})
