// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

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

// newKarta builds a Karta with the given name and optional root GVK. It is
// not intended to be a "valid" Karta in the validation sense - this PR does
// not exercise spec validation.
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
