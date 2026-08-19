<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta Metrics Exporter

The exporter makes existing per-pod metrics addressable at workload and
component scope. It watches Karta CRs, the workloads they describe, and
pods, and publishes identity and state series. Recording rules shipped in
the same chart join those series with DCGM and cadvisor telemetry to
produce aggregated `karta:*` series.

The exporter holds no telemetry. It never scrapes DCGM, never queries
Prometheus, and never sees a GPU number. GPU values keep the timestamps and
staleness behavior of their original source.

## Enabling

```bash
helm upgrade --install karta charts/karta \
  --set exporter.enabled=true \
  --set exporter.serviceMonitor.enabled=true
```

The recording rules (a PrometheusRule) are rendered by default whenever the
exporter is enabled and the prometheus-operator CRDs are installed.

## Metric contract

Labels are additive-only: a released metric never loses or renames a label.
Every workload-scoped series carries `namespace`, `workload`,
`workload_kind`, and `workload_group`.

| Metric | Labels beyond the common set | Meaning |
| --- | --- | --- |
| `karta_pod_workload_info` | `pod`, `uid`, `component`, `component_instance`, `replica` | One series per attributed pod, value 1. The attribution primitive. |
| `karta_workload_info` | `workload_version`, `karta` | Identity and provenance, value 1. `workload_version` lives only here so a CRD storage-version bump does not split dashboards. |
| `karta_workload_status` | `phase` | One 0/1 series per normalized phase. All phases are always present; several can be 1 at once (for example Running and Degraded). Absent entirely when the Karta has no StatusDefinition. |
| `karta_workload_component_replicas` | `component`, `component_instance` | Desired replicas from the workload spec. |
| `karta_workload_component_pods` | `component`, `component_instance`, `phase` | Observed pod count per pod phase, zero-filled across all five phases. |

Sentinel values: `component="<unknown>"` means component inference failed
for that pod; `component_instance="<unknown>"` means the instance id did
not match any declared instance. Sentinel pods stay in workload-level
aggregates and are excluded from component-level rules. A plain empty
`component_instance=""` is a legitimate single-instance component, not a
failure.

Recorded aggregate names (part of the same contract):

```text
karta:gpu_utilization:{workload|component|component_instance}
karta:gpu_memory_used_bytes:{workload|component|component_instance}
karta:gpu_memory_total_bytes:{workload|component|component_instance}
karta:cpu_usage_cores:{workload|component|component_instance}
karta:memory_working_set_bytes:{workload|component|component_instance}
karta:join_coverage:ratio
```

GPU memory "total" is FB_USED plus FB_FREE, since DCGM's FB_TOTAL counter
is not in dcgm-exporter's default counter list. CPU and memory come from
cadvisor: metrics-server is an API rather than a Prometheus exposition
source, and node exporter is node-granular and cannot be attributed per
pod.

Self-observability series (not part of the consumer contract):
`karta_exporter_kartas`, `karta_exporter_workloads`,
`karta_exporter_unattributed_pods`, `karta_exporter_attribution_errors_total`,
`karta_exporter_last_event_timestamp_seconds`.

## Example queries

Component-instance imbalance (prefill vs decode):

```promql
karta:gpu_utilization:component_instance{workload="my-jobset"}
```

Idle workload detection (the consumer defines the threshold and window,
the exporter does not):

```promql
max_over_time(karta:gpu_utilization:workload[15m]) < 5
```

Time in phase:

```promql
avg_over_time(karta_workload_status{phase="Running"}[1h])
```

Under-replicated components:

```promql
karta_workload_component_replicas
  - on (namespace, workload, workload_group, workload_kind, component, component_instance)
    karta_workload_component_pods{phase="Running"} > 0
```

## Without recording rules

Consumers that cannot load rules still get identity, status, replica, and
pod-count series; only the `karta:*` aggregate names are absent. The raw
join behind the GPU utilization rule, usable directly in a dashboard:

```promql
avg by (namespace, workload, workload_kind, workload_group, component_instance) (
  DCGM_FI_DEV_GPU_UTIL
  * on (namespace, pod) group_left (workload, workload_kind, workload_group, component_instance)
  max by (namespace, pod, workload, workload_kind, workload_group, component_instance)
    (karta_pod_workload_info)
)
```

## Attribution health

`karta:join_coverage:ratio` is the fraction of GPU series that join the
identity series. Alert when it drops below 1: it means a label mismatch
between the telemetry source and the exporter, most often a Prometheus
relabeling scheme that renamed `pod` to `exported_pod` (honor_labels). The
source metric names and join labels are Helm values
(`exporter.prometheusRule.*`), so a mismatch is fixed by a values change,
not a release.

`karta_exporter_unattributed_pods{reason}` shows pods the exporter could
not fully attribute: `no_owner` (the owner chain dead-ends; usually a kind
missing from the Karta's `additionalChildKinds`), `unknown_instance`, and
`jq_error`.

## RBAC for custom Kartas

The exporter's ClusterRole enumerates resources; there are no wildcards.
The bound role aggregates every ClusterRole labeled
`karta.run.ai/exporter-rbac: "true"`. To grant access for a custom Karta:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: karta-exporter-mykind
  labels:
    karta.run.ai/exporter-rbac: "true"
rules:
  - apiGroups: ["example.io"]
    resources: ["mykinds"]
    verbs: ["get", "list", "watch"]
```

Default readable kinds are in `exporter.rbac.workloadRules` in values.yaml.

## Scoping and behavior notes

- The exporter serves only workloads whose Karta CR is applied to the
  cluster. There is no built-in catalog fallback; the catalog YAMLs under
  docs/catalog are what admins apply.
- One Karta serves each (group, kind). When two Kartas target the same
  kind, the oldest wins and the others surface as
  `karta_exporter_kartas{valid="false", reason="shadowed"}`.
- The pod cache is trimmed to metadata, `spec.nodeName`, and
  `status.phase`. A custom Karta whose pod selectors read other pod fields
  needs `exporter.fullPodCache=true`; without it, its pods surface only as
  unattributed.
- `/readyz` stays false until all informers sync, so a restarting exporter
  is a visible scrape gap, never plausible zeros.
- The metrics endpoint serves cluster-wide workload identity and is
  unauthenticated. Enable `exporter.networkPolicy.enabled` to restrict
  ingress; authenticated serving is a planned follow-up.

## Cardinality

At 5000 workloads / 50000 pods / around 3 component instances per
workload, the exporter emits about 190k series, comparable to a mid-size
kube-state-metrics install. The levers when that is too much: a namespace
allowlist on the scrape config, and dropping the `uid` or `replica` labels
with `metric_relabel_configs`. The pod-count family is the largest single
contributor.
