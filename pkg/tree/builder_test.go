// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package tree_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/tree"
	"github.com/run-ai/karta/test/types"
)

func newPod(name, role string) corev1.Pod {
	return corev1.Pod{
		TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    map[string]string{"role": role},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

var _ = Describe("Build", func() {
	var (
		ctx     context.Context
		karta   *v1alpha1.Karta
		matcher tree.PodMatcher
	)

	BeforeEach(func() {
		ctx = context.Background()
		karta = types.PyFlowKarta()
		matcher = tree.JQMatcher{}
	})

	It("rejects a nil karta definition", func() {
		_, err := tree.Build(ctx, nil, types.NewPyFlowObject(), nil, matcher)
		Expect(err).To(HaveOccurred())
	})

	It("rejects a nil workload", func() {
		_, err := tree.Build(ctx, karta, nil, nil, matcher)
		Expect(err).To(HaveOccurred())
	})

	It("returns root-level child components for a single-instance workload", func() {
		workload := types.NewPyFlowObject()
		pods := []corev1.Pod{
			newPod("master-0", "master"),
			newPod("worker-0", "worker"),
			newPod("worker-1", "worker"),
		}

		got, err := tree.Build(ctx, karta, workload, pods, matcher)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).NotTo(BeNil())
		Expect(got.Children).To(HaveLen(2))

		byName := map[string]tree.ComponentNode{}
		for _, c := range got.Children {
			byName[c.Name] = c
		}

		Expect(byName).To(HaveKey("master"))
		Expect(byName).To(HaveKey("worker"))

		master := byName["master"]
		Expect(master.Instances).To(HaveLen(1))
		Expect(master.Instances[0].Pods).To(HaveLen(1))
		Expect(master.Instances[0].Pods[0].Name).To(Equal("master-0"))

		worker := byName["worker"]
		Expect(worker.Instances).To(HaveLen(1))
		Expect(worker.Instances[0].Pods).To(HaveLen(2))
	})

	It("attaches no pods when none match the component selectors", func() {
		workload := types.NewPyFlowObject()
		pods := []corev1.Pod{newPod("orphan-0", "stranger")}

		got, err := tree.Build(ctx, karta, workload, pods, matcher)
		Expect(err).NotTo(HaveOccurred())
		for _, comp := range got.Children {
			for _, inst := range comp.Instances {
				Expect(inst.Pods).To(BeEmpty())
			}
		}
	})

	It("preserves the declared component name in tree order", func() {
		workload := types.NewPyFlowObject()
		got, err := tree.Build(ctx, karta, workload, nil, matcher)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Children).To(HaveLen(2))
		Expect(got.Children[0].Name).To(Equal("master"))
		Expect(got.Children[1].Name).To(Equal("worker"))
	})

	It("uses JQMatcher by default when matcher is nil", func() {
		workload := types.NewPyFlowObject()
		pods := []corev1.Pod{newPod("master-0", "master")}

		got, err := tree.Build(ctx, karta, workload, pods, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).NotTo(BeNil())
	})
})
