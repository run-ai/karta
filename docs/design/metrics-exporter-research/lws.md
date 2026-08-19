<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# lws: leader/worker role identity for metric attribution

## TL;DR

- LWS does not export any workload-level Prometheus metrics of its own. The controller exposes only the default controller-runtime `/metrics` endpoint; `prometheus/client_golang` is an indirect dependency and no custom metric is defined anywhere in `pkg/`.
- Instead, LWS invests in a rich pod label taxonomy (`leaderworkerset.sigs.k8s.io/name`, `group-index`, `worker-index`, `group-key`, `subgroup-index`) injected by a pod webhook. Any metrics pipeline can attribute per-pod telemetry to workload / group / role by joining on these labels.
- Role identity is positional, not a dedicated label: the leader is the pod with `worker-index: 0`. The controller and the HPA selector both encode "leader" as `worker-index=0`.
- The newer DisaggregatedSet CRD (prefill/decode LLM inference) adds an explicit role label (`disaggregatedset.x-k8s.io/role`) plus per-role status (`roleStatuses[]` with replicas/readyReplicas/updatedReplicas per role name).
- For metric aggregation, LWS delegates to the application: the API docs explicitly suggest the leader pod aggregate metrics from its group and expose them as a summary custom metric, with HPA reading only leader pods via `status.hpaPodSelector`.

## How it works

### Component identity via injected labels

The label vocabulary lives in `api/leaderworkerset/v1/leaderworkerset_types.go`:

- `leaderworkerset.sigs.k8s.io/name`: workload identity, on every Pod/StatefulSet/Service.
- `leaderworkerset.sigs.k8s.io/group-index`: which replica group (component instance) the pod belongs to, 0..N-1.
- `leaderworkerset.sigs.k8s.io/worker-index`: identity inside the group; the leader is always 0, workers are 1..M.
- `leaderworkerset.sigs.k8s.io/group-key`: a unique hash shared by all pods of one group, useful as a stable join key.
- `leaderworkerset.sigs.k8s.io/subgroup-index` and `subgroup-key`: finer-grained sub-component identity when SubGroupPolicy is set.
- Annotations carry the shape: `size` (pods per group), `replicas` (group count), `leader-name` on workers.

The pod mutating webhook (`pkg/webhooks/pod_webhook.go`) stamps `group-index`, `worker-index`, and `subgroup-index` at pod admission, so the labels exist before any scrape. The same values are mirrored into container env vars (`LWS_WORKER_INDEX`, `LWS_GROUP_SIZE`, `LWS_LEADER_ADDRESS`), so the application itself can label its own metrics with its role and group. The full table of labels, annotations, and env vars is documented in `site/content/en/docs/reference/labels-annotations-and-environment-variables.md`, which also recommends the Downward API to turn labels into env vars.

### Role is positional in LWS, explicit in DisaggregatedSet

LWS has no `role: leader|worker` label. "Leader" means `worker-index == "0"`. `pkg/controllers/leaderworkerset_controller.go` selects leaders with exactly that selector, both in `updateConditions` (lines 416-419) and when building `status.hpaPodSelector` (lines 533-540):

```go
MatchLabels: map[string]string{
    leaderworkerset.SetNameLabelKey:     lws.Name,
    leaderworkerset.WorkerIndexLabelKey: "0", // select leaders
}
```

DisaggregatedSet (`api/disaggregatedset/v1/disaggregatedset_types.go`) fixes this for the multi-role case: `disaggregatedset.x-k8s.io/role` carries the role name (for example "prefill", "decode") on the per-role LWS, its Service, and its Pods, next to `disaggregatedset.x-k8s.io/name` and a `slice` label. Each role is a full LeaderWorkerSetTemplateSpec, so a pod in a prefill/decode deployment carries two layers of component identity: DSet name + role, then LWS group-index + worker-index.

### Status semantics: group-as-unit readiness

