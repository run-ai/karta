// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package pkg

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("TrimCRDFields", func() {
	var full *apiextensionsv1.CustomResourceDefinition

	BeforeEach(func() {
		full = &apiextensionsv1.CustomResourceDefinition{
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
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Kind:   "Foo",
					Plural: "foos",
				},
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
	})

	It("passes through non-CRD objects unchanged", func() {
		other := "not-a-crd"
		result, err := TrimCRDFields(other)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(other))
	})

	It("retains only the required ObjectMeta fields", func() {
		result, err := TrimCRDFields(full)
		Expect(err).NotTo(HaveOccurred())
		trimmed := result.(*apiextensionsv1.CustomResourceDefinition)

		Expect(trimmed.Name).To(Equal("foos.test.io"))
		Expect(trimmed.ResourceVersion).To(Equal("42"))
		Expect(trimmed.Generation).To(Equal(int64(3)))
		Expect(trimmed.Labels).To(BeNil())
		Expect(trimmed.Annotations).To(BeNil())
		Expect(trimmed.ManagedFields).To(BeEmpty())
	})

	It("retains Group and Kind, drops Plural and other Names fields", func() {
		result, err := TrimCRDFields(full)
		Expect(err).NotTo(HaveOccurred())
		trimmed := result.(*apiextensionsv1.CustomResourceDefinition)

		Expect(trimmed.Spec.Group).To(Equal("test.io"))
		Expect(trimmed.Spec.Names.Kind).To(Equal("Foo"))
		Expect(trimmed.Spec.Names.Plural).To(BeEmpty())
	})

	It("retains version Name and Served, drops Schema and printer columns", func() {
		result, err := TrimCRDFields(full)
		Expect(err).NotTo(HaveOccurred())
		trimmed := result.(*apiextensionsv1.CustomResourceDefinition)

		Expect(trimmed.Spec.Versions).To(HaveLen(2))
		Expect(trimmed.Spec.Versions[0].Name).To(Equal("v1"))
		Expect(trimmed.Spec.Versions[0].Served).To(BeTrue())
		Expect(trimmed.Spec.Versions[0].Schema).To(BeNil())
		Expect(trimmed.Spec.Versions[0].AdditionalPrinterColumns).To(BeEmpty())
		Expect(trimmed.Spec.Versions[1].Name).To(Equal("v1beta1"))
		Expect(trimmed.Spec.Versions[1].Served).To(BeFalse())
	})

	It("drops Conversion", func() {
		result, err := TrimCRDFields(full)
		Expect(err).NotTo(HaveOccurred())
		trimmed := result.(*apiextensionsv1.CustomResourceDefinition)
		Expect(trimmed.Spec.Conversion).To(BeNil())
	})

	It("satisfies the crdGroupKindIndex func", func() {
		result, err := TrimCRDFields(full)
		Expect(err).NotTo(HaveOccurred())
		trimmed := result.(*apiextensionsv1.CustomResourceDefinition)

		indexFunc := func(obj client.Object) []string {
			crd := obj.(*apiextensionsv1.CustomResourceDefinition)
			return []string{schema.GroupKind{Group: crd.Spec.Group, Kind: crd.Spec.Names.Kind}.String()}
		}
		Expect(indexFunc(trimmed)).To(ConsistOf("test.io/Foo"))
	})

	It("satisfies crdServesVersion for served versions", func() {
		result, err := TrimCRDFields(full)
		Expect(err).NotTo(HaveOccurred())
		trimmed := result.(*apiextensionsv1.CustomResourceDefinition)

		Expect(crdServesVersion(trimmed, "v1")).To(BeTrue())
		Expect(crdServesVersion(trimmed, "v1beta1")).To(BeFalse())
		Expect(crdServesVersion(trimmed, "v2")).To(BeFalse())
	})
})
