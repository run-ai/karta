<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Workload and component metric attribution: what the panel does, and what Karta should do

## Purpose and method

Question: how do OSS projects attribute per-pod metrics to workload and
component granularity, and re-expose them as Prometheus series? This feeds
the Karta Metrics Exporter design (PRD: GPU utilization and memory per
workload, per component, per component instance; normalized phase as a
time-series; replica counts; 30s freshness; no idleness metric).

Method: a panel of eight local repo checkouts under ~/workspace, one
deep-dive per repo, local filesystem only (three repos were cloned for this
study: dcgm-exporter, kube-state-metrics, prometheus-adapter). Every claim in
the per-repo reports cites a path that was actually read. Karta context comes
from the current checkout (pkg/api/runai/v1alpha1, pkg/resource,
pkg/instructions, pkg/catalog, operator/, charts/karta, ROADMAP.md).

## Comparison matrix

| Repo | Attribution mechanism | Where aggregation happens | Phase as time-series | Cardinality policy | Data path |
|---|---|---|---|---|---|
| dcgm-exporter | kubelet pod-resources API joins device to pod at scrape time | none (pod level only) | no | node-scoped informer, label allowlist | background sampling, scrape-time transform stages |
| kube-state-metrics | none (per-object only); `_info` join metrics for consumers | PromQL, by design ("no pre-computation") | yes, StateSet 0/1 per phase | uid on pods, allowlists for user labels, stability ladder | generate on watch events, serve bytes on scrape |
| kueue | per-CRD Go adapters (16 packages) | in-memory cache, queue level only | yes, one-hot status gauge | hard cap at queue level, opt-in tiers, DeletePartialMatch cleanup | informer-driven recompute on mutation |
| jobset | stamps identity labels on every child Job and pod at creation | in CR status (replicatedJobsStatus), never in metrics | no | two counters total | reconcile-time status aggregation |
| training-operator | v1 pod label triple (job-name, replica-type, replica-index); mpi-operator info-series join recipe | PromQL join documented, never done by the operator | no (a documented gap) | five counters, namespace+framework only | event counters plus CR status ints |
| lws | admission webhook stamps name, group-index, worker-index; role is positional | pushed to the application (leader aggregates) | no | zero workload metrics | none (labels only) |
| opencost | queries Prometheus for owner/selector series and joins in Go | in Go, served over HTTP API, not metrics | no | stale-gauge deletion, disableable metrics | historical PromQL fan-out; newer collector-source scrapes kubelet and dcgm directly at 30s |
| prometheus-adapter | config rule maps a Prometheus label to a Kubernetes resource | pushed down to PromQL (metricsQuery template) | no | caches identity only, never samples | on-demand PromQL per API request |

Convergent findings across the panel:

1. Identity lives on the pod. JobSet, LWS, and training-operator v1 all stamp
   workload/component/instance identity as pod labels at creation or
   admission. Karta's PodSelector model is the declarative generalization of
   exactly these label schemes, and the catalog already encodes them.
2. Nobody re-exports foreign telemetry. Not one panel member copies DCGM or
   cadvisor samples into new series. The ecosystem answer is an identity join:
   publish a value-1 info series and let PromQL `group_left` attach the
   labels (mpi-operator documents this verbatim; kube-state-metrics codifies
   it as "avoid pre-computation"; kueue uses info metrics for hierarchy).
3. Workload-level and component-level series do not exist today. JobSet,
   LWS, and training-operator all compute per-component counts every
   reconcile and bury them in CR status. This is the gap the PRD names, and
   it is real.
4. State metrics are generated on watch events, never at scrape time
   (kube-state-metrics stores pre-rendered bytes; kueue recomputes gauges on
   cache mutation). Phase is exposed as one 0/1 series per value from a
   closed list.
5. Stale series deletion is a first-class requirement (kueue
   DeletePartialMatch scopes, opencost seen-maps). Exporters that skip it
   leave frozen series after workload deletion.
