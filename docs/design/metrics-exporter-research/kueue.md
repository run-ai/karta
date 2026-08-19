<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# kueue: metrics in a generic multi-CRD workload layer

## TL;DR

- Kueue abstracts heterogeneous job CRDs with an imperative Go adapter per type: a required `GenericJob` interface plus roughly 15 optional capability interfaces, registered through an `IntegrationCallbacks` registry. Each adapter is a full Go package (roughly 900 to 7500 lines).
- Metrics split into two families: event-driven counters and histograms recorded at state transitions, and gauges recomputed from an in-memory cache every time the cache mutates. There are no polling timers for gauges.
- Cardinality is capped at the queue level. No metric carries a workload or pod name label. Per-workload visibility is served by a separate on-demand aggregated API server, not by time series.
- Lifecycle state is exposed as a one-hot gauge: for each object, every possible status gets a series and exactly one is set to 1.
- Stale series hygiene is explicit and pervasive: every gauge is tracked in cleanup scopes and `DeletePartialMatch` is called on object deletion, label change, and HA role change.

## How it works

### Job abstraction: interface plus optional capabilities

`pkg/controller/jobframework/interface.go` defines `GenericJob` (Object, IsSuspended, Suspend, RunWithPodSetsInfo, Finished, PodSets, IsActive, PodsReady, GVK) and a long list of optional interfaces (`JobWithPodLabelSelector`, `JobWithReclaimablePods`, `ComposableJob`, `TopLevelJob`, and so on). The single generic reconciler type-asserts for each capability. `pkg/controller/jobframework/integrationmanager.go` holds the registry: each integration provides `NewJob`, `NewReconciler`, `SetupWebhook`, `JobType`, indexes, and scheme setup via `IntegrationCallbacks`, keyed by name and GVK. The cost shows in `pkg/controller/jobs/`: 16 integration packages, each hand-written Go (batch Job alone is about 7500 lines including tests, LeaderWorkerSet about 5200). This is the per-CRD adapter tax that Karta's declarative descriptor avoids.

### Metric definitions and registration

All metric vectors live in one package, `pkg/metrics/metrics.go`. They are created in `InitMetricVectors(extraLabels []string)` so operator-configured extra labels can be appended to every vector at startup. `Register()` (line 1388) registers everything into the controller-runtime global registry (`sigs.k8s.io/controller-runtime/pkg/metrics`). Serving is the standard controller-runtime metrics server configured in `cmd/kueue/main.go` (line 185): `metricsserver.Options` with `SecureServing`, `FilterProvider: filters.WithAuthenticationAndAuthorization`, and TLS cert watching. No custom HTTP server.

### Event-driven vs recomputed-from-state

Counters and histograms (`admitted_workloads_total`, `evicted_workloads_total`, `quota_reserved_wait_time_seconds`, `workload_creation_latency_seconds`) are recorded at transition points in the scheduler and jobframework reconciler (`pkg/controller/jobframework/reconciler.go` line 1655 records creation latency). Gauges (`pending_workloads`, `reserving_active_workloads`, `cluster_queue_resource_usage`, quota gauges) are set from Kueue's in-memory cache whenever it changes: `pkg/cache/scheduler/clusterqueue.go` has `reportActiveWorkloads()` and `reportResourceMetrics()` called from add/delete/update paths, and `pkg/cache/queue/metrics.go` recomputes pending counts per ClusterQueue and LocalQueue. Freshness therefore tracks the informer cache, not a scrape or poll interval.

### Cardinality decisions

Labels are queue-scoped: `cluster_queue`, or `name` plus `namespace` for LocalQueue, plus low-cardinality dimensions (`priority_class`, `flavor`, `resource`, `reason`, `status`). Nothing is labeled per workload or per pod; workload identity is aggregated away into counts and histograms. The namespace-cardinality LocalQueue metrics are double-gated: a `LocalQueueMetrics` feature gate and a config block with an opt-in `LocalQueueSelector` label selector (`pkg/metrics/localqueue.go`, `apis/config/v1beta2/configuration_types.go` line 221). For per-workload questions (which workloads are pending and why), Kueue ships a separate visibility extension API server (`pkg/visibility/server.go`) that answers on demand instead of exporting high-cardinality series.

