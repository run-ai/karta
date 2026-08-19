// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package state

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog"
	"github.com/run-ai/karta/pkg/instructions"

	"github.com/run-ai/karta/exporter/pkg/registry"
	"github.com/run-ai/karta/exporter/pkg/store"
)

func jobsetEntry() *registry.Entry {
	karta, err := catalog.Get(schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"})
	Expect(err).NotTo(HaveOccurred())

	r := registry.New()
	r.Set(karta)
	entry, ok := r.EntryFor(schema.GroupKind{Group: "jobset.x-k8s.io", Kind: "JobSet"})
	Expect(ok).To(BeTrue())
	return entry
}

func jobsetObject(status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "llm", "namespace": "team-a", "uid": "wl-1"},
		"spec": map[string]any{
			"replicatedJobs": []any{
				map[string]any{
					"name": "prefill", "replicas": int64(1),
					"template": map[string]any{"spec": map[string]any{"parallelism": int64(2)}},
				},
				map[string]any{
					"name": "decode", "replicas": int64(1),
					"template": map[string]any{"spec": map[string]any{"parallelism": int64(1)}},
				},
			},
		},
		"status": status,
	}}
}

var _ = Describe("Build", func() {
	ctx := context.Background()
	ref := store.WorkloadRef{Namespace: "team-a", Name: "llm", Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}

	replicas := func(record store.WorkloadRecord) map[string]int32 {
		result := map[string]int32{}
		for _, componentState := range record.Components {
			if componentState.Replicas != nil {
				result[componentState.Component+"/"+componentState.Instance] = *componentState.Replicas
			}
		}
		return result
	}

	It("extracts matched phases and per-instance desired replicas", func() {
		workload := jobsetObject(map[string]any{
			"conditions": []any{},
			"replicatedJobsStatus": []any{
				map[string]any{"name": "prefill", "active": int64(2), "ready": int64(2), "failed": int64(0)},
				map[string]any{"name": "decode", "active": int64(1), "ready": int64(1), "failed": int64(0)},
			},
		})

		record, err := Build(ctx, jobsetEntry(), workload, "wl-1", ref)

		Expect(err).NotTo(HaveOccurred())
		Expect(record.HasStatus).To(BeTrue())
		Expect(record.Phases).To(ConsistOf(v1alpha1.RunningStatus))
		Expect(replicas(record)).To(Equal(map[string]int32{
			"replicatedjob/prefill": 2,
			"replicatedjob/decode":  1,
		}))
		Expect(record.Karta).To(Equal("jobset-x-k8s-io-jobset-v1alpha2"))
	})

	It("emits Undefined when mappings are present but nothing matches", func() {
		workload := jobsetObject(map[string]any{
			"conditions":           []any{},
			"replicatedJobsStatus": []any{},
		})

		record, err := Build(ctx, jobsetEntry(), workload, "wl-1", ref)

		Expect(err).NotTo(HaveOccurred())
		Expect(record.HasStatus).To(BeTrue())
		Expect(record.Phases).To(ConsistOf(v1alpha1.UndefinedStatus))
	})

	It("matches condition-based statuses", func() {
		workload := jobsetObject(map[string]any{
			"conditions": []any{
				map[string]any{"type": "Suspended", "status": "True"},
			},
			"replicatedJobsStatus": []any{},
		})

		record, err := Build(ctx, jobsetEntry(), workload, "wl-1", ref)

		Expect(err).NotTo(HaveOccurred())
		Expect(record.Phases).To(ConsistOf(v1alpha1.SuspendedStatus))
	})

	It("distinguishes a missing StatusDefinition from Undefined", func() {
		karta := &v1alpha1.Karta{
			ObjectMeta: metav1.ObjectMeta{Name: "thing"},
			Spec: v1alpha1.KartaSpec{
				StructureDefinition: v1alpha1.StructureDefinition{
					RootComponent: v1alpha1.ComponentDefinition{
						Name: "thing",
						Kind: &v1alpha1.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Thing"},
					},
				},
			},
		}
		summary, err := instructions.NewStructureSummary(karta)
		Expect(err).NotTo(HaveOccurred())
		entry := &registry.Entry{
			Karta:   karta,
			RootGVK: schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Thing"},
			Summary: summary,
		}
		workload := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "example.io/v1",
			"kind":       "Thing",
			"metadata":   map[string]any{"name": "t", "namespace": "team-a", "uid": "wl-9"},
		}}

		record, err := Build(ctx, entry, workload, "wl-9", store.WorkloadRef{Namespace: "team-a", Name: "t", Group: "example.io", Version: "v1", Kind: "Thing"})

		Expect(err).NotTo(HaveOccurred())
		Expect(record.HasStatus).To(BeFalse())
		Expect(record.Phases).To(BeEmpty())
	})
})
