// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package workload

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/cli/pkg/definitions"
	"github.com/run-ai/karta/pkg/catalog"
)

// resolveFixture reads a workload manifest from testdata and resolves it through
// the built-in catalog, the way the get command resolves a live object.
func resolveFixture(name string) (*View, error) {
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())

	obj := &unstructured.Unstructured{}
	Expect(yaml.Unmarshal(raw, obj)).To(Succeed())

	resolver := definitions.New(catalog.List(), nil)
	def, err := resolver.Resolve(obj.GroupVersionKind())
	Expect(err).NotTo(HaveOccurred())

	return Resolve(context.Background(), obj, def)
}

// components flattens a view into "name(replicas)" pairs for readable assertions.
func components(view *View) map[string]int32 {
	out := make(map[string]int32, len(view.Components))
	for _, component := range view.Components {
		out[component.Name] = component.Replicas
	}
	return out
}

var _ = Describe("Resolve", func() {
	It("reports one entry per replica spec, with GPUs multiplied by replicas", func() {
		view, err := resolveFixture("pytorchjob.yaml")
		Expect(err).NotTo(HaveOccurred())

		Expect(components(view)).To(Equal(map[string]int32{"master": 1, "worker": 4}))
		// 1*1 for master plus 8*4 for worker.
		Expect(view.GPUs).To(BeEquivalentTo(33))
		Expect(view.Definition).To(Equal("kubeflow-org-pytorchjob-v1"))
		Expect(view.Origin).To(Equal(string(definitions.OriginCatalog)))
	})

	// tree.Build hoists the root's children and drops the root itself, so a
	// workload carrying its pod template on the root resolves to nothing unless
	// the root is read separately.
	It("includes the root component when it carries the pod template", func() {
		view, err := resolveFixture("deployment.yaml")
		Expect(err).NotTo(HaveOccurred())

		Expect(components(view)).To(Equal(map[string]int32{"deployment": 3}))
		// The fixture declares GPUs only under limits, exercising the fallback.
		Expect(view.GPUs).To(BeEquivalentTo(6))
	})

	// KServe declares only a minimum, so defaulting to 1 would under-report an
	// autoscaled component by its floor.
	It("falls back to the minimum when a component declares no replica count", func() {
		view, err := resolveFixture("inferenceservice.yaml")
		Expect(err).NotTo(HaveOccurred())

		Expect(components(view)).To(HaveKeyWithValue("predictor", int32(4)))
		Expect(view.GPUs).To(BeEquivalentTo(8))
	})

	// The definition declares a transformer this workload does not use. The
	// extracted instance is non-nil but zero, so a pointer test would emit it.
	It("omits a component the workload does not use", func() {
		view, err := resolveFixture("inferenceservice.yaml")
		Expect(err).NotTo(HaveOccurred())

		Expect(components(view)).NotTo(HaveKey("transformer"))
	})

	// The single "service" child expands over .spec.services, so the row names
	// come from the instance keys rather than the component name. The definition
	// declares both a container path and a bare resources path, so counting both
	// would double the total.
	It("names multi-instance components by their instance key", func() {
		view, err := resolveFixture("dynamographdeployment.yaml")
		Expect(err).NotTo(HaveOccurred())

		Expect(components(view)).To(Equal(map[string]int32{"Frontend": 2, "PrefillWorker": 4}))
		// 1*2 for Frontend plus 8*4 for PrefillWorker, counted once.
		Expect(view.GPUs).To(BeEquivalentTo(34))
	})

	// leader and worker hang off an intermediate "group" component that carries a
	// scale but no pod spec. It should collapse out of the row rather than
	// appear as plumbing, and its size must not be multiplied in a second time -
	// the leaf replica paths already account for it.
	It("collapses an intermediate component that bears no pod spec", func() {
		view, err := resolveFixture("leaderworkerset.yaml")
		Expect(err).NotTo(HaveOccurred())

		// replicas 2, size 4: two leaders and two groups of three workers.
		Expect(components(view)).To(Equal(map[string]int32{"leader": 2, "worker": 6}))
		Expect(view.GPUs).To(BeEquivalentTo(50))
	})

	// Milvus declares eighteen children and reads GPUs from a bare resources
	// path rather than a container list.
	It("emits only the components a standalone workload uses", func() {
		view, err := resolveFixture("milvus.yaml")
		Expect(err).NotTo(HaveOccurred())

		Expect(components(view)).To(Equal(map[string]int32{"standalone": 1}))
		Expect(view.GPUs).To(BeEquivalentTo(1))
	})
})

var _ = Describe("machine output", func() {
	// A nil slice marshals to null, which a consumer cannot iterate.
	It("carries an empty component list rather than nil", func() {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "jobset.x-k8s.io/v1alpha2",
			"kind":       "JobSet",
			"metadata":   map[string]any{"name": "empty", "namespace": "ml-team"},
			"spec":       map[string]any{"replicatedJobs": []any{}},
		}}

		resolver := definitions.New(catalog.List(), nil)
		def, err := resolver.Resolve(obj.GroupVersionKind())
		Expect(err).NotTo(HaveOccurred())

		view, err := Resolve(context.Background(), obj, def)
		Expect(err).NotTo(HaveOccurred())
		Expect(view.Components).NotTo(BeNil())

		encoded, err := json.Marshal(view)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(encoded)).To(ContainSubstring(`"components":[]`))
	})
})

var _ = Describe("GPU accounting", func() {
	// Kubernetes schedules against max(largest init container, containers plus
	// sidecars), so a GPU held only for a warmup step still counts.
	It("counts a GPU requested by an init container", func() {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": "warmup", "namespace": "ml-team"},
			"spec": map[string]any{
				"replicas": int64(1),
				"template": map[string]any{"spec": map[string]any{
					"initContainers": []any{map[string]any{
						"name": "fetch-model",
						"resources": map[string]any{
							"requests": map[string]any{"nvidia.com/gpu": "4"},
						},
					}},
					"containers": []any{map[string]any{"name": "serve"}},
				}},
			},
		}}

		resolver := definitions.New(catalog.List(), nil)
		def, err := resolver.Resolve(obj.GroupVersionKind())
		Expect(err).NotTo(HaveOccurred())

		view, err := Resolve(context.Background(), obj, def)
		Expect(err).NotTo(HaveOccurred())
		Expect(view.GPUs).To(BeEquivalentTo(4))
	})
})
