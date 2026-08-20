// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package definitions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	v1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// newFakeDynamic builds a dynamic fake over a hand-rolled unstructured scheme.
// The Karta types must map to unstructured here: the object tracker builds the
// list result with meta.SetList, which cannot assign []*unstructured.Unstructured
// into the typed KartaList that v1alpha1.AddToScheme would register.
func newFakeDynamic(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(v1alpha1.GroupVersion.WithKind("Karta"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(v1alpha1.GroupVersion.WithKind("KartaList"), &unstructured.UnstructuredList{})

	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{GVR: "KartaList"},
		objects...,
	)
}

// loadFixture reads a "kubectl get -o yaml" style List document into the raw
// unstructured objects a dynamic client would hand back.
func loadFixture(name string) []runtime.Object {
	GinkgoHelper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())

	var list unstructured.UnstructuredList
	Expect(utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096).Decode(&list)).To(Succeed())

	objects := make([]runtime.Object, 0, len(list.Items))
	for i := range list.Items {
		objects = append(objects, &list.Items[i])
	}
	return objects
}

var _ = Describe("listWithClient", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("converts the cluster payload into typed Kartas", func() {
		kartas, err := listWithClient(ctx, newFakeDynamic(loadFixture("kartas-list.yaml")...))
		Expect(err).NotTo(HaveOccurred())
		Expect(kartas).To(HaveLen(2))

		byName := map[string]*v1alpha1.Karta{}
		for _, k := range kartas {
			byName[k.Name] = k
		}
		Expect(byName).To(HaveKey("cluster-deployment"))
		Expect(byName).To(HaveKey("cluster-pod"))

		deployment := byName["cluster-deployment"]
		Expect(deployment.Kind).To(Equal("Karta"))
		Expect(deployment.APIVersion).To(Equal("run.ai/v1alpha1"))
		Expect(deployment.Generation).To(BeEquivalentTo(3))
		Expect(deployment.CreationTimestamp.IsZero()).To(BeFalse())
		Expect(deployment.Labels).To(HaveKeyWithValue("karta.run.ai/kind", "deployment"))

		root := deployment.Spec.StructureDefinition.RootComponent
		Expect(root.Kind).To(Equal(&v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}))
		Expect(root.SpecDefinition.PodTemplateSpecPath).To(HaveValue(Equal(".spec.template")))
		Expect(root.ScaleDefinition.ReplicasPath).To(HaveValue(Equal(".spec.replicas")))
		Expect(root.StatusDefinition.ConditionsDefinition.Path).To(Equal(".status.conditions"))
		Expect(root.StatusDefinition.StatusMappings.Running).To(HaveLen(1))
		Expect(root.StatusDefinition.StatusMappings.Running[0].ByConditions[0].Status).To(HaveValue(Equal("True")))
		Expect(root.StatusDefinition.StatusMappings.Failed[0].ByExpression.ExpectedResult).To(Equal("1"))
		Expect(root.SuspendDefinition.SuspendActions).To(Equal([]v1alpha1.SuspendAction{{Path: ".spec.replicas", Value: "0"}}))

		children := deployment.Spec.StructureDefinition.ChildComponents
		Expect(children).To(HaveLen(1))
		Expect(children[0].OwnerRef).To(HaveValue(Equal("deployment")))
		Expect(children[0].InstanceIdPath).To(HaveValue(Equal(".metadata.name")))
		Expect(children[0].PodSelector.ComponentTypeSelector.Value).To(HaveValue(Equal("worker")))
		Expect(children[0].PodSelector.ReplicaSelector.KeyPath).To(Equal(`.metadata.labels["replica-index"]`))

		Expect(deployment.Spec.StructureDefinition.AdditionalChildKinds).
			To(Equal([]v1alpha1.GroupVersionKind{{Version: "v1", Kind: "Service"}}))

		podGroup := deployment.Spec.Instructions.GangScheduling.PodGroup
		Expect(podGroup.Name).To(Equal("deployment-group"))
		Expect(podGroup.Topology.RequiredTopologyLevel).To(Equal("datacenter"))
		Expect(podGroup.SubGroups[0].Topology.PreferredTopologyLevel).To(Equal("rack"))

		Expect(deployment.Status.Conditions).To(HaveLen(1))
		Expect(deployment.Status.Conditions[0].Type).To(Equal("Validated"))
		Expect(deployment.Status.Conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
		Expect(deployment.Status.Conditions[0].ObservedGeneration).To(BeEquivalentTo(3))

		pod := byName["cluster-pod"]
		Expect(pod.Spec.StructureDefinition.RootComponent.Kind.Group).To(BeEmpty())
		Expect(pod.Spec.StructureDefinition.RootComponent.SpecDefinition.MetadataPath).To(HaveValue(Equal(".metadata")))
	})

	It("issues a cluster-scoped list against the kartas resource", func() {
		client := newFakeDynamic(loadFixture("kartas-list.yaml")...)

		var recorded []k8stesting.ListAction
		client.PrependReactor("list", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
			recorded = append(recorded, action.(k8stesting.ListAction))
			return false, nil, nil
		})

		_, err := listWithClient(ctx, client)
		Expect(err).NotTo(HaveOccurred())

		Expect(recorded).To(HaveLen(1))
		Expect(recorded[0].GetResource()).To(Equal(GVR))
		Expect(recorded[0].GetNamespace()).To(BeEmpty())
		Expect(recorded[0].GetListRestrictions().Labels.Empty()).To(BeTrue())
		Expect(recorded[0].GetListRestrictions().Fields.Empty()).To(BeTrue())
	})

	It("returns no definitions and no error for an empty cluster", func() {
		kartas, err := listWithClient(ctx, newFakeDynamic())
		Expect(err).NotTo(HaveOccurred())
		Expect(kartas).To(BeEmpty())
	})
})
