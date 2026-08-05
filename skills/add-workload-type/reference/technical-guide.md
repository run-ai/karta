<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta technical guide (cheatsheet)

A condensed field reference for authoring a Karta definition. It matches the API
types in `pkg/api/runai/v1alpha1/` and the validator in
`pkg/api/runai/v1alpha1/validation.go`. The prose reference is
`docs/Technical Guide.md`.

## Top-level shape

```yaml
apiVersion: run.ai/v1alpha1
kind: Karta
metadata:
  name: <lower-case-name>
spec:
  structureDefinition:
    rootComponent: {}          # required
    childComponents: []        # optional
    additionalChildKinds: []   # optional
  optimizationInstructions: {} # optional
```

## Component model

A component is one node in the workload tree.

- Root component: exactly one. Requires a full GVK and a `statusDefinition`. Must
  not have an `ownerRef`.
- Child component: requires an `ownerRef` naming another component (root or
  another child). Owner chains must reach the root with no cycles.
- Component `name` is a free-form identifier, unique within the Karta.
- `kind` is the full GVK. All of group, version, and kind are required. The core
  `Pod` kind is the only kind allowed to omit the group (use `group: ""`).

Fields available on a component (`ComponentDefinition`):

| Field | Purpose |
|---|---|
| `name` | Unique identifier. Required. |
| `kind` | Full GVK. Required on root; recommended on children. |
| `ownerRef` | Parent component name. Required on children, forbidden on root. |
| `specDefinition` | Where the pod template lives. |
| `scaleDefinition` | Where replica counts live. |
| `statusDefinition` | Status mapping. Required on root. |
| `suspendDefinition` | Native suspend/resume field assignments. |
| `instanceIdPath` | jq path to instance names for multi-instance components. |
| `podSelector` | How pods map to this component and its instances. |

## Spec definitions (mutually exclusive)

Set exactly one of these three per component. Setting more than one fails
validation with `has multiple pod spec definitions`.

| Pattern | Use when | Example |
|---|---|---|
| `podTemplateSpecPath` | CRD embeds a full PodTemplateSpec | `.spec.template` |
| `podSpecPath` (+ `metadataPath`) | CRD embeds a bare PodSpec, metadata separate | `.spec.jobTemplate.spec`, `.spec.jobTemplate.metadata` |
| `fragmentedPodSpecDefinition` | Pod fields scattered across the spec | see below |

`fragmentedPodSpecDefinition` fields (all optional; set only those that exist):
`schedulerNamePath`, `labelsPath`, `annotationsPath`, `resourcesPath`,
`resourceClaimsPath`, `podAffinityPath`, `nodeAffinityPath`, `containersPath`,
`containerPath` (single container), `priorityClassNamePath`, `imagePath`.

```yaml
specDefinition:
  fragmentedPodSpecDefinition:
    labelsPath: .spec.labels
    resourcesPath: .spec.resources
    containerPath: .spec.components[] | .podTemplate.spec.containers[] | select(.name == "main")
```

Fragmented paths must be assignable. Every `fragmentedPodSpecDefinition` path is
used both to read the field and to write it back when a consumer mutates the pod
spec, so each path must be a jq path expression that jq can assign through.
Navigation (`.a.b`), array iteration (`.items[]`), and path-preserving filters
(`select(...)`) are assignable. A `//` fallback is not: `.a // .b` produces
values, not a path, so it reads fine but fails on mutation. This passes both the
schema and the jq safety validator, so it is a silent trap. When a field can be
overridden (for example a workflow-level default plus a per-template override),
target one layer with an assignable path rather than a fallback expression, and
model per-item variation with a multi-instance component (`instanceIdPath`) so
each item's field stays index-aligned and assignable.

## Status definition

Required on the root component. Optional on children. Structure:

