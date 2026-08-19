// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package collector

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// Metric names exposed to consumers. These names, together with the label
// names below, are Karta's public metric contract: additive-only changes.
const (
	MetricPodWorkloadInfo   = "karta_pod_workload_info"
	MetricWorkloadInfo      = "karta_workload_info"
	MetricWorkloadStatus    = "karta_workload_status"
	MetricComponentReplicas = "karta_workload_component_replicas"
	MetricComponentPods     = "karta_workload_component_pods"
)

// Self-observability metric names. Not part of the consumer contract.
const (
	MetricExporterKartas            = "karta_exporter_kartas"
	MetricExporterWorkloads         = "karta_exporter_workloads"
	MetricExporterUnattributedPods  = "karta_exporter_unattributed_pods"
	MetricExporterAttributionErrors = "karta_exporter_attribution_errors_total"
	MetricExporterLastEventSeconds  = "karta_exporter_last_event_timestamp_seconds"
)

// Label names.
const (
	LabelNamespace         = "namespace"
	LabelPod               = "pod"
	LabelUID               = "uid"
	LabelWorkload          = "workload"
	LabelWorkloadKind      = "workload_kind"
	LabelWorkloadGroup     = "workload_group"
	LabelWorkloadVersion   = "workload_version"
	LabelKarta             = "karta"
	LabelComponent         = "component"
	LabelComponentInstance = "component_instance"
	LabelReplica           = "replica"
	LabelPhase             = "phase"
	LabelValid             = "valid"
	LabelReason            = "reason"
)

// SentinelUnknown marks a component or component instance the exporter could
// not resolve. Angle brackets cannot collide with a Kubernetes name.
const SentinelUnknown = "<unknown>"

// Reason label values for self-observability metrics.
const (
	ReasonNoOwner            = "no_owner"
	ReasonUnknownInstance    = "unknown_instance"
	ReasonJQError            = "jq_error"
	ReasonStatusEval         = "status_eval"
	ReasonInvalid            = "invalid"
	ReasonShadowed           = "shadowed"
	ReasonNoStatusDefinition = "no_status_definition"
)

// AllPhases is the closed set of normalized statuses. Every workload with a
// StatusDefinition gets one 0/1 series per entry, so time-in-phase queries
// never break on a missing series. Undefined is the library fallback when
// mappings are present but nothing matched.
var AllPhases = []v1alpha1.ResourceStatus{
	v1alpha1.InitializingStatus,
	v1alpha1.RunningStatus,
	v1alpha1.CompletedStatus,
	v1alpha1.FailedStatus,
	v1alpha1.DegradedStatus,
	v1alpha1.SuspendedStatus,
	v1alpha1.SuspendingStatus,
	v1alpha1.ResumingStatus,
	v1alpha1.UndefinedStatus,
}

// AllPodPhases is the closed set of pod phases used by the component pod
// count series.
var AllPodPhases = []corev1.PodPhase{
	corev1.PodPending,
	corev1.PodRunning,
	corev1.PodSucceeded,
	corev1.PodFailed,
	corev1.PodUnknown,
}
