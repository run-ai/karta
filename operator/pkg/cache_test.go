// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("TrimCRDFields", func() {
	It("passes through non-CRD objects unchanged", func() {
		result, err := TrimCRDFields("not-a-crd")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("not-a-crd"))
	})

	It("keeps only the fields the reconciler reads, and nothing else", func() {
		full := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "foos.test.io",
				ResourceVersion: "42",
				Generation:      3,
				Labels:          map[string]string{"some": "label"},
				Annotations:     map[string]string{"some": "annotation"},
				ManagedFields:   []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "test.io",
				Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "Foo", Plural: "foos"},
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{Name: "v1", Served: true, Storage: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
						},
						AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
							{Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp"},
						},
					},
					{Name: "v1beta1", Served: false, Storage: false},
				},
				Conversion: &apiextensionsv1.CustomResourceConversion{Strategy: "None"},
			},
		}

		result, err := TrimCRDFields(full)
		Expect(err).NotTo(HaveOccurred())

		// Deep-equal the minimal object: any leaked or dropped field fails here.
		Expect(result).To(Equal(&apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "foos.test.io",
				ResourceVersion: "42",
				Generation:      3,
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "test.io",
				Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "Foo"},
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{Name: "v1", Served: true},
					{Name: "v1beta1", Served: false},
				},
			},
		}))

		// Drift guard: still usable by the cache readers.
		trimmed := result.(*apiextensionsv1.CustomResourceDefinition)
		Expect(schema.GroupKind{Group: trimmed.Spec.Group, Kind: trimmed.Spec.Names.Kind}.String()).To(Equal("Foo.test.io"))
		Expect(crdServesVersion(trimmed, "v1")).To(BeTrue())
		Expect(crdServesVersion(trimmed, "v1beta1")).To(BeFalse())
	})
})
