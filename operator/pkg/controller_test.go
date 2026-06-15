// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
