<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# jobset: component-instance identity via labels and status

## TL;DR

- JobSet solves component-instance identity purely with stamped metadata. The controller writes a fixed set of `jobset.sigs.k8s.io/*` keys as both labels and annotations on every child Job and on the Job's pod template, so every pod carries workload name, component name (replicatedjob-name), and instance index (job-index, job-global-index).
- The same identity keys drive everything downstream: status aggregation, headless service selectors, pod affinity for exclusive placement, and (for external tools) metric joins. There is one identity scheme, stamped once.
- Component-level rollups live in status, not in metrics. `status.replicatedJobsStatus` holds per-replicatedJob counts (ready, succeeded, failed, active, suspended), recomputed each reconcile by grouping child Jobs on the `replicatedjob-name` label.
- The operator's own Prometheus metrics are minimal: two counters (`jobset_failed_total`, `jobset_completed_total`) labeled only by jobset name and namespace. There is no per-replicatedJob or per-pod metric export; JobSet leaves that to scrapers that use the pod labels.
- Identity is duplicated as labels (selectable) and annotations (not length-limited in the same way, readable via downward API). Pods can consume their own identity, e.g. the coordinator endpoint via `fieldPath: metadata.labels[...]`.

## How it works

### Identity stamping (the component-instance scheme)

The well-known keys are constants in `api/jobset/v1alpha2/jobset_types.go`:

- `jobset.sigs.k8s.io/jobset-name`, `jobset.sigs.k8s.io/jobset-uid`: workload identity.
- `jobset.sigs.k8s.io/replicatedjob-name`: the component (e.g. `prefill` or `decode`).
- `jobset.sigs.k8s.io/job-index`: instance index within the replicatedJob, 0 to replicas-1.
- `jobset.sigs.k8s.io/job-global-index`: instance index unique across the whole JobSet.
- `jobset.sigs.k8s.io/replicatedjob-replicas` and `jobset.sigs.k8s.io/global-replicas`: expected cardinality, stamped on each child so a consumer can compute "N of M ready" without reading the parent spec.
- `jobset.sigs.k8s.io/job-key`: SHA256 of the namespaced job name, a single collision-free key for the instance.
- Group variants (`group-name`, `group-replicas`, `job-group-index`) for replicatedJobs grouped together.

`labelAndAnnotateObject` in `pkg/controllers/jobset_controller.go` writes all of these as both labels and annotations. It is called twice per child Job in `constructJob` (lines 949-950): once on the Job object and once on `job.Spec.Template`, so every pod is born with the full identity set. No webhook or watch is needed to attribute a pod; the identity is immutable metadata on the pod itself.

The same labels are reused as selectors elsewhere: the headless service selects on `jobset-name` (line 813-814 of `pkg/controllers/jobset_controller.go`), and the exclusive-placement pod webhook builds pod affinity/anti-affinity terms on the `job-key` label (`pkg/webhooks/pod_webhook.go`, around lines 108-131).

### Per-component status aggregation

`JobSetStatus.ReplicatedJobsStatus` is a list-map keyed by replicatedJob name (`api/jobset/v1alpha2/jobset_types.go`, lines 253-289) with counts: Ready, Succeeded, Failed, Active, Suspended, plus per-instance JobRestarts arrays indexed by job index.

`calculateReplicatedJobStatuses` (`pkg/controllers/jobset_controller.go`, lines 456-560) computes it by iterating owned child Jobs and bucketing on `job.Labels[jobset.ReplicatedJobNameKey]`. Notable semantics:

- "Ready" is derived, not copied: a child Job counts as ready when `job.Status.Succeeded + job.Status.Ready >= min(parallelism, completions)`. So readiness is defined against the expected pod count of the instance.
- Jobs missing the label, or labeled with an unknown replicatedJob name, are logged and skipped, never misattributed.
- The status update is skipped when semantically equal (`updateReplicatedJobsStatuses`, lines 445-453), avoiding write churn.

### Operator metrics

`pkg/metrics/metrics.go` defines exactly two CounterVecs, `jobset_failed_total` and `jobset_completed_total`, labeled `{jobset_name, namespace}`, registered into the controller-runtime registry. They are incremented at terminal-state transitions (`pkg/controllers/failure_policy.go` line 422, `pkg/controllers/jobset_controller.go` line 1279). There is no gauge mirroring `replicatedJobsStatus` and no per-pod re-export. `main.go` serves them on the standard controller-runtime endpoint with `SecureServing: true` and `FilterProvider: filters.WithAuthenticationAndAuthorization` (lines 179-188).

### Docs and monitoring examples

`site/content/en/docs/reference/metrics.md` documents the two custom counters plus the generic controller-runtime reconcile metrics. `site/static/examples/prometheus-operator/prometheus.yaml` ships a full Prometheus Operator setup (ServiceAccount, RBAC, Prometheus CR, ServiceMonitor) for scraping the operator; `config/prometheus/kustomization.yaml` wires the same into kustomize overlays. `site/content/en/docs/concepts/_index.md` documents the label scheme under "JobSet labels" and shows pods consuming their own labels via the downward API (`fieldPath: "metadata.labels['jobset.sigs.k8s.io/coordinator']"`). There is no doc on joining pod metrics to replicatedJobs; the labels make it possible but JobSet does not do it.

## Relevance to Karta

