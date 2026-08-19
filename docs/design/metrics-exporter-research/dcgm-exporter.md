<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# dcgm-exporter: per-pod GPU metric attribution

## TL;DR

- Attribution is a scrape-time transformation pipeline, not part of collection. DCGM collectors gather raw device metrics; a chain of `Transform` implementations (PodMapper, hpcMapper, containerMapper) rewrites them before rendering.
- The authoritative device-to-pod binding comes from the kubelet pod-resources gRPC API (unix socket), queried fresh on every scrape. No heuristics: kubelet says which pod/container owns which GPU device ID.
- Pod metadata enrichment (UID, pod labels) comes from a separate source: a client-go informer scoped to the node via a `spec.nodeName` field selector, with a regex allowlist plus LRU cache for label filtering.
- Shared devices fan out: one metric copy per pod, and for shared GPUs a per-process NVML path splits utilization/memory across pods via `/proc/<pid>/cgroup` parsing, emitting 0 for idle pods so series never disappear.
- Enrichment failure never fails the scrape. Mapping errors degrade to unlabeled device metrics with a warning.

## How it works

Collection and attribution are decoupled. `DCGMCollector` (internal/pkg/collector/gpu_collector.go) watches DCGM fields; the DCGM host engine samples them in the background at `--collect-interval` (default 30000 ms, pkg/cmd/app.go) via `WatchFieldsWithGroupEx` (internal/pkg/devicewatcher/device_watcher.go). A Prometheus scrape of `/metrics` (internal/pkg/server/server.go) calls `Registry.Gather()`, which runs all collectors concurrently with an errgroup (internal/pkg/registry/registry.go) and reads the latest cached values. So sampling resolution is set by the watch interval, not the scrape rate.

The server then applies transformations per entity group before rendering one combined text document (`MetricsServer.render` in internal/pkg/server/server.go). `GetTransformations` (internal/pkg/transformation/transformer.go) builds the chain from config: Kubernetes mode adds `PodMapper`, `--hpc-job-mapping-dir` adds `hpcMapper`, standalone container mode adds `containerMapper`.

`PodMapper.Process` (internal/pkg/transformation/kubernetes.go) connects to the kubelet pod-resources socket on every scrape, lists pod resources with a raised 16 MiB gRPC receive limit, and builds a deviceID-to-PodInfo map. Device IDs are normalized across device plugin flavors: MIG UUIDs resolve to GPU instance identifiers via NVML, GKE MIG/vGPU IDs are parsed with regexes, and `::N` sharing suffixes are split. The mapping key type is configurable (`--kubernetes-gpu-id-type`: GPU UUID or device name). Each matched metric gets `pod`, `namespace`, `container` attributes, optionally `pod_uid` and `vgpu`.

Pod labels come from a node-scoped informer: `NewPodMapper` builds a SharedInformerFactory filtered by `spec.nodeName=$NODE_NAME` and reads from the lister cache in `createPodInfo`. Label keys pass through a regex allowlist backed by a bounded LRU cache (default 150k entries), get sanitized, and collide-safe renaming (`pod_label_` prefix, `_conflictN` suffix) protects reserved renderer labels per entity type.

For time-shared GPUs (`--kubernetes-virtual-gpus`), per-process metrics (GPU util, FB used) are read from NVML per PID, PIDs map to pod UIDs by parsing `/proc/<pid>/cgroup` (internal/pkg/transformation/pidmapper.go), values are summed per pod, and pods with no active process get an explicit 0 (internal/pkg/transformation/kubernetes.go `buildIdlePodValues`). The original device-level metric is kept alongside the per-pod copies.

The HPC path (internal/pkg/transformation/hpc.go) is a file-drop contract: Slurm prolog scripts write files named `<GPU_ID>` or `<GPU_ID>.<GPU_INSTANCE_ID>` containing job names, and the mapper emits one metric copy per job with an `hpc_job` attribute.

## Relevance to Karta

dcgm-exporter solves the bottom half of Karta's problem: attributing device telemetry to pod/container. It stops there; it has no notion of workload or component, and as a per-node DaemonSet it cannot aggregate across nodes. Karta's exporter would sit exactly one level up: consume pod-labeled series (which dcgm-exporter already emits) and roll them up to workload/component/component-instance using Karta's pod selectors. The architectural patterns transfer directly: a transformation stage separate from collection, an authoritative source for the join key (Karta CR pod selectors playing the role the pod-resources API plays here), an informer cache for metadata, and strict never-fail-the-scrape degradation. The 30s default collect interval also confirms 30s-or-finer is the established granularity for GPU telemetry.

## Evidence

