// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package registry

import (
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/catalog"
)

var _ = ginkgo.Describe("Registry", func() {
	var r *Registry

	jobsetGVK := schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}
	jobsetGroupKind := jobsetGVK.GroupKind()

	jobsetKarta := func(name string, age time.Duration) *v1alpha1.Karta {
		karta, err := catalog.Get(jobsetGVK)
		Expect(err).NotTo(HaveOccurred())
		karta.Name = name
		karta.CreationTimestamp = metav1.NewTime(time.Now().Add(-age))
		return karta
	}

	ginkgo.BeforeEach(func() {
		r = New()
	})

	ginkgo.It("registers a valid Karta and derives its child kinds", func() {
		r.Set(jobsetKarta("jobset", time.Hour))

		entry, ok := r.EntryFor(jobsetGroupKind)
		Expect(ok).To(BeTrue())
		Expect(entry.RootGVK).To(Equal(jobsetGVK))
		Expect(entry.ChildKinds).To(ContainElement(schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}))
		Expect(r.IsRoot(jobsetGroupKind)).To(BeTrue())
		Expect(r.Stats()).To(Equal(Stats{Valid: 1}))
	})

	ginkgo.It("chooses the oldest Karta per group and kind and shadows the rest", func() {
		r.Set(jobsetKarta("newer", time.Hour))
		r.Set(jobsetKarta("older", 2*time.Hour))

		entry, ok := r.EntryFor(jobsetGroupKind)
		Expect(ok).To(BeTrue())
		Expect(entry.Karta.Name).To(Equal("older"))
		Expect(r.Stats()).To(Equal(Stats{Valid: 1, Shadowed: 1}))
	})

	ginkgo.It("breaks creation time ties by name", func() {
		timestamp := metav1.NewTime(time.Now())
		first := jobsetKarta("aaa", 0)
		first.CreationTimestamp = timestamp
		second := jobsetKarta("bbb", 0)
		second.CreationTimestamp = timestamp

		r.Set(second)
		r.Set(first)

		entry, _ := r.EntryFor(jobsetGroupKind)
		Expect(entry.Karta.Name).To(Equal("aaa"))
	})

	ginkgo.It("counts invalid Kartas without choosing them", func() {
		r.Set(&v1alpha1.Karta{ObjectMeta: metav1.ObjectMeta{Name: "broken"}})

		Expect(r.Entries()).To(BeEmpty())
		Expect(r.Stats()).To(Equal(Stats{Invalid: 1}))
	})

	ginkgo.It("promotes the shadowed Karta when the chosen one is removed", func() {
		r.Set(jobsetKarta("older", 2*time.Hour))
		r.Set(jobsetKarta("newer", time.Hour))

		r.Remove("older")

		entry, ok := r.EntryFor(jobsetGroupKind)
		Expect(ok).To(BeTrue())
		Expect(entry.Karta.Name).To(Equal("newer"))
		Expect(r.Stats()).To(Equal(Stats{Valid: 1}))
	})

	ginkgo.It("drops the entry when the last Karta of a kind is removed", func() {
		r.Set(jobsetKarta("jobset", time.Hour))
		r.Remove("jobset")

		Expect(r.Entries()).To(BeEmpty())
		Expect(r.IsRoot(jobsetGroupKind)).To(BeFalse())
	})
})