6. Query-Prometheus-and-join is the expensive path. opencost built a whole
   subsystem (query batching, rate limiting, UID disambiguation, recording
   rule prerequisites) and still added a direct-scrape collector-source at
   30s because the TSDB round trip cannot cheaply hit fine resolution.

## Options, ranked

### Option 1 (recommended): attribution-index exporter plus shipped recording rules

What: the exporter watches Karta CRs, Karta-described workloads, and their
pods. It emits only first-party series:

- `karta_pod_info{...} 1` per attributed pod, carrying workload kind, name,
  namespace, component, component instance, and replica key. This is the
  attribution primitive.
- `karta_workload_status_phase` StateSet (one 0/1 series per normalized
  phase), `karta_workload_info`, `karta_component_spec_replicas`,
  `karta_component_status_pods{phase=...}`.
- Attribution health signals (unattributed pod count, definition errors).

GPU and CPU numbers (REQ-1 to REQ-4) come from PromQL joins against the
existing DCGM and cadvisor series. The chart ships recording rules so
consumers get stable, first-class aggregated series
(`karta:gpu_utilization:workload`, `:component`, `:component_instance`)
without writing joins themselves.

Who does it: mpi-operator documents the exact join
(docs/legacy-v1/user-guides/mpi.md in training-operator); kube-state-metrics
is the reference for the state-metric half (pkg/metrics_store,
internal/store/pod.go phase StateSet); opencost emits the same kind of helper
series (pkg/metrics/podmetrics.go kube_pod_owner); kueue proves the one-hot
phase gauge and series cleanup patterns (pkg/metrics/metrics.go).

Effort: low to medium. No telemetry ingestion, no source scraping, no
Prometheus client in the exporter. Karta's library already answers the pod to
component question (pkg/resource/pod_querier.go, pkg/instructions/pod.go);
the exporter adds the workload-to-pods listing and the metric surface.

Risk: the join depends on DCGM stamping correct pod labels (it does, via the
kubelet pod-resources API) and on consumers installing the recording rules
for the convenience series. Aggregated numbers are only as fresh as the
source exporters' scrape interval, which is the DCGM default of 30s, meeting
REQ-7.

Fit for Karta: strongest. It matches the repo's dependency-thin philosophy,
reuses the existing library exactly where it is strong, and follows the
ecosystem consensus instead of fighting it.

### Option 2: option 1 plus direct-scrape re-export (the opencost collector-source direction)

What: everything in option 1, plus the exporter discovers dcgm-exporter
endpoints and kubelet stats, scrapes them on its own 30s clock, joins
in-process against the attribution index, and re-emits aggregated
`karta_workload_gpu_utilization` style series itself.

Who does it: opencost modules/collector-source (scrape/dcgm.go,
scrape/statsummary.go, 30s default); dcgm-exporter's transform-stage
architecture shows how to keep enrichment out of collection.

Effort: high. Endpoint discovery, scrape client, per-signal degradation,
value staleness, series lifecycle for aggregated values.

Risk: duplicates telemetry, doubles the failure surface, and the panel shows
nobody does this as their primary design. Worth keeping as an explicit
follow-up layer if real consumers cannot run recording rules.

### Option 3: query-Prometheus-and-re-emit

