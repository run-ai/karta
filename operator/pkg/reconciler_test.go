// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"errors"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// drainEvents returns all events currently in the recorder buffer without blocking.
func drainEvents(rec *record.FakeRecorder) []string {
	var out []string
	for {
		select {
		case e := <-rec.Events:
			out = append(out, e)
		default:
			return out
		}
	}
}

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
		r = newReconciler(k8s)
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
		r = newReconciler(k8s)
	})

	reconcileAndGet := func(name string) *kartav1alpha1.Karta {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: name}})
		Expect(err).NotTo(HaveOccurred())
		out := &kartav1alpha1.Karta{}
		Expect(k8s.Get(ctx, client.ObjectKey{Name: name}, out)).To(Succeed())
		return out
	}

	Context("Story 1.2 — Validated", func() {
		It("sets Validated=False when root component has no StatusDefinition", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newKarta("karta-invalid", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-invalid")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionValidated).Status).
				To(Equal(metav1.ConditionFalse))
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionValidated).Reason).
				To(Equal(ReasonValidationFailed))
		})

		It("sets Validated=True when spec is valid", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newValidKarta("karta-valid", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-valid")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionValidated).Status).
				To(Equal(metav1.ConditionTrue))
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionValidated).Reason).
				To(Equal(ReasonValidationSucceeded))
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
		It("sets Ready=False when Validated=False regardless of CRDExists", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
			Expect(k8s.Create(ctx, newKarta("karta-invalid-crd-ok", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-invalid-crd-ok")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionReady).Status).
				To(Equal(metav1.ConditionFalse))
		})

		It("sets Ready=False when CRDExists=False regardless of Validated", func() {
			gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
			Expect(k8s.Create(ctx, newValidKarta("karta-valid-no-crd", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-valid-no-crd")
			Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionReady).Status).
				To(Equal(metav1.ConditionFalse))
		})

		It("sets Ready=True when both Validated and CRDExists are True", func() {
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

	Context("Label stamping (ensureLabels)", func() {
		It("stamps the karta/gvk index label after the first reconcile", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			Expect(k8s.Create(ctx, newKarta("karta-no-labels", &gvk))).To(Succeed())

			got := reconcileAndGet("karta-no-labels")
			Expect(got.Labels[kartav1alpha1.LabelGVK]).To(Equal(
				kartav1alpha1.FormatGVKLabel(gvk.Group, gvk.Version, gvk.Kind)))
		})

		It("updates the label when root GVK changes", func() {
			oldGVK := schema.GroupVersionKind{Group: "old.run.ai", Version: "v1", Kind: "Old"}
			k := newKarta("karta-stale-labels", &oldGVK)
			k.Labels = kartaLabels(oldGVK)
			Expect(k8s.Create(ctx, k)).To(Succeed())

			// Simulate someone updating the root GVK in spec
			newGVK := schema.GroupVersionKind{Group: "new.run.ai", Version: "v2", Kind: "New"}
			k.Spec.StructureDefinition.RootComponent.Kind = &kartav1alpha1.GroupVersionKind{
				Group: newGVK.Group, Version: newGVK.Version, Kind: newGVK.Kind,
			}
			Expect(k8s.Update(ctx, k)).To(Succeed())

			got := reconcileAndGet("karta-stale-labels")
			Expect(got.Labels[kartav1alpha1.LabelGVK]).To(Equal(
				kartav1alpha1.FormatGVKLabel(newGVK.Group, newGVK.Version, newGVK.Kind)))
		})

		It("preserves unrelated labels", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			k := newKarta("karta-extra-labels", &gvk)
			k.Labels = map[string]string{"custom/label": "keep-me"}
			Expect(k8s.Create(ctx, k)).To(Succeed())

			got := reconcileAndGet("karta-extra-labels")
			Expect(got.Labels["custom/label"]).To(Equal("keep-me"))
			Expect(got.Labels).To(HaveKey(kartav1alpha1.LabelGVK))
		})

		It("does not patch labels when they are already correct", func() {
			gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
			k := newKarta("karta-correct-labels", &gvk)
			k.Labels = kartaLabels(gvk)
			Expect(k8s.Create(ctx, k)).To(Succeed())

			// reconcile twice and check the resource version doesn't change
			// (no patch was issued the second time)
			first := reconcileAndGet("karta-correct-labels")
			second := reconcileAndGet("karta-correct-labels")
			Expect(second.ResourceVersion).To(Equal(first.ResourceVersion))
		})

		It("skips label stamping when Karta has no root kind", func() {
			Expect(k8s.Create(ctx, newKarta("karta-no-kind", nil))).To(Succeed())
			got := reconcileAndGet("karta-no-kind")
			Expect(got.Labels).NotTo(HaveKey(kartav1alpha1.LabelGVK))
		})

		It("removes the karta/gvk label when root kind is removed from spec", func() {
			oldGVK := schema.GroupVersionKind{Group: "old.run.ai", Version: "v1", Kind: "Old"}
			k := newKarta("karta-lost-kind", &oldGVK)
			k.Labels = kartaLabels(oldGVK)
			k.Labels["custom/label"] = "keep-me"
			Expect(k8s.Create(ctx, k)).To(Succeed())

			k.Spec.StructureDefinition.RootComponent.Kind = nil
			Expect(k8s.Update(ctx, k)).To(Succeed())

			got := reconcileAndGet("karta-lost-kind")
			Expect(got.Labels).NotTo(HaveKey(kartav1alpha1.LabelGVK))
			Expect(got.Labels["custom/label"]).To(Equal("keep-me"))
		})

		It("does not patch when root kind is absent and no index labels are present", func() {
			// First reconcile creates conditions but stamps no labels (no root kind).
			Expect(k8s.Create(ctx, newKarta("karta-no-kind-idempotent", nil))).To(Succeed())
			first := reconcileAndGet("karta-no-kind-idempotent")

			// Second reconcile must be a true no-op for metadata (and for status).
			second := reconcileAndGet("karta-no-kind-idempotent")
			Expect(second.ResourceVersion).To(Equal(first.ResourceVersion))
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

// This test pins the labels-last design: when the label patch fails the
// reconcile must surface the error (so controller-runtime requeues), but the
// status conditions must already be persisted from the earlier steps.
var _ = Describe("Reconciler — label-patch failure does not block status", func() {
	var (
		ctx context.Context
		k8s client.WithWatch
		r   *Reconciler
	)

	labelPatchErr := errors.New("simulated: cannot patch labels")

	BeforeEach(func() {
		ctx = context.Background()
		k8s = fake.NewClientBuilder().
			WithScheme(buildScheme()).
			WithStatusSubresource(&kartav1alpha1.Karta{}).
			// Reject every non-status Patch (i.e. the labels patch). Status
			// patches go through SubResourcePatch, which we leave untouched.
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
					return labelPatchErr
				},
			}).
			Build()
		r = newReconciler(k8s)
	})

	It("persists status conditions and returns the label-patch error", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
		Expect(k8s.Create(ctx, newValidKarta("karta-label-fail", &gvk))).To(Succeed())

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: "karta-label-fail"}})
		// Reconcile surfaces the label error so the manager will requeue.
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, labelPatchErr)).To(BeTrue())

		// But the status conditions are already on the cluster.
		got := &kartav1alpha1.Karta{}
		Expect(k8s.Get(ctx, client.ObjectKey{Name: "karta-label-fail"}, got)).To(Succeed())
		Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionValidated).Status).
			To(Equal(metav1.ConditionTrue))
		Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Status).
			To(Equal(metav1.ConditionTrue))
		Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionReady).Status).
			To(Equal(metav1.ConditionTrue))

		// And the label is still missing — the next reconcile will retry.
		Expect(got.Labels).NotTo(HaveKey(kartav1alpha1.LabelGVK))
	})
})

