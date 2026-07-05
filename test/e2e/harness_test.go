// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/pkg/resource"
)

// readyFunc reports whether a live workload object has reached the stable state
// the case asserts against. Assertions only ever target stable states, never a
// transient mid-state the upstream operator can pass through (see README).
type readyFunc func(*unstructured.Unstructured) bool

// condTrue matches when a status condition of the given type is True.
func condTrue(condType string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conds {
			m, ok := c.(map[string]any)
			if ok && m["type"] == condType && m["status"] == "True" {
				return true
			}
		}
		return false
	}
}

// phaseEq matches when a string field at the given path equals want.
func phaseEq(want string, path ...string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		got, _, _ := unstructured.NestedString(u.Object, path...)
		return got == want
	}
}

// intAtLeast matches when an integer field at the given path is present and >= min.
func intAtLeast(min int64, path ...string) readyFunc {
	return func(u *unstructured.Unstructured) bool {
		got, found, err := unstructured.NestedInt64(u.Object, path...)
		return err == nil && found && got >= min
	}
}

// extractCheck asserts on a component's extracted instances. When keys is empty
// it only requires at least one instance; otherwise each key must be present.
type extractCheck struct {
	component string
	keys      []string
}

// workloadCase is one operator's end-to-end contract: a bundled Karta plus a
// real workload, the stable state to wait for, and what Karta should report.
//
// Every case runs against a real upstream operator that drives the workload to
// its state, except builtin cases: the root type is a built-in Kubernetes kind
// (not a CRD), so the Karta operator reports CRDExists=False (it only inspects
// CRDs) while the library still maps and extracts.
type workloadCase struct {
	name         string
	kartaFile    string
	kartaName    string
	workloadFile string
	ready        readyFunc
	want         kartav1alpha1.ResourceStatus
	extracts     []extractCheck
	timeout      time.Duration
	builtin      bool
}

// run registers the case as an Ordered Ginkgo container: a BeforeAll creates the
// Karta and the workload, then two specs assert that the operator reconciles the
// Karta and that the library maps the live workload's status and extracts its
// components.
func (tc workloadCase) run() {
	Describe(tc.name, Ordered, func() {
		var (
			karta *kartav1alpha1.Karta
			obj   *unstructured.Unstructured
		)

		BeforeAll(func() {
			karta = &kartav1alpha1.Karta{}
			Expect(yaml.Unmarshal(mustRead(tc.kartaFile), karta)).To(Succeed())
			if karta.GetName() == "" {
				karta.SetName(tc.kartaName) // some bundled samples omit metadata.name
			}
			Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, karta))).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, karta) })

			obj = &unstructured.Unstructured{}
			Expect(yaml.Unmarshal(mustRead(tc.workloadFile), obj)).To(Succeed())
			Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, obj))).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
		})

		It("operator reconciles the Karta definition", func() {
			Eventually(func(g Gomega) {
				got := &kartav1alpha1.Karta{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tc.kartaName}, got)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "Validated")).To(BeTrue(), "Validated")
				if tc.builtin {
					// Built-in kinds have no CustomResourceDefinition, so the operator
					// (which inspects CRDs) reports CRDExists=False; the library still works.
					g.Expect(apimeta.IsStatusConditionFalse(got.Status.Conditions, "CRDExists")).To(BeTrue(), "CRDExists")
				} else {
					g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "CRDExists")).To(BeTrue(), "CRDExists")
					g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "Ready")).To(BeTrue(), "Ready")
				}
			}, time.Minute, 2*time.Second).Should(Succeed())
		})

		It("maps the live workload status to "+string(tc.want)+" and extracts its components", func() {
			timeout := tc.timeout
			if timeout == 0 {
				timeout = 3 * time.Minute
			}

			By("waiting for the workload to reach its target state")
			Eventually(func(g Gomega) {
				live := emptyLike(obj)
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), live)).To(Succeed())
				g.Expect(tc.ready(live)).To(BeTrue())
			}, timeout, 5*time.Second).Should(Succeed())

			By("running the Karta library against the live object")
			live := emptyLike(obj)
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), live)).To(Succeed())
			factory := resource.NewComponentFactoryFromObject(karta, live)

			root, err := factory.GetRootComponent()
			Expect(err).NotTo(HaveOccurred())
			status, err := root.GetStatus(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(status.MatchedStatuses).To(ContainElement(tc.want))

			for _, ec := range tc.extracts {
				comp, err := factory.GetComponent(ec.component)
				Expect(err).NotTo(HaveOccurred(), ec.component)
				inst, err := comp.GetExtractedInstances(ctx)
				Expect(err).NotTo(HaveOccurred(), ec.component)
				if len(ec.keys) == 0 {
					Expect(inst).NotTo(BeEmpty(), ec.component)
					continue
				}
				for _, k := range ec.keys {
					Expect(inst).To(HaveKey(k), ec.component)
				}
			}
		})
	})
}

var _ = Describe("Karta against live workloads", func() {
	for _, tc := range workloadCases {
		tc.run()
	}
})

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), "read %s", path)
	return b
}

func emptyLike(src *unstructured.Unstructured) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(src.GroupVersionKind())
	return u
}
