---
name: add-workload-type
description: >-
  Author and validate a Karta definition that teaches Karta a new Kubernetes
  workload type. Use when a user wants to add, register, onboard, or support a
  workload framework or CRD in Karta (for example an Argo Workflow, a Volcano
  Job, a SparkApplication, or any custom operator), or to write, fix, or review
  a Karta YAML that maps a workload's status, pod template, and scale. Covers
  choosing the closest sample, picking the correct specDefinition pattern,
  writing null-safe jq paths, mapping real conditions or phases to Karta
  statuses, and self-checking against the validator. Not for consuming an
  existing definition from Go code or operating a live cluster.
license: Apache-2.0
---
<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Add a workload type to Karta

A Karta definition describes one Kubernetes workload type as a tree of
components. Once written, any controller or platform built on the Karta library
reads status, scale, and pod specs for that workload through one uniform API,
with no per-type code. This skill walks through authoring a correct definition
and validating it before use.

Every path in a Karta is a jq expression. Paths in `specDefinition`,
`scaleDefinition`, and `statusDefinition` run against the workload object. Paths
in `podSelector` and `optimizationInstructions` run against pod manifests.
Mixing these up is the most common mistake, so keep it in mind throughout.

## Bundled references

Load these as needed. Do not guess field names or rules; confirm them here.

- `reference/technical-guide.md` - the full field and schema cheatsheet:
  component model, the three spec patterns, status mapping semantics, jq safety
  rules, scale, suspend, multi-instance, gang scheduling, and the checklist.
- `reference/sample-index.md` - a decision table that maps a workload shape to
  the closest existing sample under `docs/samples/`. Start here in step 2.
- `reference/troubleshooting.md` - every validator, jq, and runtime error mapped
  to its cause and fix, plus the mistakes that pass validation but behave wrong.

## Workflow

### 1. Gather the target facts first

Do not write anything until these four facts are known. Read the target CRD
source or documentation to get them right.

- The full GVK: group, version, and kind. All three are required (the core
  `Pod` kind is the only one allowed to omit the group).
- The real statuses the controller reports: the exact condition types and their
  status and reason values, or the phase strings it writes to `.status`. Use the
  names the controller actually sets. Inventing condition types produces a
  definition that validates but never resolves a status.
- Where the pod template lives in the spec, and whether the workload has one
  role or several (for example master and worker, or head and worker groups).
- How replicas are expressed, if at all. For grouped or replicated workloads,
  note that the spec usually holds two different numbers (how many groups or
  replicas, and how many members each has). A component's `replicasPath` is the
  count of that component's own instances, not the pods beneath it, so a
  group-level component scales on the group count, not members-per-group. See the
  scale section of `reference/technical-guide.md`.

### 2. Start from the closest sample

Open `reference/sample-index.md`, find the row that matches the workload shape,
and copy that sample from `docs/samples/` as the starting skeleton. Adapting a
working sample is faster and safer than starting from an empty file. Change the
GVK, the paths, and the status mapping to fit the target.

### 3. Pick one specDefinition pattern per component

The three patterns are mutually exclusive. Set exactly one per component:

- `podTemplateSpecPath` when the CRD embeds a full PodTemplateSpec (metadata and
  spec), for example a Job at `.spec.template`.
- `podSpecPath`, with optional `metadataPath`, when the CRD embeds a bare PodSpec
  and optionally a separate metadata object.
- `fragmentedPodSpecDefinition` when pod fields are scattered across the spec.
  List only the field paths that exist (labels, annotations, resources,
  containers, nodeAffinity, and so on). Each fragmented path is used to mutate
  the field, not only read it, so it must be a path jq can assign through: a `//`
  fallback reads fine but breaks on write. When a field has a default plus an
  override, model the varying items as a multi-instance component
  (`instanceIdPath`) rather than reaching for a fallback. See
  `reference/technical-guide.md`.

A component may also have no spec definition when it exists only to model
ownership or scale. See `reference/technical-guide.md` for the full field list.

### 4. Write null-safe jq paths against the correct resource

- Use absolute paths from the resource root, starting with `.`.
- Supply a default for any field that can be absent, so evaluation never fails on
  null. Examples: `(.status.active // 0)`, `.spec.parallelism // 1`.
- Confirm the resource: spec, scale, and status paths read the workload object;
  selector and optimization paths read a pod manifest.
- Karta rejects jq that can mutate or explode. Do not use assignment or update
  operators, `del`, the recursive descent `..`, or `range`, `paths`, `recurse`,
  `walk`, or `repeat`. Read-only navigation and standard builtins only.
- Test a path with the jq CLI against a real manifest before committing it:
  `kubectl get <resource> -o json | jq '<expr>'`.

### 5. Map real conditions or phases to Karta statuses

`statusDefinition` is required on the root component. It translates the
workload's own conditions or phases into Karta's normalized statuses:
`Initializing`, `Running`, `Completed`, `Failed`, `Degraded`, `Suspended`,
`Suspending`, `Resuming`. A workload that matches no rule resolves to
`Undefined`.

- To match conditions, add `conditionsDefinition` (its path plus field names),
  then use `byConditions`. All conditions in one `byConditions` entry must hold
  (AND). Each entry needs at least a `status` or a `reason`.
- To match a phase string, add `phaseDefinition` (its path), then use `byPhase`.
- When the state lives in status fields rather than conditions or a phase, use
  `byExpression` with a jq expression and an expected result string. Some
  controllers report only status fields (for example replica counts) and no
  aggregate phase; match those with `byExpression`. Do not invent a phase value
  or condition type the controller never sets.
- Rules listed under the same status are OR'd; any one matching resolves that
  status. A single matcher may also combine `byPhase`, `byConditions`, and
  `byExpression`, in which case all of them must hold (AND). Map only the
  statuses the workload actually reports.

### 6. Self-check against the validator

Before finishing, confirm each item:

- All kinds use a full GVK (only `Pod` may omit the group).
- The root component has a `statusDefinition` and no `ownerRef`.
- Every child component has an `ownerRef` naming an existing component, with no
  ownership cycles.
- Component names are unique and non-empty.
- No component sets more than one of the three spec patterns.
- `instanceIdPath` and a `componentInstanceSelector` are either both present or
  both absent on a component.
- Pod selectors reference pod fields, not workload fields, and are mutually
  exclusive across components. Verify role-label keys against the controller's
  real pod labels (they are operator-specific), and when two roles share a label,
  disambiguate by matching a key only one role carries (key existence).
- Status conditions and phases match the workload's real API.
- Every gang-scheduling member names a defined component.

If a check fails or a later error appears, look it up in
`reference/troubleshooting.md` by the message text.

## Exercise the definition without a cluster

The offline quickstart at `docs/examples/quickstart/` loads a Karta definition
with a sample workload and exercises the uniform API: it reads status, scale, and
pod template, mutates pods, and writes them back, all without a cluster. It runs
the validator on load, so it is the fastest way to confirm a new definition is
structurally valid. Use it as the pattern for loading and testing the definition
just written.
