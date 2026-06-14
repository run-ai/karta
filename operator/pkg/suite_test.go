// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	"strings"
	"testing"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Karta Operator Suite")
}

// buildScheme builds a runtime.Scheme populated with the API types this
// operator interacts with. Used by every test that creates a fake client.
func buildScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	Expect(kartav1alpha1.AddToScheme(s)).To(Succeed())
	Expect(apiextensionsv1.AddToScheme(s)).To(Succeed())
	return s
}

// defaultRLConfig is the rate-limiter config used by all test reconcilers.
var defaultRLConfig = RateLimiterConfig{
	BaseDelay: DefaultRateLimiterBaseDelay,
	MaxDelay:  DefaultRateLimiterMaxDelay,
}

// newReconciler wraps NewReconciler with default config so test files don't
// need to repeat the config on every call. Events are buffered and silently
// discarded unless the test explicitly reads from the recorder channel.
func newReconciler(k8s client.WithWatch) *Reconciler {
	return NewReconciler(k8s, defaultRLConfig, record.NewFakeRecorder(64))
}

// newReconcilerWithRecorder creates a Reconciler whose events can be asserted.
func newReconcilerWithRecorder(k8s client.WithWatch) (*Reconciler, *record.FakeRecorder) {
	rec := record.NewFakeRecorder(64)
	return NewReconciler(k8s, defaultRLConfig, rec), rec
}

// kartaLabels returns the three GVK index labels the operator stamps for the given GVK.
// Mirrors the logic in ensureLabels so tests can set up expected state.
func kartaLabels(gvk schema.GroupVersionKind) map[string]string {
	return map[string]string{
		kartav1alpha1.LabelRootGroup:   gvk.Group,
		kartav1alpha1.LabelRootVersion: gvk.Version,
		kartav1alpha1.LabelRootKind:    gvk.Kind,
	}
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

// findCondition returns the condition with the given type, failing the test
// when not found.
func findCondition(conds []metav1.Condition, t kartav1alpha1.ConditionType) metav1.Condition {
	GinkgoHelper()
	c, found := findConditionOpt(conds, t)
	if !found {
		Fail("condition not found: " + string(t))
	}
	return c
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

// newCRD builds a CustomResourceDefinition serving the given versions for
// the supplied group/kind. The first version is marked as storage.
func newCRD(name, group, kind string, versions ...string) *apiextensionsv1.CustomResourceDefinition {
	v := make([]apiextensionsv1.CustomResourceDefinitionVersion, 0, len(versions))
	for _, ver := range versions {
		v = append(v, apiextensionsv1.CustomResourceDefinitionVersion{
			Name: ver, Served: true, Storage: ver == versions[0],
		})
	}
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   kind,
				Plural: strings.ToLower(kind) + "s",
			},
			Scope:    apiextensionsv1.ClusterScoped,
			Versions: v,
		},
	}
}
