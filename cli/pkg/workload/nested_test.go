// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package workload

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/cli/pkg/definitions"
	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// nestedKarta declares a multi-instance parent whose child bears the pod spec.
// No built-in definition has this shape, so it is built here rather than loaded
// from testdata.
func nestedKarta() *v1alpha1.Karta {
	return &v1alpha1.Karta{
		TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
		ObjectMeta: metav1.ObjectMeta{Name: "example-com-pipeline-v1"},
		Spec: v1alpha1.KartaSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "pipeline",
					Kind: &v1alpha1.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Pipeline"},
					StatusDefinition: &v1alpha1.StatusDefinition{
						PhaseDefinition: &v1alpha1.PhaseDefinition{Path: ".status.phase"},
						StatusMappings: v1alpha1.StatusMappings{
							Running: []v1alpha1.StatusMatcher{{ByPhase: "Running"}},
						},
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:           "service",
						OwnerRef:       ptr.To("pipeline"),
						InstanceIdPath: ptr.To(".spec.services | to_entries[] | .key"),
						PodSelector: &v1alpha1.PodSelector{
							ComponentInstanceSelector: &v1alpha1.ComponentInstanceSelector{
								IdPath: `.metadata.labels["service"]`,
							},
						},
					},
					{
						Name:     "runner",
						OwnerRef: ptr.To("service"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodSpecPath: ptr.To(".spec.runner"),
						},
						ScaleDefinition: &v1alpha1.ScaleDefinition{
							ReplicasPath: ptr.To(".spec.runner.replicas"),
						},
					},
				},
			},
		},
	}
}

var _ = Describe("Resolve with a multi-instance parent", func() {
	// The child must be reported regardless of how many instances the parent
	// happens to have, and counted exactly once.
	It("reports a pod-bearing child of a multi-instance component", func() {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Pipeline",
			"metadata":   map[string]any{"name": "pipe", "namespace": "ml-team"},
			"spec": map[string]any{
				"services": map[string]any{"a": map[string]any{}, "b": map[string]any{}},
				"runner": map[string]any{
					"replicas": int64(2),
					"containers": []any{map[string]any{
						"name": "run",
						"resources": map[string]any{
							"requests": map[string]any{"nvidia.com/gpu": "4"},
						},
					}},
				},
			},
		}}

		view, err := Resolve(context.Background(), obj,
			definitions.Definition{Karta: nestedKarta(), Origin: definitions.OriginCatalog})
		Expect(err).NotTo(HaveOccurred())

		Expect(components(view)).To(Equal(map[string]int32{"runner": 2}))
		Expect(view.GPUs).To(BeEquivalentTo(8))
	})
})
