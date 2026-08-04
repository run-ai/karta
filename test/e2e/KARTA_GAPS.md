<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta catalog gaps found by the e2e recorder

The e2e recorder drives a real workload through a live cluster and classifies each
observed CR from the workload's own status fields, independent of Karta. When a
settled CR matches none of the recorder's declared states it is recorded as
`Undefined`. An `Undefined` step is a signal that the built-in Karta definition
does not cover a status the operator actually produces.

This file records every such gap: the Karta mapping involved, the exact CR status
that triggered it, what Karta read before the fix, the change made, and what Karta
reads after. Reproduce any row with the `kread` tool:

```
cd test/e2e && go run ./cmd/kread ../../docs/catalog/<def>.yaml <cr.json>
```

## 1. batch/v1 Job: Completed misses SuccessCriteriaMet

A Job that meets its success criteria first gets a `SuccessCriteriaMet` condition,
then `Complete` a moment later. During that window the only success condition
present is `SuccessCriteriaMet`, which the Completed mapping did not match.

CR status:

```json
{"status": {"conditions": [{"type": "SuccessCriteriaMet", "status": "True"}], "succeeded": 1}}
```

Karta before: `[Undefined]` (Completed matched only `Complete=True`).

Added a second Completed matcher (matchers within a status are OR'd):

```go
Completed: []v1alpha1.StatusMatcher{
    {ByConditions: []v1alpha1.ExpectedCondition{{Type: "Complete", Status: ptr.To("True")}}},
    {ByConditions: []v1alpha1.ExpectedCondition{{Type: "SuccessCriteriaMet", Status: ptr.To("True")}}},
},
```

Karta after: `[Completed]`.

## 2. batch/v1 Job: Failed misses FailureTarget

Symmetric to gap 1. A failing Job gets a `FailureTarget` condition before `Failed`.

CR status:

```json
{"status": {"conditions": [{"type": "FailureTarget", "status": "True", "reason": "BackoffLimitExceeded"}], "failed": 1}}
```

Karta before: `[Undefined]` (Failed matched only `Failed=True`).

Added a second Failed matcher:

```go
Failed: []v1alpha1.StatusMatcher{
    {ByConditions: []v1alpha1.ExpectedCondition{{Type: "Failed", Status: ptr.To("True")}}},
    {ByConditions: []v1alpha1.ExpectedCondition{{Type: "FailureTarget", Status: ptr.To("True")}}},
},
```

Karta after: `[Failed]`.

## 3. apps/v1 Deployment: Initializing misses the just-created state

A freshly created Deployment writes `Progressing=True/NewReplicaSetCreated` before
any `Available` condition exists. The Initializing mapping was a `byConditions`
matcher requiring `Available=False`, but a `byConditions` clause only matches a
condition that is present, so an absent `Available` never matched.

CR status:

```json
{"status": {"conditions": [{"type": "Progressing", "status": "True", "reason": "NewReplicaSetCreated"}]}}
```

Karta before: `[Undefined]`.

Replaced the Initializing matcher with a `byExpression` that fires when
`Progressing=True` and no `Available=True` condition exists (whether `Available`
is False or absent):

```jq
(([.status.conditions[]? | select(.type == "Progressing" and .status == "True")] | length) > 0)
and
(([.status.conditions[]? | select(.type == "Available" and .status == "True")] | length) == 0)
```

Karta after: `[Initializing]`. A running Deployment
(`Progressing=True/NewReplicaSetAvailable`, `Available=True`) still reads
`[Running]`, so the two do not overlap.

## 4. apps/v1 StatefulSet: no state during scale-down

While scaling down (for example 3 to 1), a transient CR has more ready pods than
desired because the extra pods are still terminating, yet `observedGeneration`
and `updatedReplicas` have already caught up to the new spec. Running needs
`readyReplicas == spec.replicas`, Degraded needs `readyReplicas < spec.replicas`,
and Initializing only covered "not enough ready yet", so `readyReplicas >
spec.replicas` matched nothing.

CR status:

```json
{"spec": {"replicas": 1},
 "status": {"observedGeneration": 3, "replicas": 2, "readyReplicas": 2, "updatedReplicas": 1,
            "currentReplicas": 1, "currentRevision": "r", "updateRevision": "r"}}
```

Karta before: `[Undefined]`.

Added a `readyReplicas > spec.replicas` term to the Initializing expression:

```jq
(.spec.replicas // 1) > 0 and (
    (.status.observedGeneration // 0) != (.metadata.generation // 0)
    or (.status.readyReplicas // 0) == 0
    or (.status.readyReplicas // 0) > (.spec.replicas // 1)
    or (.status.updatedReplicas // 0) != (.spec.replicas // 1)
    or (.status.currentRevision != .status.updateRevision)
)
```

Karta after: `[Initializing]`. The new term is disjoint from Running (`==`) and
Degraded (`<`), so a steady Degraded StatefulSet still reads `[Degraded]` and a
fully rolled-out one still reads `[Running]`.

## 5. jobset.x-k8s.io/v1alpha2 JobSet: no state at the ends of the run

Two windows had no state. A just-created JobSet writes `replicatedJobsStatus`
with all counts zero before any pod is active. And after a job succeeds, there is
a window where the replicatedJob shows `succeeded` but the JobSet-level `Completed`
condition is not set yet. Initializing required `active > 0`, Running required
`ready > 0 and active > 0`, and the terminal states are by condition, so both
windows matched nothing. The controller also briefly flaps `ready` to 0 mid-run
while `active` stays set, which made a `ready`-only Running oscillate.

CR status (just-created; the succeeded window is the same with `succeeded: 1`):

```json
{"status": {"replicatedJobsStatus": [{"name": "workers", "active": 0, "ready": 0, "succeeded": 0, "failed": 0}]}}
```

Karta before: `[Undefined]`.

Redefined the two non-terminal states so they are stable and total. Running is
now "has working pods" (active or ready), which the stable `active` count keeps
from flapping. Initializing is "in progress with no working pods and no terminal
or suspended condition", which covers just-created and the succeeded-pending
window:

```jq
# Running
(.status.replicatedJobsStatus // []) | any((.active // 0) > 0 or (.ready // 0) > 0) and all((.failed // 0) == 0)

# Initializing
((.status.replicatedJobsStatus // []) | length) > 0
and ((.status.replicatedJobsStatus // []) | all((.active // 0) == 0 and (.ready // 0) == 0))
and (([.status.conditions[]? | select((.type == "Completed" or .type == "Failed" or .type == "Suspended") and .status == "True")] | length) == 0)
```

Karta after: just-created and succeeded-pending read `[Initializing]`, a working
JobSet reads `[Running]`, and a completed one reads `[Completed]`.

## 6. leaderworkerset.x-k8s.io/v1 LeaderWorkerSet: Initializing misses startup

Same shape as the Deployment gap. A starting LeaderWorkerSet is
`Progressing=True` before it writes an `Available` condition. The Initializing
matcher required `Available=False` by condition, which cannot match an absent
condition, so startup read Undefined.

CR status:

```json
{"spec": {"replicas": 1},
 "status": {"replicas": 1, "updatedReplicas": 1, "conditions": [{"type": "Progressing", "status": "True"}]}}
```

Karta before: `[Undefined]`.

Replaced the Initializing matcher with the same `byExpression` used for the
Deployment: `Progressing=True` and no `Available=True` condition. Karta after:
`[Initializing]`, and a ready LeaderWorkerSet still reads `[Running]`.

## 7. leaderworkerset.x-k8s.io/v1 LeaderWorkerSet: no state during scale-down

While scaling down, `status.replicas` lags at the old count while the desired
replica is already ready (`Available=True`). Running keyed off the replica counts
(`readyReplicas == status.replicas`), which fails when `status.replicas` lags, so
the transient read Undefined. A separate Running matcher tried to use the
`Available=True` condition but required `reason=AllGroupsReady`, and the
LeaderWorkerSet ConditionsDefinition does not extract `reason`, so that matcher
never matched and the count-based one was the only live path.

CR status:

```json
{"spec": {"replicas": 1},
 "status": {"replicas": 2, "readyReplicas": 1, "updatedReplicas": 2,
            "conditions": [{"type": "Progressing", "status": "False"}, {"type": "Available", "status": "True", "reason": "AllGroupsReady"}]}}
```

Karta before: `[Undefined]`.

`Available=True` is the operator's authoritative "all groups ready" signal and
stays True through the scale-down. Dropped the `reason` requirement (which the
def cannot read) so the matcher keys on `Available=True` alone:

```go
Running: {ByConditions: [{Type: "Available", Status: "True"}]}, // + the replica-settled fallback
```

Karta after: `[Running]`. Startup (`Available=False`) still reads `[Initializing]`
because the Initializing byExpression requires no `Available=True` condition.

## 8. ray.io/v1 RayCluster: no Initializing state

The RayCluster definition mapped only Running (`.status.state == "ready"`), Failed
(`"failed"`), and Suspended. It had no Initializing, so a provisioning cluster
(state empty until the head and worker pods are ready) and the resume window
(spec.suspend already false while status.state still lags at "suspended") both
read Undefined.

CR status (provisioning; the resume window is `state: "suspended"` with
`suspend: false`):

```json
{"spec": {"suspend": false}, "status": {"conditions": [{"type": "RayClusterProvisioned", "status": "False"}]}}
```

Karta before: `[Undefined]`.

Added an Initializing matcher for "converging toward ready": not suspended and
state not yet "ready" or "failed".

```jq
(.spec.suspend != true) and (.status.state != "ready") and (.status.state != "failed")
```

Karta after: `[Initializing]`. A ready cluster still reads `[Running]`, a suspended
one `[Suspended]`.

## 9. ray.io/v1 RayJob: Initializing misses the provisioning window

RayJob mapped Initializing only to jobStatus `PENDING`. Before that, the RayJob
brings up its own cluster while `.status.jobStatus` is still empty (and
`jobDeploymentStatus` is `Initializing`, then `Running` before the job is
submitted), so the provisioning window read Undefined.

CR status:

```json
{"status": {"jobDeploymentStatus": "Initializing"}}
```

Karta before: `[Undefined]`.

Added a second Initializing matcher for the empty-jobStatus window, excluding the
suspended states:

```jq
(.status.jobStatus // "") == "" and (.status.jobDeploymentStatus // "") != "Suspended" and (.status.jobDeploymentStatus // "") != "Suspending"
```

Karta after: `[Initializing]` for both the provisioning window and PENDING; RUNNING
reads `[Running]`, SUCCEEDED `[Completed]`, and a suspended RayJob `[Suspended]`.

While suspending, the operator reports `jobDeploymentStatus: Suspending` before
`Suspended`; that transient read Undefined too, so Suspended now matches both.

## Recorder robustness note

RayCluster provisioning takes minutes, and a slowly-changing object sitting idle
while other operators churn lets etcd compact its resourceVersion. The recorder's
RetryWatcher then closed with "too old resource version" and failed the flow. The
watch loop now re-lists for a fresh resourceVersion (retrying transient control
plane errors) and resumes, so a slow workload no longer fails to record. This is a
recorder change, not a Karta gap, but it is what let the RayCluster flow observe
the provisioning states above.

## 10. serving.knative.dev/v1 Service: only Running was mapped

The Knative Service definition mapped only Running (Ready=True). While a revision,
route, and ingress come up, Ready is Unknown (reasons OutOfDate, RevisionMissing,
IngressNotConfigured, Uninitialized), so the whole deploy read Undefined; a broken
Service (Ready=False) would too.

CR status:

```json
{"status": {"conditions": [{"type": "Ready", "status": "Unknown", "reason": "RevisionMissing"}]}}
```

Karta before: `[Undefined]`.

Added Initializing for Ready=Unknown and Failed for Ready=False:

```go
Initializing: {ByConditions: [{Type: "Ready", Status: "Unknown"}]},
Failed:       {ByConditions: [{Type: "Ready", Status: "False"}]},
```

Karta after: deploying reads `[Initializing]`, ready `[Running]`, a failed Service
`[Failed]`. The recorder exercises Initializing then Running; Failed is by the
Knative Ready model.

## 11. serving.kserve.io/v1beta1 InferenceService: deploy phase unmapped

The InferenceService definition mapped Running (PredictorReady, RoutesReady,
LatestDeploymentReady all True) and Failed (PredictorReady,
PredictorConfigurationReady, RoutesReady all False) but nothing in between. While
deploying, the controller writes conditions gradually - none at first, then a few
Unknown, then Ready=Unknown - so the whole deploy read Undefined.

CR status (an early step; the first step has no conditions at all):

```json
{"status": {"conditions": [{"type": "Ready", "status": "Unknown"}, {"type": "PredictorReady", "status": "Unknown"}]}}
```

Karta before: `[Undefined]`.

Added Initializing for "Ready not yet decided" - no Ready=True and no Ready=False,
which also covers the early window before Ready is written:

```jq
(([.status.conditions[]? | select(.type == "Ready" and .status == "True")] | length) == 0)
and
(([.status.conditions[]? | select(.type == "Ready" and .status == "False")] | length) == 0)
```

Karta after: the deploy reads `[Initializing]`, ready `[Running]`, and the
all-False predictor failure `[Failed]` (Ready is False there, so it stays out of
Initializing). Recorder side uses a matching CondPending predicate.

## 12. milvus.io/v1beta1 Milvus: Initializing misses the just-created state

The Milvus definition mapped Initializing to the "Pending" phase, Running to
"Healthy", and Degraded to "Unhealthy" (status.status). But a just-created Milvus
has no status.status yet, so the first steps read Undefined before the operator
writes "Pending".

CR status:

```json
{"status": {}}
```

Karta before: `[Undefined]`.

Added a byExpression for the empty phase alongside the Pending matcher:

```jq
(.status.status // "") == ""
```

Karta after: just-created and Pending read `[Initializing]`, Healthy `[Running]`,
Unhealthy `[Degraded]`.

## 13. nvidia.com/v1alpha1 DynamoGraphDeployment: Initializing misses the just-created state

The Dynamo definition mapped Initializing to the "initializing" and "pending"
phases (.status.state), Running to "successful", and Failed to "failed". A
just-created DynamoGraphDeployment has no status.state yet, so the first steps
read Undefined before the operator writes "pending".

CR status:

```json
{"status": {}}
```

Karta before: `[Undefined]`.

Added a byExpression for the empty phase alongside the initializing/pending
matchers:

```jq
(.status.state // "") == ""
```

Karta after: just-created, initializing, and pending read `[Initializing]`;
successful `[Running]`; failed `[Failed]`.

## 14. apps.nvidia.com/v1alpha1 NIMService: Initializing misses the just-created state

The NIMService definition mapped Initializing to the "NotReady" and "Pending"
phases (.status.state), Running to "Ready", Failed to "Failed". A just-created
NIMService has no status.state yet, so the first steps read Undefined before the
operator writes "NotReady".

CR status:

```json
{"status": {}}
```

Karta before: `[Undefined]`.

Added a byExpression for the empty phase alongside the NotReady/Pending matchers:

```jq
(.status.state // "") == ""
```

Karta after: just-created, NotReady, and Pending read `[Initializing]`; Ready
`[Running]`; Failed `[Failed]`.

## Coverage and what remains

Fixed and recorded clean: milvus, grove, dynamo (initializing), nim (initializing), the built-ins (batch/v1 Job, apps/v1 Deployment and
StatefulSet, CronJob, Pod), plus JobSet, LeaderWorkerSet, PyTorchJob, MPIJob,
RayCluster, Knative Service, and KServe InferenceService.

Fixed and kread-verified, fixtures partial (the operator is heavy enough to
outlast a kind control plane under record load on this environment): RayCluster
resumed, and all RayJob flows (provisioning and Suspending windows).

Not yet recorded here: the dynamo and nim running flows (its mocker decode worker needs Dynamo's distributed runtime, which the e2e install keeps off; and nim's fictive CPU image never serves, so both stay in Initializing). Their controllers run
inference or database workloads whose smoke or reconcile load crashes a kind
control plane during provisioning on this environment, so they could not be driven
end to end. Their definitions already map Initializing, Running, Failed, and
(where relevant) Degraded, so they are the lower-risk set; recording them needs a
sturdier cluster than kind-on-Docker-Desktop.
