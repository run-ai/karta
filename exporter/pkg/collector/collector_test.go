// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package collector

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"

	"github.com/run-ai/karta/exporter/pkg/store"
)

// goldenContract is the label-contract regression gate: any change to metric
// names, label names, or series shape fails this test visibly.
const goldenContract = `# HELP karta_pod_workload_info Pod to workload, component, and component instance attribution derived from the Karta definition. Value is always 1.
# TYPE karta_pod_workload_info gauge
karta_pod_workload_info{component="replicatedjob",component_instance="decode",namespace="team-a",pod="decode-0",replica="",uid="p2",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 1
karta_pod_workload_info{component="replicatedjob",component_instance="prefill",namespace="team-a",pod="prefill-0",replica="",uid="p1",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 1
# HELP karta_workload_component_pods Observed pod count per component instance, broken down by pod phase.
# TYPE karta_workload_component_pods gauge
karta_workload_component_pods{component="replicatedjob",component_instance="decode",namespace="team-a",phase="Failed",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_component_pods{component="replicatedjob",component_instance="decode",namespace="team-a",phase="Pending",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 1
karta_workload_component_pods{component="replicatedjob",component_instance="decode",namespace="team-a",phase="Running",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_component_pods{component="replicatedjob",component_instance="decode",namespace="team-a",phase="Succeeded",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_component_pods{component="replicatedjob",component_instance="decode",namespace="team-a",phase="Unknown",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_component_pods{component="replicatedjob",component_instance="prefill",namespace="team-a",phase="Failed",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_component_pods{component="replicatedjob",component_instance="prefill",namespace="team-a",phase="Pending",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_component_pods{component="replicatedjob",component_instance="prefill",namespace="team-a",phase="Running",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 1
karta_workload_component_pods{component="replicatedjob",component_instance="prefill",namespace="team-a",phase="Succeeded",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_component_pods{component="replicatedjob",component_instance="prefill",namespace="team-a",phase="Unknown",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
# HELP karta_workload_component_replicas Desired replicas per component instance, from the workload spec.
# TYPE karta_workload_component_replicas gauge
karta_workload_component_replicas{component="replicatedjob",component_instance="decode",namespace="team-a",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 1
karta_workload_component_replicas{component="replicatedjob",component_instance="prefill",namespace="team-a",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 2
# HELP karta_workload_info Identity and provenance of a Karta-described workload. Value is always 1.
# TYPE karta_workload_info gauge
karta_workload_info{karta="jobset-x-k8s-io-jobset-v1alpha2",namespace="team-a",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet",workload_version="v1alpha2"} 1
# HELP karta_workload_status Normalized workload status. One series per phase; several phases can be 1 at the same time.
# TYPE karta_workload_status gauge
karta_workload_status{namespace="team-a",phase="Completed",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_status{namespace="team-a",phase="Degraded",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_status{namespace="team-a",phase="Failed",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_status{namespace="team-a",phase="Initializing",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_status{namespace="team-a",phase="Resuming",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_status{namespace="team-a",phase="Running",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 1
karta_workload_status{namespace="team-a",phase="Suspended",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_status{namespace="team-a",phase="Suspending",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
karta_workload_status{namespace="team-a",phase="Undefined",workload="llm",workload_group="jobset.x-k8s.io",workload_kind="JobSet"} 0
`

func fixtureStore() *store.Store {
	s := store.New()
	ref := store.WorkloadRef{Namespace: "team-a", Name: "llm", Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}

	s.UpsertWorkload(store.WorkloadRecord{
		UID:       "w1",
		Ref:       ref,
		Karta:     "jobset-x-k8s-io-jobset-v1alpha2",
		HasStatus: true,
		Phases:    []v1alpha1.ResourceStatus{v1alpha1.RunningStatus},
		Components: []store.ComponentState{
			{Component: "replicatedjob", Instance: "prefill", Replicas: ptr.To(int32(2))},
			{Component: "replicatedjob", Instance: "decode", Replicas: ptr.To(int32(1))},
		},
	})
	s.UpsertPod(store.PodRecord{
		UID: "p1", Namespace: "team-a", Name: "prefill-0", WorkloadUID: "w1", Workload: ref,
		Component: "replicatedjob", Instance: "prefill", Phase: corev1.PodRunning,
	})
	s.UpsertPod(store.PodRecord{
		UID: "p2", Namespace: "team-a", Name: "decode-0", WorkloadUID: "w1", Workload: ref,
		Component: "replicatedjob", Instance: "decode", Phase: corev1.PodPending,
	})
	return s
}

var _ = Describe("Collector", func() {
	contractFamilies := []string{
		MetricPodWorkloadInfo,
		MetricWorkloadInfo,
		MetricWorkloadStatus,
		MetricComponentReplicas,
		MetricComponentPods,
	}

	It("renders the exact metric contract from a snapshot", func() {
		c := New(fixtureStore(), Options{})

		Expect(testutil.CollectAndCompare(c, strings.NewReader(goldenContract), contractFamilies...)).To(Succeed())
	})

	It("passes promlint", func() {
		c := New(fixtureStore(), Options{
			PendingPods: func() int { return 1 },
			KartaCounts: func() (int, int, int) { return 1, 0, 0 },
		})

		problems, err := testutil.CollectAndLint(c)
		Expect(err).NotTo(HaveOccurred())
		Expect(problems).To(BeEmpty())
	})

	It("omits status series for workloads without a StatusDefinition", func() {
		s := store.New()
		s.UpsertWorkload(store.WorkloadRecord{
			UID: "w9",
			Ref: store.WorkloadRef{Namespace: "team-a", Name: "t", Group: "example.io", Version: "v1", Kind: "Thing"},
		})
		c := New(s, Options{})

		count := testutil.CollectAndCount(c, MetricWorkloadStatus)
		Expect(count).To(BeZero())
	})

	It("counts unattributed pods by reason", func() {
		s := fixtureStore()
		ref := store.WorkloadRef{Namespace: "team-a", Name: "llm", Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"}
		s.UpsertPod(store.PodRecord{
			UID: "p3", Namespace: "team-a", Name: "weird-0", WorkloadUID: "w1", Workload: ref,
			Component: "replicatedjob", Instance: SentinelUnknown, Reason: ReasonUnknownInstance, Phase: corev1.PodRunning,
		})
		c := New(s, Options{
			PendingPods: func() int { return 2 },
			KartaCounts: func() (int, int, int) { return 1, 1, 0 },
		})

		expected := `# HELP karta_exporter_unattributed_pods Pods the exporter could not fully attribute, by reason.
# TYPE karta_exporter_unattributed_pods gauge
karta_exporter_unattributed_pods{reason="jq_error"} 0
karta_exporter_unattributed_pods{reason="no_owner"} 2
karta_exporter_unattributed_pods{reason="unknown_instance"} 1
`
		Expect(testutil.CollectAndCompare(c, strings.NewReader(expected), MetricExporterUnattributedPods)).To(Succeed())
	})
})