`status.replicas`, `readyReplicas`, and `updatedReplicas` all count groups, not pods (`api/leaderworkerset/v1/leaderworkerset_types.go` lines 363-392). A group is ready only when the leader pod is running and ready AND its worker StatefulSet is ready (`updateConditions` in `pkg/controllers/leaderworkerset_controller.go`, using `podutils.PodRunningAndReady` and `statefulsetutils.StatefulsetReady`). `updatedReplicas` is revision-hash based and independent of readiness. Conditions (`Available`, `Progressing`, `UpdateInProgress`) are derived from the same per-group ready/updated walk. DisaggregatedSet repeats the pattern per role: `status.roleStatuses[]` has `{name, replicas, readyReplicas, updatedReplicas}` per role.

### Prometheus: controller metrics only, aggregation pushed to the app

`cmd/main.go` (around lines 335-350) configures the stock controller-runtime metrics server with authn/authz filtering; there is no custom collector. `config/components/prometheus/monitor.yaml` ships a ServiceMonitor for the controller-manager `/metrics` endpoint, and `site/content/en/docs/manage/prometheus.md` only covers enabling that ServiceMonitor (Kustomize or Helm, with optional cert-manager TLS). Nothing scrapes or re-labels workload pod metrics.

For workload-level metrics, the API comment on `spec.replicas` (`api/leaderworkerset/v1/leaderworkerset_types.go` lines 112-116) states the design position: the HPA selector targets leader pods only, and "the leader pod could aggregate metrics from the rest of the group and expose them as a summary custom metric representing the whole group." The HPA example (`site/content/en/docs/examples/hpa.md`) repeats this: HPA monitors leader pods only, scaling moves whole groups, and users must "ensure leaders represent the load of the entire group."

### Per-role scaling for prefill/decode (KEP-849)

`keps/849-DisaggregatedSet-HPA/README.md` proposes a per-role `/scale` delegation CR (`DisaggregatedSetRoleScaler`, named `<ds>-<role>`) so HPA/KEDA can drive one role's replicas from role-specific metrics. Notably, metrics themselves stay out of scope: "Autoscaling metrics or recommendations... is entirely the user's responsibility." The motivating problem is instructive: per-role LWS names embed a revision hash and change on rollout, so anything (HPA or a metrics join) keyed on the LWS name breaks; stable role identity has to come from the DSet-level labels or a stable intermediary object.

## Relevance to Karta

LWS is the concrete shape of the "component-instance" granularity Karta wants to attribute: workload (LWS name) -> component (leader vs worker role) -> component instance (group-index) -> pod (worker-index). The disaggregated LLM inference case from the question maps directly to DisaggregatedSet, where role is a first-class label ("prefill"/"decode") and status is already per-role. LWS proves the ecosystem pattern: the workload controller's job is to make identity cheap to join on (labels stamped at admission, stable across restarts), and the metrics pipeline (Prometheus relabeling, an exporter, or the app itself) does the attribution. LWS also shows the gap Karta's exporter fills: LWS ships zero workload-level metrics, so today users must hand-write PromQL joins over `leaderworkerset_sigs_k8s_io_*` pod labels or make the leader aggregate in-app.

## Evidence