What: the exporter fires PromQL at Prometheus, joins in Go, re-exposes
aggregated series (opencost's classic path, pkg/costmodel/allocation.go).

Why ranked low: hard runtime dependency on Prometheus, freshness bounded by
scrape plus query resolution, query load scales with workload count, and
opencost itself built collector-source to escape these costs at 30s. Wrong
starting point for a fresh design.

### Option 4: per-workload-type adapters

What: a Go adapter per workload CRD, kueue-style
(pkg/controller/jobframework, 16 packages of 900 to 7500 lines).

Why ranked last: it is the maintenance model Karta exists to eliminate. The
declarative catalog already covers 20 workload types with YAML-sized
definitions.

## Decision for Karta

Build option 1 as the v1 exporter, structured so option 2 can be added as a
layer later without reshaping the metric contract.

Mapped to the current code:

- New `exporter/` Go module mirroring `operator/` (the root module stays
  dependency-thin). controller-runtime for informers, prometheus/client_golang
  via the controller-runtime metrics registry.
- Karta discovery: cluster KartaList indexed by root GVK, the same pattern as
  operator/pkg/controller.go indexKartaByRootGVK, with pkg/catalog as the
  no-CR fallback if desired.
- Workload resolution: dynamic informers per root GVK;
  resource.NewComponentFactoryFromObject plus tree.Build for the desired
  view (components, instances, Scale.Replicas, normalized phases).
- Pod attribution: pod informer; owner-reference walk to the root workload
  (additionalChildKinds names the intermediate kinds and drives RBAC), then
  resource.NewPodQuerier, instructions.InferPodComponent,
  InferPodComponentInstance, ExtractReplicaKey. Cache StructureSummary per
  Karta and the factory per workload generation; only per-pod selector
  evaluation runs on pod events.
- Metric surface: generate on watch events, serve bytes on scrape
  (kube-state-metrics model). Phase StateSet iterates the closed
  ResourceStatus set; Karta's status matching is multi-valued, so the
  StateSet emits 1 for every matched status rather than forcing a collapse.
- Series hygiene: delete all series for a workload, component instance, or
  pod on deletion (kueue DeletePartialMatch pattern). Never fail a scrape on
  attribution errors; emit the failure signal instead (dcgm-exporter rule).
- Chart: Service, ServiceMonitor, recording rules file, new ClusterRole
  (pods plus workload kinds). None of these exist in charts/karta today.

One tension to resolve in the design doc, not averaged away: the PRD phrases
REQ-1 to REQ-3 as "the exporter exposes GPU utilization", while the panel
consensus is that the exporter should expose identity and let rules produce
the utilization series. Shipped recording rules satisfy the requirement (the
aggregated series exist in Prometheus under stable names), but this is a real
interface decision the design doc must state explicitly and defend, including
what happens for consumers without the Prometheus rule machinery.

## What NOT to do

- Do not ingest or proxy telemetry in v1. No DCGM scraping, no metrics-server
  polling, no Prometheus querying inside the exporter.
- Do not build a per-metric mapping config DSL. prometheus-adapter shows the
  authoring and silent-failure burden; kube-state-metrics froze
  CustomResourceState. Karta's CRD is the config.
- Do not write per-workload-type Go adapters for any part of the exporter.
- Do not compute idleness, ratios, or any derived judgment (REQ-9 and the
  kube-state-metrics no-pre-computation rule agree).
- Do not compute metrics at scrape time; generate on watch events.
- Do not put generated child resource names, revision hashes, or unbounded
  user strings in labels (lws KEP-849 documents the breakage; KSM allowlists
  user labels).
- Do not leave stale series behind on deletion, and do not fail scrapes on
  enrichment errors.
- Do not add an aggregated API server for per-workload visibility (kueue's
  heavyweight answer; CR status already covers it).
- Do not start with sharding or multi-cluster; workload-object counts are
  small and the PRD scopes v1 to single-cluster.

## Per-repo index

- dcgm-exporter: scrape-time transform pipeline, pod-resources join, never
  fail the scrape. ./metrics-exporter-research/dcgm-exporter.md
- kube-state-metrics: watch-event generation, byte serving, StateSet phases,
  info-metric joins, frozen path DSL. ./metrics-exporter-research/kube-state-metrics.md
- kueue: one-hot status gauges, series cleanup scopes, the per-CRD adapter
  tax. ./metrics-exporter-research/kueue.md
- jobset: identity stamped as labels at creation, per-component status never
  exported. ./metrics-exporter-research/jobset.md
- training-operator: the pod label triple, mpi-operator join recipe, v2
  retreat from metrics. ./metrics-exporter-research/training-operator.md
- lws: three identity layers, positional role encoding, zero workload
  metrics. ./metrics-exporter-research/lws.md
- opencost: query-and-join costs, collector-source escape hatch, stale gauge
  hygiene. ./metrics-exporter-research/opencost.md
- prometheus-adapter: label-join fragility, delegation to PromQL, identity-only
  caching. ./metrics-exporter-research/prometheus-adapter.md
