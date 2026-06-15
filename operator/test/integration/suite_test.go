// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package integration holds the envtest-backed integration tests for the Karta
// operator. They run against a real apiserver+etcd (started via envtest) with
// the operator running inside a manager, so they require the envtest binaries
// (KUBEBUILDER_ASSETS). Pure unit tests live in the operator/pkg package and do
// not depend on these binaries.
package integration

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/run-ai/karta/operator/pkg"
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
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

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Karta Operator Integration Suite")
}

// BeforeSuite starts a real apiserver+etcd via envtest, installs the Karta CRD,
// and runs the operator inside a manager. Tests then create Karta objects and
// assert on the reconcile outcome via Eventually (no direct Reconcile calls).
var _ = BeforeSuite(func() {
	testCtx, testCancel = context.WithCancel(context.Background())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "charts", "karta", "crds")},
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
	Expect(pkg.NewReconciler(mgr.GetClient(),
		mgr.GetEventRecorderFor(pkg.ControllerName)).SetupWithManager(mgr)).To(Succeed())

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

// buildScheme builds a runtime.Scheme with the API types the operator touches.
func buildScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	Expect(kartav1alpha1.AddToScheme(s)).To(Succeed())
	Expect(apiextensionsv1.AddToScheme(s)).To(Succeed())
	return s
}

// newKarta builds a Karta with the given name and optional root GVK. The root
// component has no StatusDefinition so it will fail spec validation — use
// newValidKarta when Validated=True is expected.
func newKarta(name string, gvk *schema.GroupVersionKind) *kartav1alpha1.Karta {
	k := &kartav1alpha1.Karta{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	if gvk != nil {
		k.Spec.StructureDefinition.RootComponent.Name = name + "-root"
		k.Spec.StructureDefinition.RootComponent.Kind = &kartav1alpha1.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		}
	}
	return k
}

// newValidKarta builds a Karta that passes KartaValidator.Validate(): a root
// component with a full GVK and a minimal StatusDefinition.
func newValidKarta(name string, gvk *schema.GroupVersionKind) *kartav1alpha1.Karta {
	k := newKarta(name, gvk)
	if gvk != nil {
		k.Spec.StructureDefinition.RootComponent.StatusDefinition = &kartav1alpha1.StatusDefinition{
			StatusMappings: kartav1alpha1.StatusMappings{},
		}
	}
	return k
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

// envtestCRD builds a schema-bearing CRD that the real apiserver will accept.
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
