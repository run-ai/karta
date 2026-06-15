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

// The tests in this file call Reconcile directly on purpose: they inject
// faults (List/Patch errors) or capture emitted events synchronously, neither
// of which can be reproduced against the real apiserver that envtest runs.
// Happy-path reconcile behavior is covered end-to-end in controller_test.go.

// newValidKarta builds a Karta that passes KartaValidator.Validate(): it has
// a root component with a full GVK and a minimal StatusDefinition. Shared with
// the envtest suite in controller_test.go.
func newValidKarta(name string, gvk *schema.GroupVersionKind) *kartav1alpha1.Karta {
	k := newKarta(name, gvk)
	if gvk != nil {
		k.Spec.StructureDefinition.RootComponent.StatusDefinition = &kartav1alpha1.StatusDefinition{
			StatusMappings: kartav1alpha1.StatusMappings{},
		}
	}
	return k
}

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

// These early-return guards can't be exercised deterministically under
// envtest (a create immediately races the reconcile), so they call Reconcile
// directly against a fake client.
var _ = Describe("Reconciler — control-flow guards", func() {
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

	It("returns no error when the Karta does not exist", func() {
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: "missing"}})
		Expect(err).NotTo(HaveOccurred())
	})

	It("skips reconciliation and leaves status empty while the Karta is being deleted", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		k := newValidKarta("karta-deleting", &gvk)
		k.Finalizers = []string{"keep.test/finalizer"}
		Expect(k8s.Create(ctx, k)).To(Succeed())
		Expect(k8s.Delete(ctx, k)).To(Succeed())

		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: k.Name}})
		Expect(err).NotTo(HaveOccurred())

		got := &kartav1alpha1.Karta{}
		Expect(k8s.Get(ctx, client.ObjectKey{Name: k.Name}, got)).To(Succeed())
		Expect(got.Status.Conditions).To(BeEmpty())
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

		// And the labels are still missing — the next reconcile will retry.
		Expect(got.Labels).NotTo(HaveKey(kartav1alpha1.LabelRootGroup))
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
		Expect(got.Labels).NotTo(HaveKey(kartav1alpha1.LabelRootGroup),
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
		Expect(events).To(ContainElement(ContainSubstring(ReasonValidationFailed)))
	})

	It("emits a Warning event for CRDExists=False when the CRD is missing", func() {
		gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
		Expect(k8s.Create(ctx, newValidKarta("karta-no-crd-ev", &gvk))).To(Succeed())
		reconcile("karta-no-crd-ev")

		events := drainEvents(rec)
		Expect(events).To(ContainElement(ContainSubstring(ReasonCRDNotFound)))
		Expect(events).To(ContainElement(ContainSubstring("absent.run.ai")))
	})

	It("re-emits events on every reconcile while conditions are False (k8s aggregator updates count)", func() {
		gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
		Expect(k8s.Create(ctx, newValidKarta("karta-stable-false", &gvk))).To(Succeed())

		reconcile("karta-stable-false")
		Expect(drainEvents(rec)).NotTo(BeEmpty())

		reconcile("karta-stable-false") // still False → event fires again, aggregator increments count
		Expect(drainEvents(rec)).NotTo(BeEmpty())
	})

	It("emits no Warning events when Karta is fully ready", func() {
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