### Lifecycle state as time series

`ReportClusterQueueStatus` (`pkg/metrics/metrics.go` line 1129) iterates all statuses (`pending`, `active`, `terminating`) and sets exactly one series to 1, the rest to 0. `ReportLocalQueueStatus` does the same over condition statuses. This one-hot encoding makes `max by (...) (metric == 1)` and alerting trivial. `cohort_info` and `cluster_queue_info` are value-1 info metrics carrying hierarchy labels (`parent_cohort`, `root_cohort`) intended for PromQL joins rather than duplicating hierarchy labels on every metric.

### Series lifecycle management

Every gauge vector is registered into cleanup scopes via `trackGaugeVec` (metrics.go line 319). Deleting a ClusterQueue, LocalQueue, or Cohort calls `ClearClusterQueueMetrics` and friends, which `DeletePartialMatch` all series for that object, including counters and histograms. HA adds a `replica_role` label (`leader`, `follower`, `standalone` from `pkg/util/roletracker/tracker.go`) and `ClearGaugeMetricsForRole` wipes stale series on role transitions. `pkg/metrics/custom_labels.go` maps object labels or annotations into extra `custom_<name>` metric labels per operator config, with a per-object store that detects value changes so old series can be cleared before re-reporting.

## Relevance to Karta

Kueue is the closest OSS analog to Karta's problem shape: one controller over many workload CRDs, needing to emit workload-level telemetry without knowing each CRD natively. Its answer to heterogeneity (compiled Go adapters) is the approach Karta explicitly rejects, and the size of `pkg/controller/jobs/` is the strongest evidence for Karta's declarative bet. Its metrics architecture, however, is directly reusable: the Karta Metrics Exporter will face the same questions (event vs state metrics, per-workload cardinality, stale series on workload deletion, phase-as-time-series), and Kueue has mature, battle-tested answers for each. The one place Karta must diverge is cardinality: Karta's whole point is workload and component granularity, which Kueue deliberately refuses to put into Prometheus labels.

## Evidence

- `pkg/controller/jobframework/interface.go` - `GenericJob` required interface plus optional capability interfaces per job type. https://github.com/kubernetes-sigs/kueue/blob/main/pkg/controller/jobframework/interface.go
- `pkg/controller/jobframework/integrationmanager.go` - `IntegrationCallbacks` registry keyed by name and GVK; mandatory NewReconciler, SetupWebhook, JobType. https://github.com/kubernetes-sigs/kueue/blob/main/pkg/controller/jobframework/integrationmanager.go
- `pkg/controller/jobs/` - 16 per-CRD adapter packages, roughly 900 to 7500 lines each; the cost of the imperative approach. https://github.com/kubernetes-sigs/kueue/tree/main/pkg/controller/jobs
- `pkg/metrics/metrics.go` - all metric vectors; `InitMetricVectors(extraLabels)`, one-hot `ReportClusterQueueStatus`, `trackGaugeVec` cleanup scopes, `Register()` into controller-runtime registry, `Clear*` functions using `DeletePartialMatch`. https://github.com/kubernetes-sigs/kueue/blob/main/pkg/metrics/metrics.go
- `pkg/metrics/custom_labels.go` - operator-configured extra labels sourced from object labels or annotations, with change detection for stale series cleanup. https://github.com/kubernetes-sigs/kueue/blob/main/pkg/metrics/custom_labels.go
- `pkg/metrics/localqueue.go` - opt-in gating of namespace-cardinality LocalQueue metrics via feature gate plus label selector. https://github.com/kubernetes-sigs/kueue/blob/main/pkg/metrics/localqueue.go
- `apis/config/v1beta2/configuration_types.go` - `LocalQueueMetrics` config block with `Enable` and `LocalQueueSelector`. https://github.com/kubernetes-sigs/kueue/blob/main/apis/config/v1beta2/configuration_types.go
- `pkg/cache/scheduler/clusterqueue.go` - `reportActiveWorkloads` and `reportResourceMetrics`: gauges recomputed from the in-memory cache on every mutation, no polling. https://github.com/kubernetes-sigs/kueue/blob/main/pkg/cache/scheduler/clusterqueue.go
- `pkg/cache/queue/metrics.go` - pending workload gauges recomputed per ClusterQueue and LocalQueue, with LQ opt-in checks. https://github.com/kubernetes-sigs/kueue/blob/main/pkg/cache/queue/metrics.go
- `cmd/kueue/main.go` - controller-runtime `metricsserver.Options` with authn/authz filter and TLS; `metrics.Register()` at startup. https://github.com/kubernetes-sigs/kueue/blob/main/cmd/kueue/main.go
- `pkg/visibility/server.go` - separate aggregated API server for per-workload on-demand queries instead of high-cardinality metrics. https://github.com/kubernetes-sigs/kueue/blob/main/pkg/visibility/server.go
- `pkg/util/roletracker/tracker.go` - `replica_role` label values for HA replicas; stale series cleared on role transition. https://github.com/kubernetes-sigs/kueue/blob/main/pkg/util/roletracker/tracker.go