```yaml
statusDefinition:
  conditionsDefinition:      # needed only if any rule uses byConditions
    path: .status.conditions
    typeFieldName: type      # defaults: type / status / message / reason
    statusFieldName: status
    reasonFieldName: reason
    messageFieldName: message
  phaseDefinition:           # needed only if any rule uses byPhase
    path: .status.phase
  statusMappings:            # required
    running:
    - byConditions:
      - type: Ready
        status: "True"
```

Normalized statuses (the `ResourceStatus` enum): `Initializing`, `Running`,
`Completed`, `Failed`, `Degraded`, `Suspended`, `Suspending`, `Resuming`.
`Undefined` is the implicit result when no rule matches; do not map to it.

Matcher semantics (`StatusMatcher`):

- `byConditions`: a list of expected conditions, all of which must hold (AND).
  Each entry sets `type` plus at least one of `status` or `reason`.
- `byPhase`: matches a single phase string from `phaseDefinition.path`.
- `byExpression`: a jq `expression` plus an `expectedResult` string. Use it when
  the state lives in status fields (for example replica counts) rather than
  conditions or a phase.
- Rules under one status are OR'd: any matching rule resolves the status.
- Several statuses can match at once. Map only what the workload reports.

One matcher may combine kinds. A single `StatusMatcher` can set more than one of
`byPhase`, `byConditions`, and `byExpression` at once, and then all of them must
hold (AND). Use this when a status needs both a phase and an extra field check.
This is distinct from listing separate rules under a status, which are OR'd.

Not every controller has a phase or conditions. Some report only replica counts
or other status fields (for example Grove PodCliqueSet has no aggregate phase).
Do not invent a phase or a condition type the controller never sets: that
produces a definition that validates but never resolves. Match such states with
`byExpression` over the real status fields, for example
`(.status.availableReplicas // 0) >= (.spec.replicas // 0)` for running.

Example combining expression and condition rules:

```yaml
statusMappings:
  running:
  - byExpression:
      expression: (.status.active // 0) > 0 and (.status.ready // 0) > 0
      expectedResult: "true"
  completed:
  - byConditions:
    - type: Complete
      status: "True"
```

## Scale definition

```yaml
scaleDefinition:
  replicasPath: .spec.parallelism // 1
  minReplicasPath: .spec.minReplicas
  maxReplicasPath: .spec.maxReplicas
```

All three paths are optional. Keep them null-safe.

A component's replica count is the number of that component's own instances, not
the pod count beneath it. This matters for grouped or replicated workloads, where
a spec often has two different numbers: how many groups or replicas exist, and
how many members each contains. For a component that models the group or replica
level, `replicasPath` is the group or replica count, not the members-per-group.
LeaderWorkerSet is the canonical trap: `.spec.replicas` is the number of groups
and `.spec.leaderWorkerTemplate.size` is pods per group, so the `group`
component scales on `.spec.replicas // 1`, its `leader` scales on
`.spec.replicas // 1` (one leader per group), and its `worker` count is derived,
`(.spec.replicas // 1) * ((.spec.leaderWorkerTemplate.size // 1) - 1)`. Using
`size` for the group is a valid jq path that returns the wrong number, so the
validator will not catch it: check the field's real meaning in the workload API.
A quick self-check is internal consistency: sibling components that model the
same level (here `group` and `leader`) should resolve to the same count.

## Suspend definition

For workloads with native suspend support (for example `.spec.suspend` on a
Job). Both action lists require at least one entry.

```yaml
suspendDefinition:
  suspendActions:
  - path: .spec.suspend
    value: "true"
  resumeActions:
  - path: .spec.suspend
    value: "false"
```

`value` is a JSON-encoded string (`"true"`, `"0"`, `"paused"`, `"null"`).

## Pod selectors (paths run against pod manifests)

```yaml
podSelector:
  componentTypeSelector:        # maps a pod to this component type
    keyPath: .metadata.labels["training.kubeflow.org/replica-type"]
    value: worker               # optional; if omitted, only key existence is checked
  componentInstanceSelector:    # splits one component into named instances
    idPath: .metadata.labels["ray.io/group"]
  replicaSelector:              # distinguishes replicas of the same sub-structure
    keyPath: .metadata.labels["leaderworkerset.sigs.k8s.io/group-index"]
```

