<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# kube-state-metrics: state-as-metrics exporter architecture

## TL;DR

- KSM never computes metrics at scrape time. It generates and renders metric text on informer watch events, keyed by object UID, so a scrape is a pure byte concatenation over a `sync.Map`.
- The store IS the cache: a custom `cache.Store` implementation holds pre-rendered `[][]byte` per object instead of the Kubernetes objects themselves. One reflector per resource (or per namespace) feeds it.
- Enum state (like `kube_pod_status_phase`) is exposed as one series per possible value with value 0/1, generated from a hardcoded list of all phases so absent states are explicit zeros.
- CustomResourceState is a declarative YAML mapping from CRD field paths to Gauge/StateSet/Info metrics, with `labelsFromPath`, `valueFrom`, and per-element expansion over arrays and maps. It is feature-frozen upstream, which says a lot about the limits of the approach.
- Raw state only, no pre-aggregation: KSM refuses to compute derived values (no `kube_pod_total`), leaving joins and rollups to PromQL. Desired vs actual replicas are just two separate gauges from spec and status.

## How it works

### Informer-backed metric stores, generation on watch events

The core design is documented in docs/design/metrics-store-performance-optimization.md: instead of caching full Kubernetes objects and computing series on scrape, KSM keeps `map[uid][]byte` of pre-rendered time series and updates it on add/update/delete events.

The implementation is pkg/metrics_store/metrics_store.go. `MetricsStore` implements the client-go `cache.Store` interface. `Add()` runs `generateMetricsFunc(obj)`, renders all families of the object into one contiguous byte buffer (`renderFamilies`, one allocation per object), and stores the per-family slices under the object UID in a `sync.Map`. `Delete()` removes the UID entry. `Update()` is just `Add()`. A reflector is wired directly to this store in internal/store/builder.go (`startReflector`, line 716), one per resource, or one per configured namespace (`buildStores`, line ~605).

Metric generation per resource is a list of `FamilyGenerator`s (name, help, type, stability level, generate func) composed into one function (internal/store/builder.go `buildStores` calls `generator.ComposeMetricGenFuncs`). Each generator carries a `StabilityLevel` and optional deprecation text (pkg/metric_generator/generator.go).

### Serving /metrics cheaply

pkg/metricshandler/metrics_handler.go `ServeHTTP` takes a read-lock snapshot of the writer list, negotiates text vs OpenMetrics format, optionally gzips, and calls `WriteAll` per writer. pkg/metrics_store/metrics_writer.go `WriteAll` writes the family header once, then ranges the `sync.Map` of every store and writes the pre-rendered bytes for that family index. No label formatting, no float printing, no allocation on the scrape path. Headers are precomputed per exposition format at store construction (`precomputeHeaders` in metrics_store.go) and deduplicated per scrape via a pooled seen-map (`SanitizeHeaders` in metrics_writer.go).

### State enums: the kube_pod_status_phase pattern

internal/store/pod.go `createPodStatusPhaseFamilyGenerator` (line 1502) builds a fixed array of all five pod phases and emits one metric per phase with label `phase="<name>"` and value `boolFloat64(phase == current)`. So every pod always exports 5 series, exactly one of which is 1. If `status.phase` is empty, it emits nothing rather than guessing. docs/design/metrics-best-practices.md codifies this: dynamic status properties belong in a StateSet keyed by object identity labels plus the state label.

### Replicas desired vs actual

There is no combined metric. internal/store/deployment.go exposes `kube_deployment_spec_replicas` (line 268, from spec) and `kube_deployment_status_replicas` / `_ready` / `_available` / `_unavailable` / `_updated` (lines 121-185, from status) as separate gauges sharing the same identity labels. The user divides or subtracts in PromQL.

### CustomResourceState: declarative CRD-to-metric mapping

Config types are pkg/customresourcestate/config.go and config_metrics_types.go. A `Resource` names a GroupVersionKind (wildcards `*` for version/kind resolve against installed CRDs), optional `metricNamePrefix`, resource-level `commonLabels` and `labelsFromPath`, and a list of `Generator`s. Each generator is a union of one of three types (`Metric` struct, config.go line 151):

