<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# training-operator: replica-type structure and job lifecycle metrics

Scope note: the local checkout is a depth-1 clone of current master, which is
the Kubeflow Trainer v2 codebase (the repo was renamed; the remote is still
kubeflow/training-operator). The v1 Go controllers are not in the tree.
V1 behavior below is grounded in the legacy documentation the project keeps
in-tree under `docs/legacy-v1/`. V2 behavior is grounded in the Go source.

## TL;DR

- V1 stamps every pod with a three-part identity label set:
  `training.kubeflow.org/job-name`, `.../replica-type`, `.../replica-index`
  (plus `.../operator-name`). That is exactly Karta's
  workload/component/component-instance triple, expressed as pod labels
  intended for selector-based joins.
- V1 job status is a per-replica-type map (`replicaStatuses.Worker.succeeded: 2`)
  plus a normalized condition ladder shared by all frameworks:
  Created, Running, Restarting, Succeeded, Failed. Old conditions stay in the
  array with `status: "False"`, so the lifecycle history is readable.
- The v1 operator's own Prometheus surface is deliberately tiny: five event
  counters (`training_operator_jobs_{created,deleted,successful,failed,restarted}_total`)
  labeled only by `namespace` and `framework`. No phase gauge, no
  per-replica-type series. Phase-as-timeseries is left to external tooling.
- The sibling mpi-operator documents the missing half: an info gauge
  (`mpi_operator_job_info{launcher=...,namespace=...}`) built to be joined
  with `kube_pod_info` via `label_replace` + `group_left`. That join recipe is
  the attribution mechanic Karta's exporter generalizes.
- V2 (current code) removed all custom metrics. Only controller-runtime
  defaults are served. Structure moved to JobSet (replicated job name + Job
  completion index), per-component counts moved to `status.jobsStatus`, and a
  new push-based status server writes training progress into the CRD status
  instead of Prometheus.

## How it works

### V1: replica identity as pod labels

Replica types are the keys of the replica-spec map in the v1 CRDs
(`pytorchReplicaSpecs.Master`, `.Worker`; TFJob adds `PS`; MPIJob adds
`Launcher`), shown in the migration guide's before/after example
(`docs/operator-guides/migration.md`). The operator labels each pod with the
job name, the owning controller, the replica type, and the replica index. The
legacy user guides use these labels as the primary addressing scheme:

- `docs/legacy-v1/user-guides/pytorch.md` line 38 selects the rank-0 pod with
  `training.kubeflow.org/job-name=...,training.kubeflow.org/replica-type=master,training.kubeflow.org/replica-index=0`.
- `docs/legacy-v1/user-guides/paddlepaddle.md` lines 112-114 show the full
  stamped set on a Service selector, including
  `training.kubeflow.org/operator-name: paddlejob-controller` and
  `training.kubeflow.org/replica-type: Worker`.
- `docs/legacy-v1/user-guides/jax.md` line 113 shows the same triple used as a
  generic selector string.

### V1: per-replica-type status and normalized conditions

`docs/legacy-v1/user-guides/tensorflow.md` (lines 280-345) shows the status
shape: `replicaStatuses` is a map keyed by replica type holding
active/succeeded/failed integers, and `conditions` is an ordered array typed
Created, Running, Succeeded (plus Restarting and Failed), each with reason,
message, and both `lastUpdateTime` and `lastTransitionTime`. The condition
types are uniform across frameworks; only the reason strings carry the
framework prefix (`TFJobRunning`, `PyTorchJobCreated`, `MPIJobCreated` in
`docs/legacy-v1/user-guides/pytorch.md`, `mpi.md`, `xgboost.md`). A superseded
condition is flipped to `status: "False"` rather than removed, so the array
doubles as a coarse lifecycle log.

### V1: operator Prometheus metrics

`docs/legacy-v1/user-guides/monitoring.md` documents the whole surface:
a `/metrics` endpoint on port 8080 (controller-runtime
`--metrics-bind-address`) exposing five counters, all labeled with
`namespace` and `framework` only. They are event-driven: the doc warns that
"metrics are only generated in response to specific events", i.e. a series
does not exist until the first job of that framework is created. There are no
gauges, no per-replica-type labels, and no current-phase series. Note a
naming drift inside the same doc: the sample output shows
`training_operator_jobs_created_total{framework="tensorflow",job_namespace="kubeflow"}`
while the label table says `namespace`.

### V1: the join pattern with kube-state-metrics

`docs/legacy-v1/user-guides/mpi.md` (lines 270-281) documents mpi-operator's
metrics: three lifecycle counters plus one info gauge,
`mpi_operator_job_info{launcher=<launcher-pod-name>,namespace=...}`, and a
"Join Metrics" recipe:
`kube_pod_info * on(pod,namespace) group_left label_replace(mpi_operator_job_infos, "pod", "$0", "launcher", ".*")`.
The operator never re-exports pod telemetry; it publishes just enough
identity (an info series carrying a pod name) for PromQL to attribute
kube-state-metrics/cadvisor/DCGM series to the job. This is the only
DCGM/cadvisor-adjacent monitoring content in the repo.