- `api/leaderworkerset/v1/leaderworkerset_types.go` - the full label/annotation vocabulary (name, group-index, worker-index, group-key, subgroup-*), group-counted status fields, and the spec.replicas comment recommending leader-side metric aggregation for HPA. https://github.com/kubernetes-sigs/lws/blob/main/api/leaderworkerset/v1/leaderworkerset_types.go
- `pkg/webhooks/pod_webhook.go` - mutating webhook stamps group-index, worker-index, subgroup-index on pods at admission. https://github.com/kubernetes-sigs/lws/blob/main/pkg/webhooks/pod_webhook.go
- `pkg/controllers/leaderworkerset_controller.go` - per-group readiness (leader pod ready AND worker sts ready), readyReplicas/updatedReplicas counting, and hpaPodSelector built as name + worker-index=0. https://github.com/kubernetes-sigs/lws/blob/main/pkg/controllers/leaderworkerset_controller.go
- `api/disaggregatedset/v1/disaggregatedset_types.go` - explicit role/slice/revision labels and per-role RoleStatus (replicas/readyReplicas/updatedReplicas). https://github.com/kubernetes-sigs/lws/blob/main/api/disaggregatedset/v1/disaggregatedset_types.go
- `cmd/main.go` - only the controller-runtime metrics server is configured; no custom collectors. https://github.com/kubernetes-sigs/lws/blob/main/cmd/main.go
- `config/components/prometheus/monitor.yaml` - ServiceMonitor scrapes the controller-manager /metrics endpoint only. https://github.com/kubernetes-sigs/lws/blob/main/config/components/prometheus/monitor.yaml
- `site/content/en/docs/manage/prometheus.md` - Prometheus docs cover controller metrics enablement only, no workload metrics. https://github.com/kubernetes-sigs/lws/blob/main/site/content/en/docs/manage/prometheus.md
- `site/content/en/docs/examples/hpa.md` - HPA guidance: monitor leader pods only, leaders must represent group load, scaling is group-as-unit. https://github.com/kubernetes-sigs/lws/blob/main/site/content/en/docs/examples/hpa.md
- `site/content/en/docs/reference/labels-annotations-and-environment-variables.md` - user-facing reference for all identity labels, annotations, and env vars, plus Downward API guidance. https://github.com/kubernetes-sigs/lws/blob/main/site/content/en/docs/reference/labels-annotations-and-environment-variables.md
- `site/content/en/docs/concepts/disaggregatedset/_index.md` - prefill/decode/encode disaggregated inference concept; DSet orchestrates one LWS per role. https://github.com/kubernetes-sigs/lws/blob/main/site/content/en/docs/concepts/disaggregatedset/_index.md
- `keps/849-DisaggregatedSet-HPA/README.md` - per-role scaler CR for HPA/KEDA; metrics choice explicitly out of scope; LWS-name-changes-on-rollout problem. https://github.com/kubernetes-sigs/lws/blob/main/keps/849-DisaggregatedSet-HPA/README.md

## Lessons for Karta

- Treat pod labels as the attribution contract. LWS shows that stable, admission-time labels (workload name, component instance index, in-component index) are enough for any downstream metrics join. Karta's exporter should key its pod-to-component mapping on selector labels, exactly what the Karta CRD already describes.
- Model three identity layers, not two: workload, component (role), component instance (group-index). LWS needs all three for meaningful attribution (a prefill group 3 leader is different from decode group 0 worker 2). Karta's workload/component/component-instance granularity matches this exactly.
- Positional role encoding is real and must be normalized. In plain LWS "leader" is `worker-index=0`, not a role label. Karta's exporter must be able to derive a role from a label value expression (index == 0 -> leader), not only from label presence.
- Readiness rolls up per component instance, not per pod. LWS counts a group ready only when leader + all workers are ready. A Karta exporter emitting per-component-instance readiness gauges would replicate what LWS computes internally but never exports.
- Names are unstable, labels are not. KEP-849's core pain is revision hashes inside resource names breaking HPA targets across rollouts. Karta metric label values should come from CRD-declared identity (workload name, role name), never from generated child resource names.
- There is a genuine gap to fill. LWS exports nothing at workload level and tells users to aggregate in-app or in PromQL. An external exporter that turns label taxonomy plus status into metrics is complementary, not redundant.

## What NOT to copy

- Do not copy the "leader aggregates group metrics in the application" pattern as the primary design. It couples attribution to app code and every framework must reimplement it; Karta's whole point is doing this outside the app.
- Do not copy the positional-only role encoding (`worker-index=0` means leader) as Karta's exposed model. Emit an explicit role/component label like DisaggregatedSet does; keep the positional rule only as an input mapping.
- Do not copy the absence of workload-level metrics. LWS computes per-group ready/updated states every reconcile and then discards them into aggregate status ints; exporting only pre-aggregated counts loses the per-instance detail Karta needs.
- Do not embed revision hashes or generated child names in metric labels; KEP-849 documents how that breaks anything keyed on them across rollouts.
- Do not scope the exporter's Prometheus story to controller self-metrics with a ServiceMonitor, as LWS's Prometheus docs do; that covers operability of the controller, not observability of the workloads.