JobSet is the canonical producer side of the problem Karta's exporter wants to solve. The prefill/decode example maps directly: replicatedJob name is Karta's component, job-index (or job-global-index) is the component instance, and every pod already carries these as labels. A Karta Metrics Exporter attributing per-pod telemetry for a JobSet does not need to walk ownership chains at scrape time; it can resolve `pod -> (workload, component, instance)` from the pod's own labels. Karta's generic layer is the missing piece JobSet deliberately does not build: JobSet stamps identity and aggregates status, but never re-exposes per-component telemetry as Prometheus series. Karta's workload-type definition for JobSet would encode exactly these label keys as the component and instance selectors.

## Evidence

- `api/jobset/v1alpha2/jobset_types.go` - all `jobset.sigs.k8s.io/*` label/annotation key constants (lines 23-99) and the `ReplicatedJobStatus` per-component counts struct (lines 253-289). https://github.com/kubernetes-sigs/jobset/blob/main/api/jobset/v1alpha2/jobset_types.go
- `pkg/controllers/jobset_controller.go` - `labelAndAnnotateObject` stamps identity on Job and pod template (lines 1014-1079, called at 949-950); `calculateReplicatedJobStatuses` buckets child Jobs by the replicatedjob-name label and derives ready counts (lines 456-560); headless service selector on jobset-name (line 813). https://github.com/kubernetes-sigs/jobset/blob/main/pkg/controllers/jobset_controller.go
- `pkg/metrics/metrics.go` - the operator's entire Prometheus surface: two counters labeled jobset_name and namespace, registered in the controller-runtime registry. https://github.com/kubernetes-sigs/jobset/blob/main/pkg/metrics/metrics.go
- `pkg/controllers/failure_policy.go` - `metrics.JobSetFailed` increment at terminal failure (line 422). https://github.com/kubernetes-sigs/jobset/blob/main/pkg/controllers/failure_policy.go
- `main.go` - metrics endpoint setup with secure serving and authn/authz filter (lines 179-188), `metrics.Register()` (line 101). https://github.com/kubernetes-sigs/jobset/blob/main/main.go
- `pkg/webhooks/pod_webhook.go` - pod affinity terms built from the stamped job-key label, showing the identity labels double as scheduling selectors. https://github.com/kubernetes-sigs/jobset/blob/main/pkg/webhooks/pod_webhook.go
- `site/content/en/docs/concepts/_index.md` - public documentation of the label scheme and downward-API consumption of labels by pods. https://github.com/kubernetes-sigs/jobset/blob/main/site/content/en/docs/concepts/_index.md
- `site/content/en/docs/reference/metrics.md` - documented metric names, types, and labels. https://github.com/kubernetes-sigs/jobset/blob/main/site/content/en/docs/reference/metrics.md
- `site/static/examples/prometheus-operator/prometheus.yaml` - shipped Prometheus Operator scrape setup for the controller. https://github.com/kubernetes-sigs/jobset/blob/main/site/static/examples/prometheus-operator/prometheus.yaml
- `config/prometheus/kustomization.yaml` - kustomize wiring for the prometheus component. https://github.com/kubernetes-sigs/jobset/blob/main/config/prometheus/kustomization.yaml

## Lessons for Karta

- Prefer labels already on the pod over runtime joins. When the workload controller stamps component and instance identity at creation time, attribution becomes a pure label read. Karta's exporter should treat pod labels as the primary attribution source and define, per workload type, which keys mean component and instance.
- Publish cardinality alongside identity. Stamping `replicatedjob-replicas` and `global-replicas` on each child lets a consumer emit "ready fraction" style metrics without fetching the parent spec. Karta could surface expected instance counts the same way.
- Derive readiness against expected pod count, not raw pod phase. The `succeeded + ready >= min(parallelism, completions)` rule is a normalized per-instance readiness definition, exactly the kind of normalization Karta's phases aim for.
- Keep aggregation in status and identity in metadata; keep operator metrics low-cardinality. JobSet's split (rich per-component status, tiny metric surface) shows the aggregation logic can live in the controller while metric fan-out is left to a dedicated exporter, which is Karta's niche.
- Tolerate mislabeled children explicitly: log and skip rather than misattribute. An exporter attributing pods should do the same when a pod matches no component selector.
- Duplicate identity as labels and annotations. Labels are selectable, annotations survive value-length limits and feed the downward API.

## What NOT to copy

- Name-only metric labels: `jobset_name` without UID means a deleted and recreated JobSet continues the same counter series. Karta should include UID or another instance disambiguator, or document the aliasing.
- Counters that only tick at terminal states. `failed_total`/`completed_total` say nothing about in-flight per-component health; the useful counts exist in `replicatedJobsStatus` but are never exported. Karta's exporter exists precisely to close this gap; do not stop at terminal counters.
- String-keyed count maps (`map[string]map[string]int32` with "ready"/"failed" keys) in `calculateReplicatedJobStatuses`; a small struct is safer and cheaper.
- The label-set is append-only and flat; ten-plus keys per pod with the full set duplicated as annotations. Karta attributes existing workloads and cannot rely on adding metadata; the exporter must also handle workload types that stamp nothing and require selector-based attribution.
- Per-instance arrays in status indexed by job index (`JobRestarts []int32`, capped at 1024). This couples status size to replica count and breaks past the cap; fine for JobSet, wrong shape for a generic CRD.