- Gauge: `path` to a value, array, or object; arrays/objects expand to one metric per element; `valueFrom` picks the numeric field; `labelFromKey` turns a map key into a label; `NilIsZero` for unset fields.
- StateSet: `path` to a string field, `list` of possible values, `labelName` for the state label. Emits one 0/1 series per listed value, same shape as kube_pod_status_phase (docs/metrics/extend/customresourcestate-metrics.md, StateSet section).
- Info: value always 1, all content in labels, including `*` wildcard copying of whole label/annotation maps with optional prefixes.

Path syntax (same doc, Path Syntax section) is a list of segments with array indexing (`"0"`), list-element matching (`"[name=a]"`), and field-value guards. Compilation happens once at config load (pkg/customresourcestate/registry_factory.go `compile`/`compileFamily`), and the compiled family plugs into the exact same store/reflector machinery as built-in resources via `RegistryFactory` (internal/store/builder.go `WithCustomResourceStoreFactories`, line 213). Every CR metric is forcibly labeled with `customresource_group`, `customresource_version`, `customresource_kind` (registry_factory.go lines 40-46); those label names are reserved.

Gauge value conversion is generous: bools, "true"/"yes"/"no"/"unknown" strings, RFC3339 timestamps, Kubernetes quantities ("250m", "512Gi"), and percentages all convert to float64 (docs/metrics/extend/customresourcestate-metrics.md, type conversion section).

Limitations worth noting: no arithmetic or cross-object joins, no cross-field computation, only one value per path expansion, errors on missing paths are log noise you tune with `errorLogV`, and multiple versions of the same kind bleed into each other on list (doc, Wildcard Note section). Upstream froze the feature in favor of kubernetes-sigs/resource-state-metrics (README.md, Custom Resource State Metrics note).

### Label conventions, stability, churn control

- Every metric carries the object's identity labels; pods use `namespace`, `pod`, `uid` as default labels (internal/store/pod.go line 41). The uid disambiguates recreated same-name objects at the cost of guaranteed churn per recreation.
- Kubernetes labels/annotations are only exported on `_labels`/`_annotations` metrics behind explicit allowlists (`--metric-labels-allowlist`, internal/store/builder.go `WithAllowLabels`), because arbitrary user labels are a cardinality bomb. Sanitized names get a `label_` prefix and `_conflictN` suffixes on collisions (README.md, Conflict resolution in label names).
- Stability framework: each family generator declares EXPERIMENTAL/STABLE/DEPRECATED (docs/README.md Metrics Stages; pkg/metric_generator/generator.go). Stable metrics are frozen except for added labels, with a Major.Minor+2 deprecation window (docs/design/metrics-best-practices.md, Stability section).
- Best practices doc (docs/design/metrics-best-practices.md): static 1:1 properties go in one `_info` metric; 1:n properties get their own metric; unbounded strings (error messages) become bounded labels like `error="true"`; optional fields still export the label as an empty string.

### Sharding and performance

Horizontal sharding hashes the object UID (fnv64a in pkg/sharding/listwatch.go `keep`, line 211; README says md5, the code disagrees) modulo total shards, wrapping every ListerWatcher so each instance only stores its share. Every shard still receives and unmarshals all watch traffic; only memory is divided (README.md, Horizontal sharding). Autosharding derives the shard index from the StatefulSet pod ordinal via an informer on the StatefulSet itself and rebuilds all writers on resharding (pkg/metricshandler/metrics_handler.go `Run`/`ConfigureSharding`). For pods there is a special mode: a DaemonSet with `--node=$(spec.nodeName)` field selector so each node exports its own pods (README.md, Daemonset sharding for pod metrics).

## Relevance to Karta

The Karta Metrics Exporter has two distinct jobs, and KSM is the reference implementation for exactly one of them:

1. State metrics from the Karta CR (workload phase StateSet, desired vs actual replicas, workload/component info): this is precisely what KSM does for built-in resources, and what CustomResourceState almost does for CRDs. Karta watches its own CRD, so it can hardcode generators (the built-in-resource path, which is the good path in KSM) instead of the frozen generic config path.
2. Attribution of per-pod telemetry (DCGM, metrics-server) to workload/component: KSM deliberately does not do this. Its answer to attribution is the `_info` join metric: export `kube_pod_info` style identity metrics and let PromQL `group_left` joins attach workload labels to DCGM series. Karta must decide whether to follow that philosophy (export a `karta_component_pod_info{workload,component,component_instance,pod,namespace,uid} 1` join key and ship recording rules) or to break it and re-export aggregated copies of foreign metrics. KSM's "avoid pre-computation" rule (docs/design/metrics-best-practices.md) argues for the join-key approach as the primary interface, with any re-exported aggregates as an explicit opt-in layer on top.

The event-driven store also fits Karta's 30s-or-finer requirement: freshness is bounded by watch latency (seconds), not by a poll loop, and scrape cost is independent of cluster size math done at scrape time. But note KSM only ever converts one object to metrics in isolation. Karta attribution needs pod-to-workload correlation (selector evaluation), which KSM's per-object generator signature cannot express; Karta needs a keyed multi-informer store instead.

## Evidence

- `pkg/metrics_store/metrics_store.go` - cache.Store that stores pre-rendered metric bytes per object UID, generated on Add/Update; https://github.com/kubernetes/kube-state-metrics/blob/main/pkg/metrics_store/metrics_store.go
- `pkg/metrics_store/metrics_writer.go` - scrape path is byte concatenation; header dedup and info/stateset-to-gauge rewrite for text format; https://github.com/kubernetes/kube-state-metrics/blob/main/pkg/metrics_store/metrics_writer.go
- `pkg/metricshandler/metrics_handler.go` - /metrics handler, writer snapshot under RLock, autosharding from StatefulSet ordinal, writer rebuild on reshard; https://github.com/kubernetes/kube-state-metrics/blob/main/pkg/metricshandler/metrics_handler.go
- `internal/store/builder.go` - registry of per-resource store constructors, one reflector per store, per-namespace store fan-out, label/annotation allowlists; https://github.com/kubernetes/kube-state-metrics/blob/main/internal/store/builder.go
- `internal/store/pod.go` - `createPodStatusPhaseFamilyGenerator` (line 1502): one 0/1 series per phase from a fixed list; default labels namespace/pod/uid (line 41); https://github.com/kubernetes/kube-state-metrics/blob/main/internal/store/pod.go
- `internal/store/deployment.go` - `kube_deployment_spec_replicas` vs `kube_deployment_status_replicas*` as separate gauges; https://github.com/kubernetes/kube-state-metrics/blob/main/internal/store/deployment.go
- `pkg/customresourcestate/config.go` and `pkg/customresourcestate/config_metrics_types.go` - CustomResourceState config model: Resource, Generator, Gauge/StateSet/Info union, labelsFromPath; https://github.com/kubernetes/kube-state-metrics/blob/main/pkg/customresourcestate/config.go
- `pkg/customresourcestate/registry_factory.go` - config compiled once into families; reserved customresource_* GVK labels added to everything; https://github.com/kubernetes/kube-state-metrics/blob/main/pkg/customresourcestate/registry_factory.go
- `docs/metrics/extend/customresourcestate-metrics.md` - full config format, path syntax, type conversion, StateSet semantics, wildcard GVK limitations, feature-freeze note; https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/extend/customresourcestate-metrics.md
- `docs/design/metrics-store-performance-optimization.md` - the original design doc for event-time generation and the uid-keyed byte cache; https://github.com/kubernetes/kube-state-metrics/blob/main/docs/design/metrics-store-performance-optimization.md
- `docs/design/metrics-best-practices.md` - no pre-computation rule, info-metric pattern, StateSet for dynamic status, cardinality guidance, stability policy; https://github.com/kubernetes/kube-state-metrics/blob/main/docs/design/metrics-best-practices.md
- `pkg/sharding/listwatch.go` - fnv64a(UID) mod totalShards filtering wrapped around every ListerWatcher; https://github.com/kubernetes/kube-state-metrics/blob/main/pkg/sharding/listwatch.go
- `pkg/metric_generator/generator.go` - per-family StabilityLevel and deprecation plumbing; https://github.com/kubernetes/kube-state-metrics/blob/main/pkg/metric_generator/generator.go
- `README.md` - stability guarantees, label conflict handling, sharding modes, KSM vs metrics-server positioning; https://github.com/kubernetes/kube-state-metrics/blob/main/README.md
- `docs/README.md` - EXPERIMENTAL/STABLE/DEPRECATED stages and opt-in metric list; https://github.com/kubernetes/kube-state-metrics/blob/main/docs/README.md