// This test pins the "transient List failure must not corrupt status"
// guarantee for checkCRDExists: when the CRD list call fails we must
// neither overwrite CRDExists with a guess nor patch a Ready=False value
// derived from that guess.
var _ = Describe("Reconciler — CRD list failure does not corrupt status", func() {
	var (
		ctx context.Context
		k8s client.WithWatch
		r   *Reconciler
	)

	listErr := errors.New("simulated: cannot list CRDs")

	BeforeEach(func() {
		ctx = context.Background()
		k8s = fake.NewClientBuilder().
			WithScheme(buildScheme()).
			WithStatusSubresource(&kartav1alpha1.Karta{}).
			// Fail List calls that target CRDs; other List calls (e.g.
			// MapCRDToKartaEvent listing Kartas) keep working.
			WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if _, ok := list.(*apiextensionsv1.CustomResourceDefinitionList); ok {
						return listErr
					}
					return c.List(ctx, list, opts...)
				},
			}).
			Build()
		r = newReconciler(k8s)
	})

	It("returns the list error and does not patch status or labels", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		Expect(k8s.Create(ctx, newValidKarta("karta-list-fail", &gvk))).To(Succeed())

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: "karta-list-fail"}})
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, listErr)).To(BeTrue())

		// Status is patched via defer so whatever validateKarta wrote is
		// persisted, but CRDExists is NOT set to False (checkCRDExists
		// returned early before calling setCRDExists).
		// Labels are NOT stamped because ensureLabels was not reached.
		got := &kartav1alpha1.Karta{}
		Expect(k8s.Get(ctx, client.ObjectKey{Name: "karta-list-fail"}, got)).To(Succeed())
		_, hasCRDExists := findConditionOpt(got.Status.Conditions, kartav1alpha1.ConditionCRDExists)
		Expect(hasCRDExists).To(BeFalse(),
			"CRDExists must not be set to False when the CRD list call failed transiently")
		Expect(got.Labels).NotTo(HaveKey(kartav1alpha1.LabelGVK),
			"labels must not be patched when checkCRDExists returns early")
	})

	It("does not overwrite an existing CRDExists value with a guess", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		k := newValidKarta("karta-list-fail-existing", &gvk)
		// Pre-populate a healthy CRDExists=True from a previous successful
		// reconcile.
		k.Status.Conditions = []metav1.Condition{
			{
				Type:               string(kartav1alpha1.ConditionCRDExists),
				Status:             metav1.ConditionTrue,
				Reason:             ReasonCRDFound,
				LastTransitionTime: metav1.Now(),
			},
		}
		Expect(k8s.Create(ctx, k)).To(Succeed())

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: "karta-list-fail-existing"}})
		Expect(err).To(HaveOccurred())

		got := &kartav1alpha1.Karta{}
		Expect(k8s.Get(ctx, client.ObjectKey{Name: "karta-list-fail-existing"}, got)).To(Succeed())
		// Previously-correct condition stays — we did not downgrade it to
		// False because of the transient List error.
		Expect(findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Status).
			To(Equal(metav1.ConditionTrue))
	})
})