### V2: structure delegated to JobSet, no custom metrics

- Roles are now ReplicatedJobs in a JobSet-based runtime template. The role
  of a pod template is declared with the label
  `trainer.kubeflow.org/trainjob-ancestor-step` (values
  `dataset-initializer`, `model-initializer`, `trainer`) defined in
  `pkg/constants/constants.go`; runtime manifests such as
  `manifests/base/runtimes/torch_distributed.yaml` (line 32) carry it, and
  `pkg/runtime/runtime.go` builds an internal `PodSet.Ancestor` from it.
  Master/Worker collapsed into one `node` job; per-instance identity is the
  Job completion index, which `pkg/runtime/framework/plugins/torch/torch.go`
  (lines 153-156) maps into `PET_NODE_RANK` via the downward API.
- Status mirroring: `pkg/runtime/framework/plugins/jobset/jobset.go`
  (`Status()`, lines 411-446) copies JobSet's `ReplicatedJobsStatus` into
  `TrainJob.status.jobsStatus` as per-job Ready/Succeeded/Failed/Active/Suspended
  counts (`JobStatus` in `pkg/apis/trainer/v1alpha1/trainjob_types.go`,
  lines 526-558), and maps JobSet Completed/Failed conditions onto TrainJob
  `Complete`/`Failed` conditions. Together with `Suspended`
  (set in `pkg/controller/trainjob_controller.go`), that is the entire v2
  lifecycle vocabulary; the v1 Created/Running/Restarting ladder is gone.
- Metrics: `pkg/config/config.go` (lines 59-61) wires only the
  controller-runtime metrics server (chart default `:8443`,
  `charts/kubeflow-trainer/values.yaml` line 123). No custom prometheus
  collectors exist anywhere in `pkg/` (the only prometheus references are the
  config type and a test).
- Instead of Prometheus, v2 adds a push channel: the `TrainJobStatus` plugin
  (`pkg/runtime/framework/plugins/trainjobstatus/trainjobstatus.go`) injects a
  projected service-account token and a status URL into trainer containers,
  and `pkg/statusserver/server.go` accepts authenticated POSTs that apply
  `TrainerStatus` (progressPercentage, estimatedRemainingSeconds, up to 256
  name/value metrics; `pkg/apis/trainer/v1alpha1/trainjob_types.go` lines
  560-605) onto the CRD status subresource.

## Relevance to Karta

- The v1 label triple (job-name, replica-type, replica-index) is the
  attribution key Karta's exporter needs, already present on pods of every
  v1 training job. A Karta definition for PyTorchJob can select components by
  `training.kubeflow.org/replica-type` and instances by
  `training.kubeflow.org/replica-index`; the exporter then attributes DCGM or
  cadvisor series without touching the operator.
- V1's condition ladder is a working precedent for normalized phases across
  heterogeneous frameworks: shared condition types, framework-specific
  reasons. Karta's normalized phase mapping is the same idea lifted out of
  the operator.
- Neither v1 nor v2 ever exposed phase or per-replica-type state as a
  Prometheus time-series. Users get counters (v1) or CRD status ints
  (v1 `replicaStatuses`, v2 `jobsStatus`) and must join with
  kube-state-metrics themselves. The planned Karta phase gauge and
  per-component series fill a real, documented gap.
- V2's `status.jobsStatus` counts (Ready/Active/Succeeded/Failed/Suspended per
  component) are gauge-shaped data already computed by the controller. An
  exporter that mirrors CRD status integers into gauges gets component-level
  metrics for free, no pod watching required.
- V2's TrainerStatus push server shows demand for workload-level telemetry
  strong enough that the project built a bespoke authenticated HTTP channel
  into etcd. A Prometheus-native exporter is the complementary answer for
  anything with history or rate semantics.

## Evidence

- `docs/legacy-v1/user-guides/pytorch.md` - selects the master rank-0 pod via
  `training.kubeflow.org/{job-name,replica-type,replica-index}` labels; shows
  PyTorchJobCreated/Running conditions.
  https://github.com/kubeflow/training-operator/blob/master/docs/legacy-v1/user-guides/pytorch.md
- `docs/legacy-v1/user-guides/paddlepaddle.md` - full stamped label set
  including `training.kubeflow.org/operator-name`.
  https://github.com/kubeflow/training-operator/blob/master/docs/legacy-v1/user-guides/paddlepaddle.md
- `docs/legacy-v1/user-guides/tensorflow.md` - status sample with
  `replicaStatuses.Worker.succeeded: 2` and the Created/Running/Restarting/
  Succeeded/Failed condition vocabulary.
  https://github.com/kubeflow/training-operator/blob/master/docs/legacy-v1/user-guides/tensorflow.md