`componentInstanceSelector` must pair with a component-level `instanceIdPath`,
and vice versa. Selectors of the same kind must be mutually exclusive across
components.

Role-label keys are framework-specific, and differ even between operators from
the same project. Do not copy a selector key from the nearest sample without
checking the target controller's real pod labels. For example the Kubeflow
training-operator (PyTorchJob, TFJob) labels role with
`training.kubeflow.org/replica-type` (values `master`, `worker`), while the
Kubeflow mpi-operator (MPIJob v2beta1) labels role with
`training.kubeflow.org/job-role` (values `launcher`, `worker`). Read the actual
pod labels the controller sets before writing `keyPath`.

Disambiguating roles that share a label. When two components would match the
same pod label, a plain value match is not mutually exclusive. Separate them by
matching on a key that only one role carries, using key existence (omit `value`).
LeaderWorkerSet is the canonical case: both leader and worker pods carry
`leaderworkerset.sigs.k8s.io/worker-index`, so the leader is matched by that
label with `value: "0"`, and the worker is matched by the existence of the
`leaderworkerset.sigs.k8s.io/leader-name` annotation, which only worker pods
have.

```yaml
# leader: value match on the shared label
componentTypeSelector:
  keyPath: .metadata.labels["leaderworkerset.sigs.k8s.io/worker-index"]
  value: "0"
# worker: key existence of a role-specific annotation (no value)
componentTypeSelector:
  keyPath: .metadata.annotations["leaderworkerset.sigs.k8s.io/leader-name"]
```

## Multi-instance components

When one component holds several specs (an array or a map), give it an
`instanceIdPath` so each instance is distinguishable, and a matching
`componentInstanceSelector` on the pod side.

```yaml
# array of specs
instanceIdPath: .spec.workerGroupSpecs[].groupName
# map of specs
instanceIdPath: .spec.services | to_entries[] | .key
```

## Additional child kinds

List GVKs the workload creates or manages that are not modeled as components.
Used for RBAC. Avoid duplicating a kind already declared as a component, unless
the kind must also be listed here for RBAC or owner traversal (the validator does
not reject it).

```yaml
additionalChildKinds:
- group: apps
  version: v1
  kind: Deployment
```

## Optimization instructions (paths run against pod manifests)

Optional, used by schedulers. Prefer the `podGroup` mapping; the `podGroups`
list format is deprecated but still appears in samples. Every member or subgroup
`componentName` must name a defined component.

```yaml
optimizationInstructions:
  gangScheduling:
    podGroup:
      name: job
      subGroups:
      - componentName: worker
```

## jq safety rules

Every path is validated statically. These constructs are rejected:

- Assignment and update operators (`=`, `|=`, `+=`, `-=`, and the rest).
- The `del` function.
- The recursive descent operator `..`.
- Unbounded builtins: `range`, `paths`, `recurse`, `walk`, `repeat`.

Rules for correct paths:

- Absolute, starting with `.`.
- Null-safe with `//` defaults for any field that may be absent.
- Evaluated against the correct resource (workload object vs pod manifest).

## Validation checklist

- All kinds use a full GVK (only `Pod` may omit the group).
- Root has a `statusDefinition` and no `ownerRef`.
- Every child has an `ownerRef` to an existing component; no ownership cycles.
- Component names are unique and non-empty.
- No component sets more than one spec pattern.
- `instanceIdPath` and `componentInstanceSelector` are both present or both absent.
- Pod selectors reference pod fields; selectors of the same kind are mutually exclusive across components.
- Status conditions and phases match the workload's real API.
- No redundant duplicate kinds in `additionalChildKinds` (duplicates are allowed only when needed for RBAC or owner traversal).
- Every gang-scheduling member names a defined component.