var _ = Describe("Reconciler — detailed condition messages", func() {
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
		r = newReconciler(k8s)
	})

	reconcileAndGet := func(name string) *kartav1alpha1.Karta {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: name}})
		Expect(err).NotTo(HaveOccurred())
		out := &kartav1alpha1.Karta{}
		Expect(k8s.Get(ctx, client.ObjectKey{Name: name}, out)).To(Succeed())
		return out
	}

	It("puts the real validator error in the Validated condition message", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		// newKarta has no StatusDefinition → validator produces a descriptive error
		Expect(k8s.Create(ctx, newKarta("karta-invalid-cond-msg", &gvk))).To(Succeed())
		got := reconcileAndGet("karta-invalid-cond-msg")

		msg := findCondition(got.Status.Conditions, kartav1alpha1.ConditionValidated).Message
		Expect(msg).NotTo(BeEmpty())
		Expect(msg).NotTo(Equal("Karta validation failed"),
			"expected the real validator error, not the generic fallback")
	})

	It("puts the missing GVK detail in the CRDExists condition message", func() {
		gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
		Expect(k8s.Create(ctx, newValidKarta("karta-no-crd-cond-msg", &gvk))).To(Succeed())
		got := reconcileAndGet("karta-no-crd-cond-msg")

		msg := findCondition(got.Status.Conditions, kartav1alpha1.ConditionCRDExists).Message
		Expect(msg).To(ContainSubstring("absent.run.ai"))
		Expect(msg).To(ContainSubstring("Missing"))
		Expect(msg).To(ContainSubstring("v1"))
	})
})

var _ = Describe("Reconciler — condition-transition events", func() {
	var (
		ctx context.Context
		k8s client.WithWatch
		r   *Reconciler
		rec *record.FakeRecorder
	)

	BeforeEach(func() {
		ctx = context.Background()
		k8s = fake.NewClientBuilder().
			WithScheme(buildScheme()).
			WithStatusSubresource(&kartav1alpha1.Karta{}).
			Build()
		r, rec = newReconcilerWithRecorder(k8s)
	})

	reconcile := func(name string) {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: name}})
		Expect(err).NotTo(HaveOccurred())
	}

	It("emits a Warning event for Validated=False on the first reconcile of an invalid Karta", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		// newKarta has no StatusDefinition → fails validation
		Expect(k8s.Create(ctx, newKarta("karta-invalid-ev", &gvk))).To(Succeed())
		reconcile("karta-invalid-ev")

		events := drainEvents(rec)
		Expect(events).To(ContainElement(ContainSubstring("Warning")))
		Expect(events).To(ContainElement(ContainSubstring(string(kartav1alpha1.ConditionValidated))))
	})

	It("emits a Warning event for CRDExists=False when the CRD is missing", func() {
		gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
		Expect(k8s.Create(ctx, newValidKarta("karta-no-crd-ev", &gvk))).To(Succeed())
		reconcile("karta-no-crd-ev")

		events := drainEvents(rec)
		Expect(events).To(ContainElement(ContainSubstring(string(kartav1alpha1.ConditionCRDExists))))
		Expect(events).To(ContainElement(ContainSubstring("absent.run.ai")))
	})

	It("does not re-emit events on a steady-state reconcile where conditions are already False", func() {
		gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
		Expect(k8s.Create(ctx, newValidKarta("karta-stable-false", &gvk))).To(Succeed())

		reconcile("karta-stable-false") // first reconcile: transition → events emitted
		Expect(drainEvents(rec)).NotTo(BeEmpty())

		reconcile("karta-stable-false") // second reconcile: already False → no new events
		Expect(drainEvents(rec)).To(BeEmpty())
	})

	It("emits no events when Karta is fully ready", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		Expect(k8s.Create(ctx, newCRD("foos.test.run.ai", gvk.Group, gvk.Kind, gvk.Version))).To(Succeed())
		Expect(k8s.Create(ctx, newValidKarta("karta-ready-ev", &gvk))).To(Succeed())

		reconcile("karta-ready-ev")
		for _, e := range drainEvents(rec) {
			Expect(e).NotTo(ContainSubstring("Warning"),
				"no Warning events expected when all conditions are True")
		}
	})
})

func transitionTimes(conds []metav1.Condition) map[string]metav1.Time {
	out := map[string]metav1.Time{}
	for _, c := range conds {
		out[c.Type] = c.LastTransitionTime
	}
	return out
}
