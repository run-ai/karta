// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/run-ai/karta/pkg/resource"
	"github.com/run-ai/karta/test/types"
)

var _ = Describe("WorkloadTree JSON serialization", func() {
	ctx := context.Background()

	// The JSON shape is a wire format consumed outside this module (e.g. the
	// Headlamp plugin), so field names are asserted explicitly.
	It("marshals the tree with stable camelCase field names", func() {
		factory := resource.NewComponentFactoryFromObject(types.PyFlowKarta(), types.NewPyFlowObject())
		tree, err := Build(ctx, factory)
		Expect(err).NotTo(HaveOccurred())

		raw, err := json.Marshal(tree)
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]any
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())

		status, ok := decoded["status"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(status["phases"]).To(ConsistOf("Running"))

		children, ok := decoded["children"].([]any)
		Expect(ok).To(BeTrue())
		Expect(children).To(HaveLen(2))

		names := make([]any, 0, len(children))
		for _, child := range children {
			component, ok := child.(map[string]any)
			Expect(ok).To(BeTrue())
			names = append(names, component["name"])
			Expect(component).To(HaveKey("hasPodDefinition"))

			instances, ok := component["instances"].([]any)
			Expect(ok).To(BeTrue())
			Expect(instances).NotTo(BeEmpty())
			instance, ok := instances[0].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(instance).To(HaveKey("extractedInstance"))

			if component["name"] == "worker" {
				scale, ok := instance["scale"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(scale["minReplicas"]).To(BeEquivalentTo(1))
				Expect(scale["maxReplicas"]).To(BeEquivalentTo(5))
			}
		}
		Expect(names).To(ConsistOf("master", "worker"))
	})

	It("round-trips through JSON without losing tree structure", func() {
		factory := resource.NewComponentFactoryFromObject(types.PyFlowKarta(), types.NewPyFlowObject())
		tree, err := Build(ctx, factory)
		Expect(err).NotTo(HaveOccurred())

		raw, err := json.Marshal(tree)
		Expect(err).NotTo(HaveOccurred())

		var decoded WorkloadTree
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
		Expect(decoded.Status.Phases).To(Equal(tree.Status.Phases))
		Expect(componentNames(decoded.Children)).To(ConsistOf("master", "worker"))
		worker := findComponent(decoded.Children, "worker")
		Expect(worker.Instances).To(HaveLen(1))
		Expect(*worker.Instances[0].Scale.MinReplicas).To(Equal(int32(1)))
	})
})
