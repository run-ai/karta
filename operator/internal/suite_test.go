// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package internal

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

// kartaLabels returns the index labels the operator stamps for the given GVK.
// Mirrors the logic in stepEnsureLabels so tests can set up expected state.
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

// newValidKarta builds a Karta that passes KartaValidator.Validate(): it has
// a root component with a full GVK and a minimal StatusDefinition.
func newValidKarta(name string, gvk *schema.GroupVersionKind) *kartav1alpha1.Karta {
	k := newKarta(name, gvk)
	if gvk != nil {
		k.Spec.StructureDefinition.RootComponent.StatusDefinition = &kartav1alpha1.StatusDefinition{
			StatusMappings: kartav1alpha1.StatusMappings{},
		}
	}
	return k
}

// findCondition returns the condition with the given type, failing the test
// when not found.
func findCondition(conds []metav1.Condition, t kartav1alpha1.ConditionType) metav1.Condition {
	GinkgoHelper()
	for _, c := range conds {
		if c.Type == string(t) {
			return c
		}
	}
	Fail("condition not found: " + string(t))
	return metav1.Condition{}
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