## Lessons for Karta

- Split metrics the way Kueue does: counters and histograms fire on lifecycle transitions (phase changes, replica scale events); gauges (replica counts, phase state, attributed usage) are recomputed from cached cluster state on change. For Karta's 30s-or-finer resolution, informer-driven recompute already beats any poll loop for state metrics; only the usage attribution path (DCGM, metrics-server) needs a scrape or query interval.
- Emit workload phase as a one-hot gauge exactly like `kueue_cluster_queue_status`: one series per normalized phase per workload, current phase set to 1. Enumerating all phases (not just the current one) makes transitions visible and queries simple.
- Treat series deletion as a first-class requirement from day one. Kueue's `trackGaugeVec` scopes plus `DeletePartialMatch` on object deletion is the pattern: when a workload or component instance disappears, its series must be deleted, not left frozen at the last value.
- Use value-1 info metrics for hierarchy (`karta_workload_info` with owner, kind, component labels), and keep hot metrics' label sets small; let PromQL joins attach hierarchy, as `cohort_info` and `cluster_queue_info` do.
- Make high-cardinality tiers opt-in with a selector, as Kueue does for LocalQueue metrics: workload-level on by default, component-instance-level behind config plus a selector, so operators control the series budget.
- Registering into the controller-runtime registry and reusing its authenticated metrics server is enough; Kueue ships dozens of vectors this way with no custom HTTP plumbing.
- The `CustomLabels` mechanism (config maps object labels/annotations to extra metric labels, with change detection and cleanup) is a cheap, high-value feature for tenant or team attribution that Karta can copy nearly verbatim.

## What NOT to copy

- The per-CRD adapter model itself. Sixteen hand-written Go packages, each with its own reconciler, webhook, and tests, is exactly the maintenance load Karta's declarative CRD exists to eliminate. Kueue needs imperative hooks because it mutates jobs (suspend, inject affinity); Karta's exporter only reads, so declarative selectors suffice.
- The refusal to label per workload. Kueue's queue-level cardinality cap is right for a scheduler admitting thousands of short jobs, but it is the opposite of Karta's product goal. Karta should instead borrow the mitigations (opt-in tiers, aggressive series deletion, info-metric joins) while accepting workload-level labels.
- Solving per-object visibility with an aggregated API server. Kueue's visibility server is a heavyweight answer (own apiserver, own API group, TLS, RBAC) to a problem Karta already solves with CRD status; do not add one for metrics.
- The optional-interface sprawl. Fifteen-plus type-asserted capability interfaces on `GenericJob` show how an interface-based abstraction accretes special cases per framework; a declarative schema keeps those variations in data, not code.
- Appending custom labels by re-initializing every metric vector at startup (`InitMetricVectors` called twice, from `init()` and again from `NewCustomLabels`) is fragile ordering; if Karta adopts custom labels, build the label list once before any vector is created.
