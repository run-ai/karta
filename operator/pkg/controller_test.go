// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	eventuallyTimeout  = 10 * time.Second
	eventuallyInterval = 200 * time.Millisecond
)

var (
	testEnv    *envtest.Environment
	k8sClient  client.Client
	testCtx    context.Context
	testCancel context.CancelFunc
)

// BeforeSuite starts a real apiserver+etcd via envtest, installs the Karta CRD,
// and runs the operator inside a manager. Tests then create Karta objects and
// assert on the reconcile outcome via Eventually (no direct Reconcile calls).
var _ = BeforeSuite(func() {
	testCtx, testCancel = context.WithCancel(context.Background())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "charts", "karta", "crds")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := buildScheme()

	// Direct (uncached) client for test assertions to avoid informer-cache staleness.
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"}, // disable metrics listener in tests
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(NewReconciler(mgr.GetClient(),
		mgr.GetEventRecorderFor(ControllerName)).SetupWithManager(mgr)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(testCtx)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	if testCancel != nil {
		testCancel()
	}
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})

var _ = Describe("Reconciler (envtest)", func() {
	getKarta := func(k *kartav1alpha1.Karta) *kartav1alpha1.Karta {
		out := &kartav1alpha1.Karta{}
		Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(k), out)).To(Succeed())
		return out
	}

	It("sets Validated=False with the real validator error for an invalid Karta", func() {
		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Foo"}
		k := newKarta("envtest-invalid", &gvk) // no StatusDefinition → invalid
		Expect(k8sClient.Create(testCtx, k)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, k) })

		Eventually(func(g Gomega) {
			c, ok := findConditionOpt(getKarta(k).Status.Conditions, kartav1alpha1.ConditionValidated)
			g.Expect(ok).To(BeTrue())
			g.Expect(c.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(c.Reason).To(Equal(ReasonValidationFailed))
			g.Expect(c.Message).NotTo(BeEmpty())
			g.Expect(c.Message).NotTo(Equal("Karta validation failed"),
				"expected the real validator error, not the generic fallback")
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())
	})

	It("sets CRDExists=False and Ready=False when the referenced CRD is missing", func() {
		gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
		k := newValidKarta("envtest-no-crd", &gvk)
		Expect(k8sClient.Create(testCtx, k)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, k) })

		Eventually(func(g Gomega) {
			conds := getKarta(k).Status.Conditions
			crd, ok := findConditionOpt(conds, kartav1alpha1.ConditionCRDExists)
			g.Expect(ok).To(BeTrue())
			g.Expect(crd.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(crd.Message).To(ContainSubstring("absent.run.ai"))
			ready, ok := findConditionOpt(conds, kartav1alpha1.ConditionReady)
			g.Expect(ok).To(BeTrue())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())
	})

	It("sets Ready=True and stamps index labels when the CRD exists", func() {
		crd := envtestCRD("widgets.test.run.ai", "test.run.ai", "Widget", "v1")
		Expect(k8sClient.Create(testCtx, crd)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, crd) })

		gvk := schema.GroupVersionKind{Group: "test.run.ai", Version: "v1", Kind: "Widget"}
		k := newValidKarta("envtest-ready", &gvk)
		Expect(k8sClient.Create(testCtx, k)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, k) })

		Eventually(func(g Gomega) {
			got := getKarta(k)
			validated, ok := findConditionOpt(got.Status.Conditions, kartav1alpha1.ConditionValidated)
			g.Expect(ok).To(BeTrue())
			g.Expect(validated.Status).To(Equal(metav1.ConditionTrue))
			crdExists, ok := findConditionOpt(got.Status.Conditions, kartav1alpha1.ConditionCRDExists)
			g.Expect(ok).To(BeTrue())
			g.Expect(crdExists.Status).To(Equal(metav1.ConditionTrue))
			ready, ok := findConditionOpt(got.Status.Conditions, kartav1alpha1.ConditionReady)
			g.Expect(ok).To(BeTrue())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(got.Labels[kartav1alpha1.LabelRootGroup]).To(Equal(gvk.Group))
			g.Expect(got.Labels[kartav1alpha1.LabelRootVersion]).To(Equal(gvk.Version))
			g.Expect(got.Labels[kartav1alpha1.LabelRootKind]).To(Equal(gvk.Kind))
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())
	})

	It("preserves a foreign condition written by another controller", func() {
		gvk := schema.GroupVersionKind{Group: "absent.run.ai", Version: "v1", Kind: "Missing"}
		k := newValidKarta("envtest-foreign", &gvk)
		Expect(k8sClient.Create(testCtx, k)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(testCtx, k) })

		// Wait for the operator to write its conditions, then add a foreign one.
		Eventually(func(g Gomega) {
			_, ok := findConditionOpt(getKarta(k).Status.Conditions, kartav1alpha1.ConditionReady)
			g.Expect(ok).To(BeTrue())
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())

		// Retry on conflict: the operator may patch status concurrently.
		Eventually(func() error {
			got := getKarta(k)
			got.Status.Conditions = append(got.Status.Conditions, metav1.Condition{
				Type: "RBACReady", Status: metav1.ConditionTrue, Reason: "EWI",
				LastTransitionTime: metav1.Now(),
			})
			return k8sClient.Status().Update(testCtx, got)
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())

		// Force another reconcile by touching the spec, then verify RBACReady survives.
		Eventually(func() error {
			got := getKarta(k)
			got.Spec.StructureDefinition.RootComponent.Name = "renamed-root"
			return k8sClient.Update(testCtx, got)
		}, eventuallyTimeout, eventuallyInterval).Should(Succeed())

		Consistently(func(g Gomega) {
			rbac, ok := findConditionOpt(getKarta(k).Status.Conditions, "RBACReady")
			g.Expect(ok).To(BeTrue())
			g.Expect(rbac.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(rbac.Reason).To(Equal("EWI"))
		}, 2*time.Second, eventuallyInterval).Should(Succeed())
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

// envtestCRD builds a schema-bearing CRD that the real apiserver will accept
// (the fake-client newCRD helper omits the schema and is not valid for envtest).
func envtestCRD(name, group, kind, version string) *apiextensionsv1.CustomResourceDefinition {
	preserve := true
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   kind,
				Plural: strings.ToLower(kind) + "s",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    version,
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type:                   "object",
						XPreserveUnknownFields: &preserve,
					},
				},
			}},
		},
	}
}

// ---------------------------------------------------------------------------
// Direct-Reconcile tests
//
// The specs below call Reconcile directly against a fake client on purpose:
// they inject faults (List/Patch errors) or capture emitted events
// synchronously, neither of which can be reproduced against the real apiserver
// that envtest runs. Happy-path reconcile behavior is covered by the envtest
// suite above.
// ---------------------------------------------------------------------------

// newValidKarta builds a Karta that passes KartaValidator.Validate(): it has
// a root component with a full GVK and a minimal StatusDefinition. Shared with
// the envtest suite above.
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

// newReconcilerWithRecorder creates a Reconciler whose events can be asserted.
func newReconcilerWithRecorder(k8s client.WithWatch) (*Reconciler, *record.FakeRecorder) {
	rec := record.NewFakeRecorder(64)
	return NewReconciler(k8s, rec), rec
}

// findConditionOpt returns the condition with the given type and whether it was
// found, without failing the test.
func findConditionOpt(conds []metav1.Condition, t kartav1alpha1.ConditionType) (metav1.Condition, bool) {
	for _, c := range conds {
		if c.Type == string(t) {
			return c, true
		}
	}
	return metav1.Condition{}, false
}
