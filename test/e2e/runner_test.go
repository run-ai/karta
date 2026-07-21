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
	"github.com/run-ai/karta/pkg/resource"
)

// extractCheck asserts on a component's extracted instances. When keys is empty
// it only requires at least one instance; otherwise each key must be present.
type extractCheck struct {
	component string
	keys      []string
}

// flow is one way the workload is driven end to end - the happy path, a failure,
// and so on. Each flow has its own workload manifest, the ordered states to record
// (the last terminal), and the terminal status Karta should report. A flow is
// recorded into conformance/<operator>/<version>/<kartaName>/<flow>/.
type flow struct {
	name         string
	workloadFile string
	states       []namedState
	// actions optionally maps a state name to a mutation fired once when that state
	// is reached, so a flow can drive a transition the operator will not make on its
	// own (for example resuming a suspended workload). Nil for most flows.
	actions map[string]stateAction
	want    kartav1alpha1.ResourceStatus
}

// workloadCase is one operator's end-to-end contract: a bundled Karta plus one or
// more flows that drive a real workload to a state, and what Karta should report.
// Every case runs against a real upstream operator that drives the workload.
type workloadCase struct {
	name         string
	operator     string // hack/e2e operator key; also the conformance/<operator> directory
	kartaFile    string
	kartaName    string
	workloadFile string // the default happy-flow workload, used when flows is empty
	ready        readyFunc
	want         kartav1alpha1.ResourceStatus
	flows        []flow
	extracts     []extractCheck
	timeout      time.Duration
}

// flowsOf returns the case's flows, synthesising a single happy flow from the
// default fields for cases that do not declare their own.
func (tc workloadCase) flowsOf() []flow {
	if len(tc.flows) > 0 {
		return tc.flows
	}
	return []flow{{
		name:         "happy",
		workloadFile: tc.workloadFile,
		states:       []namedState{{string(tc.want), tc.ready}},
		want:         tc.want,
	}}
}

// builtinOperators are the workloadCase operator keys whose kinds are built into
// Kubernetes (no operator install needed); they also carry the "builtin" label.
var builtinOperators = map[string]bool{
	"pod": true, "batch-job": true, "deployment": true, "statefulset": true, "cronjob": true,
}

// ginkgoLabels returns the labels for this case: its operator key (so
// -ginkgo.label-filter="nim" runs only cases needing the nim operator) plus "builtin"
// for the built-in kinds. The operator key matches hack/e2e and .installed-versions,
// so a cluster brought up with WORKLOADS="nim" pairs with E2E_LABELS="nim".
func (tc workloadCase) ginkgoLabels() Labels {
	if tc.operator == "" {
		return Label()
	}
	ls := []string{tc.operator}
	if builtinOperators[tc.operator] {
		ls = append(ls, "builtin")
	}
	return Label(ls...)
}

// run registers the case as an Ordered Ginkgo container: a BeforeAll creates the
// Karta, one spec asserts the operator reconciles it, then one spec per flow drives
// a workload to its state, asserts Karta's reading, and (under make record-e2e)
// records the states it went through. The operator-key label lets a subset run by
// operator (for example E2E_LABELS="nim").
func (tc workloadCase) run() {
	Describe(tc.name, Ordered, tc.ginkgoLabels(), func() {
		var karta *kartav1alpha1.Karta

		BeforeAll(func() {
			karta = &kartav1alpha1.Karta{}
			Expect(yaml.Unmarshal(mustRead(tc.kartaFile), karta)).To(Succeed())
			if karta.GetName() == "" {
				karta.SetName(tc.kartaName) // some bundled samples omit metadata.name
			}
			Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, karta))).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, karta) })
		})

		It("operator reconciles the Karta definition", func() {
			Eventually(func(g Gomega) {
				got := &kartav1alpha1.Karta{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tc.kartaName}, got)).To(Succeed())
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "Validated")).To(BeTrue(), "Validated")
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "CRDExists")).To(BeTrue(), "CRDExists")
				g.Expect(apimeta.IsStatusConditionTrue(got.Status.Conditions, "Ready")).To(BeTrue(), "Ready")
			}, time.Minute, 2*time.Second).Should(Succeed())
		})

		for _, fl := range tc.flowsOf() {
			fl := fl
			It(fmt.Sprintf("flow %q: maps the live workload to %s", fl.name, fl.want), func() {
				timeout := tc.timeout
				if timeout == 0 {
					timeout = 3 * time.Minute
				}

				obj := &unstructured.Unstructured{}
				Expect(yaml.Unmarshal(mustRead(fl.workloadFile), obj)).To(Succeed())
				Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, obj))).To(Succeed())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

				// Observe the ordered status transitions in both modes. The recorder
				// path additionally writes fixtures, but even a plain make test-e2e
				// watches the workload through its declared states and asserts the order
				// is a valid progression (Initializing before Running before the
				// terminal). It is the same watch either way, so the transition order is
				// verified live on every run, not only at record time. observeTransitions
				// fires each state's action (for example unsuspend a resumed flow) as it
				// is reached, and returns once the terminal state is observed.
				By("observing the workload's status transitions")
				rec := observeTransitions(tc, fl, obj, timeout)
				assertObservedOrder(fl, rec.order)

				By("running the Karta library against the live object")
				live := emptyLike(obj)
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), live)).To(Succeed())
				factory := resource.NewComponentFactoryFromObject(karta, live)

				root, err := factory.GetRootComponent()
				Expect(err).NotTo(HaveOccurred())
				status, err := root.GetStatus(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(status.MatchedStatuses).To(ContainElement(fl.want))

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

				if rec != nil {
					By("recording conformance for the observed states")
					writeFixture(tc, fl, karta, rec)
				}
			})
		}
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