- `internal/pkg/transformation/kubernetes.go` - PodMapper: pod-resources gRPC client, device ID normalization (MIG, GKE, vGPU), pod/namespace/container attribute stamping, informer-backed pod labels, allowlist LRU, per-pod fan-out and idle-pod zeros. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/transformation/kubernetes.go
- `internal/pkg/transformation/transformer.go` - transformation chain assembly from config (Kubernetes, HPC, container modes). https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/transformation/transformer.go
- `internal/pkg/transformation/types.go` - `Transform` interface, `PodMapper` and `PodInfo` structs, DRA types. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/transformation/types.go
- `internal/pkg/transformation/const.go` - attribute names: `pod`, `namespace`, `container`, `pod_uid`, `vgpu`, `hpc_job`, `pod_label_` prefix, `dra_*`. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/transformation/const.go
- `internal/pkg/transformation/hpc.go` - Slurm job mapping via per-GPU files, one metric copy per job. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/transformation/hpc.go
- `internal/pkg/transformation/pidmapper.go` - PID to pod UID via `/proc/<pid>/cgroup` parsing with memoization. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/transformation/pidmapper.go
- `internal/pkg/transformation/process_metrics.go` - per-process NVML SM util and memory, summed per pod for shared GPUs. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/transformation/process_metrics.go
- `internal/pkg/server/server.go` - `/metrics` handler: gather, per-group transformation loop, single-pass render; hot-reloadable registry. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/server/server.go
- `internal/pkg/registry/registry.go` - concurrent collector gather with errgroup, refcounted cleanup. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/registry/registry.go
- `internal/pkg/collector/gpu_collector.go` - reads latest DCGM watched values per scrape; stale profiling watch detection and one-shot repair. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/collector/gpu_collector.go
- `internal/pkg/devicewatcher/device_watcher.go` - DCGM field watches at the collect interval (`WatchFieldsWithGroupEx`). https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/devicewatcher/device_watcher.go
- `pkg/cmd/app.go` - flags: `--collect-interval` (default 30000 ms), `-k/--kubernetes`, `--kubernetes-gpu-id-type`, `--kubernetes-enable-pod-labels`, `--kubernetes-pod-label-allowlist-regex`. https://github.com/NVIDIA/dcgm-exporter/blob/main/pkg/cmd/app.go
- `internal/pkg/transformation/container.go` - non-Kubernetes container attribution via container runtime socket, same fan-out pattern. https://github.com/NVIDIA/dcgm-exporter/blob/main/internal/pkg/transformation/container.go

## Lessons for Karta

- Model attribution as a pipeline of `Transform` stages over already-collected metrics. Karta's exporter can chain workload resolution, component resolution, and aggregation as independent, testable stages.
- Pick one authoritative source per fact. Ownership binding (device to pod) and metadata (labels, UID) come from different APIs and fail independently. For Karta: pod-to-component binding from the Karta CR selectors, pod metadata from an informer.
- Scope informers hard. The `spec.nodeName` field selector keeps memory bounded; Karta's cluster-level exporter should scope by workload selectors or namespaces where possible.
- Never fail a scrape on enrichment errors. Return the unattributed metric and log; a partially labeled scrape beats a gap in Prometheus.
- Emit explicit zeros for known-but-idle members (idle pods on a shared GPU get 0). Karta should do the same for component instances with no current samples, so rollups stay continuous.
- Decouple sampling from serving. DCGM samples in the background at the collect interval and scrapes read cached values; Karta can poll metrics-server/Prometheus on its own 30s clock and serve the latest snapshot at scrape time.
- Treat label hygiene as a real subsystem: sanitization, reserved-label lists per metric family, deterministic collision renaming, and allowlist filtering with a bounded cache.

## What NOT to copy

- Node-local scope. dcgm-exporter cannot aggregate across nodes, so it never rolls up to workload level. Karta needs a cluster-scoped design; copying the DaemonSet-joins-local-kubelet pattern would make workload aggregation impossible.
- Per-scrape deep copies. Fan-out uses `utils.DeepCopy` per metric per pod inside the scrape path; at Karta's cardinality this would be an allocation hot spot. Precompute attribution maps between scrapes instead.
- Duplicated mapping logic. `toDeviceToPod` and `toDeviceToSharingPods` are acknowledged copy-paste (TODO in the code). Design one mapping function with multiplicity from the start.
- String-typed metric values and a hand-rolled text renderer instead of the Prometheus client library. Karta should use prometheus/client_golang and get exposition, escaping, and HELP/TYPE handling for free.
- The dual `Labels`/`Attributes` maps with delete-the-overlap robustness patches scattered at every enrichment site. Keep one label map with a single collision policy.
- The HPC file-drop contract (files named by GPU ID in a watched directory). It is fragile and unversioned; Karta's CRD already provides a structured alternative.