- `docs/legacy-v1/user-guides/monitoring.md` - the five v1 operator counters,
  their `namespace`/`framework` labels, port/bind-address config, and the
  "metrics only after events" caveat.
  https://github.com/kubeflow/training-operator/blob/master/docs/legacy-v1/user-guides/monitoring.md
- `docs/legacy-v1/user-guides/mpi.md` - mpi-operator counters plus the
  `mpi_operator_job_info` gauge and the kube-state-metrics
  `label_replace`/`group_left` join recipe.
  https://github.com/kubeflow/training-operator/blob/master/docs/legacy-v1/user-guides/mpi.md
- `docs/operator-guides/migration.md` - PyTorchJob Master/Worker replica specs
  and their collapse into a single TrainJob runtime.
  https://github.com/kubeflow/training-operator/blob/master/docs/operator-guides/migration.md
- `pkg/constants/constants.go` - `trainer.kubeflow.org/trainjob-ancestor-step`
  label and `JobCompletionIndexFieldPath`.
  https://github.com/kubeflow/training-operator/blob/master/pkg/constants/constants.go
- `pkg/runtime/runtime.go` - `PodSet.Ancestor` built from the ancestor-step
  label; `FindPodSetByAncestor`.
  https://github.com/kubeflow/training-operator/blob/master/pkg/runtime/runtime.go
- `pkg/runtime/framework/plugins/jobset/jobset.go` - `Status()` maps JobSet
  Completed/Failed conditions to TrainJob Complete/Failed and mirrors
  `ReplicatedJobsStatus` into `status.jobsStatus`.
  https://github.com/kubeflow/training-operator/blob/master/pkg/runtime/framework/plugins/jobset/jobset.go
- `pkg/apis/trainer/v1alpha1/trainjob_types.go` - v2 condition types
  (Suspended/Complete/Failed), `JobStatus` counts, `TrainerStatus` with
  progress and bounded metrics list.
  https://github.com/kubeflow/training-operator/blob/master/pkg/apis/trainer/v1alpha1/trainjob_types.go
- `pkg/runtime/framework/plugins/torch/torch.go` - `PET_NODE_RANK` derived
  from the Job completion-index annotation (per-instance identity in v2).
  https://github.com/kubeflow/training-operator/blob/master/pkg/runtime/framework/plugins/torch/torch.go
- `pkg/config/config.go` and `pkg/apis/config/v1alpha1/configuration_types.go` -
  metrics config is only the controller-runtime metrics server; no custom
  collectors in v2.
  https://github.com/kubeflow/training-operator/blob/master/pkg/config/config.go
- `pkg/statusserver/server.go` and
  `pkg/runtime/framework/plugins/trainjobstatus/trainjobstatus.go` - the
  authenticated push channel writing TrainerStatus into the CRD status.
  https://github.com/kubeflow/training-operator/blob/master/pkg/statusserver/server.go

## Lessons for Karta

- Attribute by joining on identity labels, not by proxying telemetry. The
  mpi-operator pattern (publish an info series carrying identity, let PromQL
  `group_left` do the join) keeps the exporter stateless and the raw
  pod-level series authoritative. Karta's exporter can emit
  `karta_component_pod_info{workload,component,component_instance,pod,namespace}`
  and let dashboards join DCGM/cadvisor series against it.
- A three-level label contract (workload, component/replica-type,
  instance/replica-index) on pods is proven at ecosystem scale; Karta
  definitions should consume the operators' existing labels
  (`training.kubeflow.org/*`, JobSet's replicated-job labels, completion
  index) rather than requiring new ones.
- Keep the workload-level metric surface low-cardinality and let identity
  labels carry the fan-out. V1's counters stayed at namespace+framework;
  everything finer lived on pods.
- Mirror CRD status integers into gauges. Per-component
  active/succeeded/failed counts already exist in both v1
  (`replicaStatuses`) and v2 (`jobsStatus`); an exporter reading only the CR
  gets component-level state without a pod informer.
- Normalize the phase vocabulary but preserve provenance: shared condition
  types with framework-specific reasons worked well in v1 and maps directly
  to Karta's normalized phase plus source-specific detail.

## What NOT to copy

- Counters without a phase gauge. V1 cannot answer "what state is this job in
  now" from metrics, and the docs must warn that series only appear after
  events. Karta's phase-as-timeseries gauge is the fix; do not ship
  lifecycle counters alone.
- Metric label drift. The same v1 doc shows `job_namespace` in output and
  `namespace` in the reference table. Define the exporter's label schema once
  and generate docs from it.
- V2's full retreat from custom metrics. Dropping the counters without a
  replacement time-series leaves only CRD status, which has no history.
- Pushing training metrics into the CRD status subresource (v2 TrainerStatus,
  capped at 256 name/value string pairs) as the telemetry channel. etcd is
  not a TSDB; keep sample streams in Prometheus and only summaries in status.
- Framework-prefixed condition reasons leaking into the normalized layer
  (`TFJobRunning` vs `PyTorchJobRunning`). Karta's normalized phase values
  must be identical across workload types; keep source flavor in a separate
  detail field.
