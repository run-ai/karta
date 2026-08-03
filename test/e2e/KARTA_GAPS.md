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
