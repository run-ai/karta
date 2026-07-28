<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Sample index

Pick the sample whose shape is closest to the target workload, copy it, and
adapt the GVK, paths, and status mapping. Samples live in `docs/samples/`.
Minimal, suspend-aware catalog definitions for built-in kinds live in
`docs/catalog/`.

## How to choose

Answer these, then choose the most specific matching row in the table below (the
one naming the most of your workload's traits). When several rows match, a
multi-instance or nested pattern (for example Ray worker groups needing
`instanceIdPath`) takes precedence over a generic role-based row.

1. Does the workload own other resources, or is it a single object with pods
   defined in its own spec?
2. Where does the pod template live: a full template, a bare pod spec, or
   scattered fields?
3. Does it report status through conditions, a phase string, or status fields?
4. Does any component hold several specs (worker groups, services, replicated
   jobs) that need per-instance identity?
5. Are there nested layers of ownership (a group that owns a leader and workers)?

## Decision table

| Workload shape | Closest sample | Why |
|---|---|---|
| Single Job, full pod template, status via conditions plus a computed expression | `docs/catalog/batch-job-v1.yaml` | `podTemplateSpecPath: .spec.template`, `byConditions` plus `byExpression`, suspend via `.spec.suspend`. |
| CronJob wrapping a Job template, pod template nested under `jobTemplate` | `docs/catalog/batch-cronjob-v1.yaml` | One child `Job`, `podTemplateSpecPath: .spec.jobTemplate.spec.template`, expression-only status, suspend. |
| Deployment or other controller that generates ReplicaSets | `docs/catalog/apps-deployment-v1.yaml`, `docs/samples/knative-serving.yaml` | Root plus a generated child; child carries the pod template. |
| Role-based distributed job (master and worker, launcher and worker) with pods as children | `docs/samples/pytorch.yaml`, `docs/samples/mpijob.yaml` | Children are `Pod` kind, one per role, each with a `componentTypeSelector` on a role label. The label key is operator-specific: PyTorchJob uses `training.kubeflow.org/replica-type`, MPIJob uses `training.kubeflow.org/job-role`. Verify the real label before copying. |
| Several worker groups under one role that need per-group identity | `docs/samples/raycluster.yaml`, `docs/samples/rayjob.yaml`, `docs/samples/rayservice.yaml` | Worker component uses `instanceIdPath` on `workerGroupSpecs[].groupName` paired with a `componentInstanceSelector`. |
| Replicated jobs, each a named template instance | `docs/samples/jobset.yaml` | `replicatedjob` child with `instanceIdPath: .spec.replicatedJobs[].name` and a `componentInstanceSelector`. |
| Nested ownership with identical replicated sub-structures (a group that owns a leader and workers) | `docs/samples/lws.yaml` | `group` child owns `leader` and `worker`; `replicaSelector` on the group; `componentTypeSelector` distinguishes leader from worker. |
| Fields scattered across the spec, status via a phase string | `docs/samples/dynamo.yaml`, `docs/samples/dynamo-v1beta1.yaml`, `docs/samples/nimservice.yaml`, `docs/samples/nimcache.yaml` | `fragmentedPodSpecDefinition` with per-field paths; `phaseDefinition` plus `byPhase` mappings. |
| Status reported through both a phase and conditions | `docs/samples/milvus.yaml` | Declares both `phaseDefinition` and `conditionsDefinition`; maps statuses `byPhase`. |
| Multi-service inference, each service its own component | `docs/samples/kserve.yaml` | Predictor and transformer children mix `fragmentedPodSpecDefinition` and `podSpecPath` plus `metadataPath`; `componentTypeSelector` per service. |
| Nested pod cliques and scaling groups | `docs/samples/grove-podcliqueset.yaml` | Multiple multi-instance children (`clique`, `scalinggroup`) each with `instanceIdPath` plus instance and replica selectors. This CRD has no aggregate phase, so status is mapped with `byExpression` over replica counts, not `byPhase`. |

## Pattern quick reference

- Single full pod template: `podTemplateSpecPath`. See `batch-job-v1.yaml`.
- Bare pod spec with separate metadata: `podSpecPath` plus `metadataPath`. See
  the transformer child in `kserve.yaml`.
- Scattered fields: `fragmentedPodSpecDefinition`. See `dynamo-v1beta1.yaml`.
- Conditions status: `conditionsDefinition` plus `byConditions`. See
  `rayservice.yaml`, which also uses `reason` to separate degraded from failed.
- Phase status: `phaseDefinition` plus `byPhase`. See `raycluster.yaml`.
- Expression status: `byExpression`. See `batch-job-v1.yaml`.
- Per-instance identity: `instanceIdPath` plus `componentInstanceSelector`. See
  `raycluster.yaml`.
- Replica identity within identical sub-structures: `replicaSelector`. See
  `lws.yaml`.