## Lessons for Karta

- Generate metric text on watch events, serve bytes on scrape. This is the single most transferable pattern: at 30s scrape resolution with many workloads, per-scrape recomputation is where exporters die. Keying by object UID makes delete handling trivial.
- Copy the StateSet contract exactly for `karta_workload_status_phase`: iterate the full closed set of Karta normalized phases, emit one series per phase with 0/1, emit nothing when phase is unset. Consumers get glitch-free `min_over_time`/alerting semantics and the pattern is already familiar from kube_pod_status_phase.
- Split desired vs actual replicas into `karta_component_spec_replicas` and `karta_component_status_replicas` (plus ready/available variants as needed) with identical identity labels. Do not invent a ratio metric.
- Ship an info/join metric as the attribution primitive: a pod-to-component identity series lets DCGM and cAdvisor metrics be re-labeled in PromQL with `group_left` without Karta touching the telemetry at all. Aggregated re-exports can then be a second, optional feature rather than the foundation.
- Adopt the stability ladder day one: mark every family EXPERIMENTAL/STABLE, put deprecation notices in HELP text, and treat stable metric shape as API (frozen except added labels). KSM retrofitted this and pays for it in docs.
- Keep spec-derived labels on spec metrics and status-derived labels on status metrics, and never put unbounded user strings in labels; allowlist anything user-controlled (KSM's `--metric-labels-allowlist` pattern).
- Identity labels: include namespace/name always; think hard about uid. KSM includes uid on pod metrics for correctness across recreation, at the cost of guaranteed series churn. For workload-level metrics that survive pod churn (the whole point of Karta), uid on the workload only, not per-pod, keeps series stable.

## What NOT to copy

- CustomResourceState's generic path-expression config. It is feature-frozen upstream and being replaced. Karta owns its CRD schema, so hardcoded typed generators (the built-in-resource path) are simpler, faster, testable, and avoid the missing-path log-noise and type-coercion ambiguity that the YAML DSL suffers from.
- Per-object-in-isolation generation as the only model. KSM's generator signature `func(obj) []Family` cannot see two objects at once, which is why KSM cannot attribute pod metrics to owners beyond copying `owner_kind`/`owner_name` labels. Karta attribution requires correlating pods with Karta components; design the store around a keyed index (workload -> pods), not around KSM's one-object contract.
- The `sync.Map`-of-rendered-bytes store as-is for aggregated values. It works because KSM values are per-object and independent. Aggregations (sum of GPU util per component) invalidate on any member pod change; a naive copy would re-render whole groups on every pod event. Use dirty-marking per group if Karta aggregates.
- Shard-by-UID list/watch filtering, at least initially. Every shard still pays full watch bandwidth and decode CPU, and correctness depends on subtle continue-token and bookmark handling (pkg/sharding/listwatch.go). Karta's object counts (workloads, not pods) are orders of magnitude smaller; one replica plus the DaemonSet-per-node pattern for anything per-pod covers realistic scale.
- Serving everything on one endpoint with query-param resource filtering (`?resources=`). Prometheus scrape configs handle this poorly; separate jobs or ports per concern (state vs attributed telemetry) are cleaner.
- The README's md5-sharding claim. The code uses fnv64a (pkg/sharding/listwatch.go line 212). A reminder to generate docs from code or test them; Karta should treat exported metric names/labels as contract-tested artifacts.
