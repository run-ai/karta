// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package store

import (
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Store", func() {
	var s *Store

	workload := func(uid string) WorkloadRecord {
		return WorkloadRecord{
			UID: types.UID(uid),
			Ref: WorkloadRef{Namespace: "team-a", Name: "llm-" + uid, Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
		}
	}

	pod := func(uid, workloadUID string) PodRecord {
		return PodRecord{
			UID:         types.UID(uid),
			Namespace:   "team-a",
			Name:        "pod-" + uid,
			WorkloadUID: types.UID(workloadUID),
			Component:   "replicatedjob",
			Phase:       corev1.PodRunning,
		}
	}

	BeforeEach(func() {
		s = New()
	})

	It("stores and returns workload records", func() {
		s.UpsertWorkload(workload("w1"))

		record, ok := s.Workload("w1")
		Expect(ok).To(BeTrue())
		Expect(record.Ref.Name).To(Equal("llm-w1"))
	})

	It("indexes pods by workload", func() {
		s.UpsertPod(pod("p1", "w1"))
		s.UpsertPod(pod("p2", "w1"))
		s.UpsertPod(pod("p3", "w2"))

		Expect(s.PodsOfWorkload("w1")).To(HaveLen(2))
		Expect(s.PodsOfWorkload("w2")).To(HaveLen(1))
	})

	It("moves a pod between workloads on re-attribution", func() {
		s.UpsertPod(pod("p1", "w1"))
		s.UpsertPod(pod("p1", "w2"))

		Expect(s.PodsOfWorkload("w1")).To(BeEmpty())
		Expect(s.PodsOfWorkload("w2")).To(HaveLen(1))
	})

	It("removes deleted pods from the reverse index", func() {
		s.UpsertPod(pod("p1", "w1"))
		s.DeletePod("p1")

		Expect(s.PodsOfWorkload("w1")).To(BeEmpty())
		_, ok := s.Pod("p1")
		Expect(ok).To(BeFalse())
	})

	It("cascades workload deletion to its pods", func() {
		s.UpsertWorkload(workload("w1"))
		s.UpsertPod(pod("p1", "w1"))
		s.UpsertPod(pod("p2", "w2"))

		s.DeleteWorkload("w1")

		snapshot := s.Snapshot()
		Expect(snapshot.Workloads).To(BeEmpty())
		Expect(snapshot.Pods).To(HaveLen(1))
		Expect(snapshot.Pods[0].UID).To(Equal(types.UID("p2")))
	})

	It("returns a consistent snapshot", func() {
		s.UpsertWorkload(workload("w1"))
		s.UpsertPod(pod("p1", "w1"))

		snapshot := s.Snapshot()
		Expect(snapshot.Workloads).To(HaveLen(1))
		Expect(snapshot.Pods).To(HaveLen(1))
	})

	It("survives concurrent mutation and snapshotting", func() {
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(2)
			go func(n int) {
				defer GinkgoRecover()
				defer wg.Done()
				uid := fmt.Sprintf("p%d", n)
				s.UpsertPod(pod(uid, fmt.Sprintf("w%d", n%5)))
				s.DeletePod(types.UID(uid))
			}(i)
			go func(n int) {
				defer GinkgoRecover()
				defer wg.Done()
				s.UpsertWorkload(workload(fmt.Sprintf("w%d", n%5)))
				_ = s.Snapshot()
			}(i)
		}
		wg.Wait()

		Expect(s.Snapshot().Pods).To(BeEmpty())
	})
})
