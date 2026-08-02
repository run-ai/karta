// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func kartaWithRootKind(gvk *kartav1alpha1.GroupVersionKind) *kartav1alpha1.Karta {
	return &kartav1alpha1.Karta{
		Spec: kartav1alpha1.KartaSpec{
			StructureDefinition: kartav1alpha1.StructureDefinition{
				RootComponent: kartav1alpha1.ComponentDefinition{
					Name: "root",
					Kind: gvk,
				},
			},
		},
	}
}

// namedKarta builds a spec-valid Karta with a name and root GVK.
func namedKarta(name string, gvk *kartav1alpha1.GroupVersionKind) *kartav1alpha1.Karta {
	k := kartaWithRootKind(gvk)
	k.Name = name
	k.Spec.StructureDefinition.RootComponent.StatusDefinition = &kartav1alpha1.StatusDefinition{}
	return k
}

// fakeReader returns a client.Reader seeded with the given Kartas.
func fakeReader(objs ...client.Object) client.Reader {
	s := runtime.NewScheme()
	utilruntime.Must(kartav1alpha1.AddToScheme(s))
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
}

var _ = Describe("KartaLabeler.Default", func() {
	var labeler *KartaLabeler
	ctx := context.Background()

	BeforeEach(func() {
		labeler = &KartaLabeler{}
	})

	It("stamps the GVK index labels for a Karta with a root component kind", func() {
		karta := kartaWithRootKind(&kartav1alpha1.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayCluster"})
		Expect(labeler.Default(ctx, karta)).To(Succeed())
		Expect(karta.Labels).To(HaveKeyWithValue(kartav1alpha1.LabelRootGroup, "ray.io"))
		Expect(karta.Labels).To(HaveKeyWithValue(kartav1alpha1.LabelRootVersion, "v1"))
		Expect(karta.Labels).To(HaveKeyWithValue(kartav1alpha1.LabelRootKind, "RayCluster"))
	})

	It("is a no-op when the Karta has no root component kind", func() {
		karta := &kartav1alpha1.Karta{}
		Expect(labeler.Default(ctx, karta)).To(Succeed())
		Expect(karta.Labels).To(BeEmpty())
	})

	It("preserves existing unrelated labels", func() {
		karta := kartaWithRootKind(&kartav1alpha1.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayCluster"})
		karta.ObjectMeta = metav1.ObjectMeta{Labels: map[string]string{"team": "ml"}}
		Expect(labeler.Default(ctx, karta)).To(Succeed())
		Expect(karta.Labels).To(HaveKeyWithValue("team", "ml"))
		Expect(karta.Labels).To(HaveKeyWithValue(kartav1alpha1.LabelRootKind, "RayCluster"))
	})

	It("produces the same labels the reconciler would (shared helper)", func() {
		karta := kartaWithRootKind(&kartav1alpha1.GroupVersionKind{Group: "kubeflow.org", Version: "v1", Kind: "PyTorchJob"})
		Expect(labeler.Default(ctx, karta)).To(Succeed())
		Expect(karta.Labels).To(Equal(desiredRootLabels(karta)))
	})
})

var _ = Describe("KartaValidator", func() {
	var validator *KartaValidator
	ctx := context.Background()

	rayGVK := &kartav1alpha1.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayCluster"}
	pytorchGVK := &kartav1alpha1.GroupVersionKind{Group: "kubeflow.org", Version: "v1", Kind: "PyTorchJob"}

	BeforeEach(func() {
		validator = &KartaValidator{reader: fakeReader()}
	})

	validKarta := func() *kartav1alpha1.Karta { return namedKarta("valid", rayGVK) }

	It("accepts a valid Karta on create", func() {
		_, err := validator.ValidateCreate(ctx, validKarta())
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects a Karta with no root component kind on create", func() {
		_, err := validator.ValidateCreate(ctx, kartaWithRootKind(nil))
		Expect(err).To(HaveOccurred())
	})

	It("rejects an invalid Karta on update", func() {
		_, err := validator.ValidateUpdate(ctx, validKarta(), kartaWithRootKind(nil))
		Expect(err).To(HaveOccurred())
	})

	It("allows delete", func() {
		_, err := validator.ValidateDelete(ctx, kartaWithRootKind(nil))
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects a Karta whose root GVK already belongs to another Karta", func() {
		validator = &KartaValidator{reader: fakeReader(namedKarta("first", rayGVK))}
		_, err := validator.ValidateCreate(ctx, namedKarta("second", rayGVK))
		Expect(err).To(MatchError(ContainSubstring(`a Karta already exists for GVK ray.io/v1 RayCluster: "first"`)))
	})

	It("accepts a Karta whose root GVK is not used by any other Karta", func() {
		validator = &KartaValidator{reader: fakeReader(namedKarta("first", rayGVK))}
		_, err := validator.ValidateCreate(ctx, namedKarta("second", pytorchGVK))
		Expect(err).NotTo(HaveOccurred())
	})

	It("allows updating a Karta without treating itself as a conflict", func() {
		existing := namedKarta("first", rayGVK)
		validator = &KartaValidator{reader: fakeReader(existing)}
		_, err := validator.ValidateUpdate(ctx, existing, namedKarta("first", rayGVK))
		Expect(err).NotTo(HaveOccurred())
	})
})
