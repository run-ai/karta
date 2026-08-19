// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package collector

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/types"

	"github.com/run-ai/karta/exporter/pkg/store"
)

var workloadLabels = []string{LabelNamespace, LabelWorkload, LabelWorkloadKind, LabelWorkloadGroup}

// Options wires the collector to state owned by other exporter parts.
type Options struct {
	// PendingPods returns the number of pods parked on a missing owner.
	PendingPods func() int
	// KartaCounts returns valid, invalid, and shadowed Karta counts.
	KartaCounts func() (valid, invalid, shadowed int)
}

// Collector renders a store snapshot as const metrics on every scrape.
// Nothing is computed per object at scrape time beyond label assembly, and
// deleted objects simply stop being rendered.
type Collector struct {
	store   *store.Store
	options Options

	podInfoDesc           *prometheus.Desc
	workloadInfoDesc      *prometheus.Desc
	workloadStatusDesc    *prometheus.Desc
	componentReplicasDesc *prometheus.Desc
	componentPodsDesc     *prometheus.Desc
	workloadsDesc         *prometheus.Desc
	unattributedDesc      *prometheus.Desc
	kartasDesc            *prometheus.Desc
}

func New(s *store.Store, options Options) *Collector {
	return &Collector{
		store:   s,
		options: options,
		podInfoDesc: prometheus.NewDesc(MetricPodWorkloadInfo,
			"Pod to workload, component, and component instance attribution derived from the Karta definition. Value is always 1.",
			[]string{LabelNamespace, LabelPod, LabelUID, LabelWorkload, LabelWorkloadKind, LabelWorkloadGroup, LabelComponent, LabelComponentInstance, LabelReplica}, nil),
		workloadInfoDesc: prometheus.NewDesc(MetricWorkloadInfo,
			"Identity and provenance of a Karta-described workload. Value is always 1.",
			append(append([]string{}, workloadLabels...), LabelWorkloadVersion, LabelKarta), nil),
		workloadStatusDesc: prometheus.NewDesc(MetricWorkloadStatus,
			"Normalized workload status. One series per phase; several phases can be 1 at the same time.",
			append(append([]string{}, workloadLabels...), LabelPhase), nil),
		componentReplicasDesc: prometheus.NewDesc(MetricComponentReplicas,
			"Desired replicas per component instance, from the workload spec.",
			append(append([]string{}, workloadLabels...), LabelComponent, LabelComponentInstance), nil),
		componentPodsDesc: prometheus.NewDesc(MetricComponentPods,
			"Observed pod count per component instance, broken down by pod phase.",
			append(append([]string{}, workloadLabels...), LabelComponent, LabelComponentInstance, LabelPhase), nil),
		workloadsDesc: prometheus.NewDesc(MetricExporterWorkloads,
			"Number of Karta-described workloads currently tracked.", nil, nil),
		unattributedDesc: prometheus.NewDesc(MetricExporterUnattributedPods,
			"Pods the exporter could not fully attribute, by reason.", []string{LabelReason}, nil),
		kartasDesc: prometheus.NewDesc(MetricExporterKartas,
			"Karta definitions known to the exporter, by validity and reason.", []string{LabelValid, LabelReason}, nil),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.podInfoDesc
	ch <- c.workloadInfoDesc
	ch <- c.workloadStatusDesc
	ch <- c.componentReplicasDesc
	ch <- c.componentPodsDesc
	ch <- c.workloadsDesc
	ch <- c.unattributedDesc
	ch <- c.kartasDesc
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	snapshot := c.store.Snapshot()

	for _, pod := range snapshot.Pods {
		ch <- prometheus.MustNewConstMetric(c.podInfoDesc, prometheus.GaugeValue, 1,
			pod.Namespace, pod.Name, string(pod.UID),
			pod.Workload.Name, pod.Workload.Kind, pod.Workload.Group,
			pod.Component, pod.Instance, pod.Replica)
	}

	for _, workload := range snapshot.Workloads {
		ref := workload.Ref
		ch <- prometheus.MustNewConstMetric(c.workloadInfoDesc, prometheus.GaugeValue, 1,
			ref.Namespace, ref.Name, ref.Kind, ref.Group, ref.Version, workload.Karta)

		if workload.HasStatus {
			matched := make(map[string]struct{}, len(workload.Phases))
			for _, phase := range workload.Phases {
				matched[string(phase)] = struct{}{}
			}
			for _, phase := range AllPhases {
				value := 0.0
				if _, ok := matched[string(phase)]; ok {
					value = 1.0
				}
				ch <- prometheus.MustNewConstMetric(c.workloadStatusDesc, prometheus.GaugeValue, value,
					ref.Namespace, ref.Name, ref.Kind, ref.Group, string(phase))
			}
		}

		for _, component := range workload.Components {
			if component.Replicas == nil {
				continue
			}
			ch <- prometheus.MustNewConstMetric(c.componentReplicasDesc, prometheus.GaugeValue, float64(*component.Replicas),
				ref.Namespace, ref.Name, ref.Kind, ref.Group, component.Component, component.Instance)
		}
	}

	c.collectComponentPods(ch, snapshot)
	c.collectSelf(ch, snapshot)
}

type componentKey struct {
	workloadUID types.UID
	component   string
	instance    string
}

// collectComponentPods emits pod counts per component instance and pod phase.
// The instance set is the union of instances declared by the workload spec
// and instances observed on pods, zero-filled across all pod phases so
// under-replication math never breaks on a missing series.
func (c *Collector) collectComponentPods(ch chan<- prometheus.Metric, snapshot store.Snapshot) {
	refs := make(map[types.UID]store.WorkloadRef, len(snapshot.Workloads))
	keys := make(map[componentKey]struct{})
	counts := make(map[componentKey]map[string]int)

	for _, workload := range snapshot.Workloads {
		refs[workload.UID] = workload.Ref
		for _, component := range workload.Components {
			keys[componentKey{workload.UID, component.Component, component.Instance}] = struct{}{}
		}
	}

	for _, pod := range snapshot.Pods {
		if _, ok := refs[pod.WorkloadUID]; !ok {
			refs[pod.WorkloadUID] = pod.Workload
		}
		key := componentKey{pod.WorkloadUID, pod.Component, pod.Instance}
		keys[key] = struct{}{}
		if counts[key] == nil {
			counts[key] = make(map[string]int)
		}
		counts[key][string(pod.Phase)]++
	}

	for key := range keys {
		ref := refs[key.workloadUID]
		for _, phase := range AllPodPhases {
			ch <- prometheus.MustNewConstMetric(c.componentPodsDesc, prometheus.GaugeValue,
				float64(counts[key][string(phase)]),
				ref.Namespace, ref.Name, ref.Kind, ref.Group, key.component, key.instance, string(phase))
		}
	}
}

func (c *Collector) collectSelf(ch chan<- prometheus.Metric, snapshot store.Snapshot) {
	ch <- prometheus.MustNewConstMetric(c.workloadsDesc, prometheus.GaugeValue, float64(len(snapshot.Workloads)))

	unknownInstance, jqErrors := 0, 0
	for _, pod := range snapshot.Pods {
		switch pod.Reason {
		case ReasonUnknownInstance:
			unknownInstance++
		case ReasonJQError:
			jqErrors++
		}
	}
	pending := 0
	if c.options.PendingPods != nil {
		pending = c.options.PendingPods()
	}
	ch <- prometheus.MustNewConstMetric(c.unattributedDesc, prometheus.GaugeValue, float64(pending), ReasonNoOwner)
	ch <- prometheus.MustNewConstMetric(c.unattributedDesc, prometheus.GaugeValue, float64(unknownInstance), ReasonUnknownInstance)
	ch <- prometheus.MustNewConstMetric(c.unattributedDesc, prometheus.GaugeValue, float64(jqErrors), ReasonJQError)

	if c.options.KartaCounts != nil {
		valid, invalid, shadowed := c.options.KartaCounts()
		ch <- prometheus.MustNewConstMetric(c.kartasDesc, prometheus.GaugeValue, float64(valid), "true", "")
		ch <- prometheus.MustNewConstMetric(c.kartasDesc, prometheus.GaugeValue, float64(invalid), "false", ReasonInvalid)
		ch <- prometheus.MustNewConstMetric(c.kartasDesc, prometheus.GaugeValue, float64(shadowed), "false", ReasonShadowed)
	}
}
