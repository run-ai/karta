// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"fmt"
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
	"github.com/run-ai/karta/test/e2e/cases"
)

// builtinOperators are the cases.WorkloadCase operator keys whose kinds are built into
// Kubernetes (no operator install needed); they also carry the "builtin" label.
var builtinOperators = map[string]bool{
	"pod": true, "batch-job": true, "deployment": true, "statefulset": true, "cronjob": true,
}

// ginkgoLabels returns the labels for this case: its operator key (so
// -ginkgo.label-filter="nim" runs only cases needing the nim operator) plus "builtin"
// for the built-in kinds. The operator key matches hack/e2e and .installed-versions,
// so a cluster brought up with WORKLOADS="nim" pairs with E2E_LABELS="nim".
func ginkgoLabels(tc cases.WorkloadCase) Labels {
	if tc.Operator == "" {
		return Label()
	}
	ls := []string{tc.Operator}
	if builtinOperators[tc.Operator] {
		ls = append(ls, "builtin")
	}
	return Label(ls...)
}

func run(tc cases.WorkloadCase) {
	Describe(tc.Name, Ordered, ginkgoLabels(tc), func() {
		var karta *kartav1alpha1.Karta

		BeforeAll(func() {
			karta = &kartav1alpha1.Karta{}
			Expect(yaml.Unmarshal(mustRead(tc.KartaFile), karta)).To(Succeed())
			if karta.GetName() == "" {
				karta.SetName(tc.KartaName) // some bundled definitions omit metadata.name
			}
			Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, karta))).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, karta) })
		})

		It("the operator reconciles the Karta definition (Validated, CRDExists, Ready)", func() {
			Eventually(func(g Gomega) {
				got := &kartav1alpha1.Karta{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tc.KartaName}, got)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "Validated")).To(BeTrue(), "Validated")
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "CRDExists")).To(BeTrue(), "CRDExists")
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "Ready")).To(BeTrue(), "Ready")
			}, time.Minute, 2*time.Second).Should(Succeed())
		})

		for _, fl := range tc.Flows {
			fl := fl
			It(fmt.Sprintf("%s flow reaches %s", fl.Name, fl.Want()), func() {
				timeout := tc.Timeout
				if timeout == 0 {
					timeout = 3 * time.Minute
				}

				By("creating the workload")
				obj := &unstructured.Unstructured{}
				Expect(yaml.Unmarshal(mustRead(fl.WorkloadFile), obj)).To(Succeed())
				Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, obj))).To(Succeed())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

				// The order check runs in both modes, so a bad progression fails a plain
				// make test-e2e too, not only a record run. A gated (scale) flow is order-checked
				// by its position walk instead (driveByPosition only advances through the declared
				// steps in order, gate by gate): its dense recording legitimately includes transient
				// dips - a scaling workload is briefly not fully ready - which the subsequence check
				// would wrongly reject.
				By("watching it move through its states, in the declared order")
				rec := observeTransitions(tc, fl, obj, karta, timeout)
				if !journeyGated(fl.Journey) {
					assertObservedOrder(fl, rec.order)
				}

				// Karta is not checked here. The e2e run drives the workload and records only what its
				// own fields did; whether Karta reads each recorded state correctly, and extracts the
				// same components, is asserted offline against the fixture (go test ./test/conformance).
				if recordEnabled() {
					By("recording the conformance fixture")
					writeFixture(tc, fl, rec, karta)
				}
			})
		}
	})
}

var _ = Describe("Karta against live workloads", func() {
	for _, tc := range cases.All {
		run(tc)
	}
})

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), "read %s", path)
	return b
}
