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
- `scripts/verify/` - the offline harness used in step 7. Validates a definition,
  runs it against a real CR, and checks the extraction against predicted values.

## Workflow

### 1. Gather the target facts first

Do not write anything until these facts are known. Read the target CRD source or
documentation to get them right.

Ask the user for two inputs up front:

- The CRD schema (`kubectl get crd <name> -o yaml`, or the operator's API types).
- At least one real example CR (`kubectl get <kind> <name> -o yaml`), ideally one
  that is running and one that has finished. Step 7 runs the definition against
  it. A definition written from the schema alone is unverified, because a jq path
  can be structurally valid and still point at a field no real object carries.

If the user cannot supply a real CR, continue, but say plainly at the end that
the definition was never exercised and which parts are unverified.

From those inputs, establish:

- The full GVK: group, version, and kind. All three are required (the core
  `Pod` kind is the only one allowed to omit the group).
- The real statuses the controller reports: the exact condition types and their
  status and reason values, or the phase strings it writes to `.status`. Use the
  names the controller actually sets. Inventing condition types produces a
  definition that validates but never resolves a status.
- Where the pod template lives in the spec, and whether the workload has one
  role or several (for example master and worker, or head and worker groups).
- How replicas are expressed, if at all.

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

This checklist is structural only. It confirms the definition is well formed. It
does not prove that any path resolves against a real object, so it cannot replace
step 7.

Before finishing, confirm each item:

- All kinds use a full GVK (only `Pod` may omit the group).
- The root component has a `statusDefinition` and no `ownerRef`.
- Every child component has an `ownerRef` naming an existing component, with no
  ownership cycles.
- Component names are unique and non-empty.
- No component sets more than one of the three spec patterns.
- `instanceIdPath` and a `componentInstanceSelector` are either both present or
  both absent on a component.
- Pod selectors reference pod fields, not workload fields. Selectors of the same
  kind must be mutually exclusive across components so a pod maps to one component
  of that kind; different selector kinds may coexist on a component. This is an
  authoring guideline, not a validator check. Verify role-label keys against the
  controller's real pod labels (they are operator-specific), and when two roles
  share a label, disambiguate by matching a key only one role carries (key
  existence).
- Status conditions and phases match the workload's real API.
- Every gang-scheduling member names a defined component.

If a check fails or a later error appears, look it up in
`reference/troubleshooting.md` by the message text.

### 7. Run the definition against the real CR

Reading the YAML back is not verification. A path can pass every check in step 6,
resolve to null against the real object, and produce a definition that reports
nothing. Prove it against the CR instead.

Use `scripts/verify/`, bundled with this skill. It validates the definition,
builds the workload tree from a real manifest, and prints the extracted status,
replica counts, and containers per component instance, with no cluster involved.
`reference` for its flags and predictions format is `scripts/verify/README.md`.

Predict before running. Writing down the expected values first is the point of
this step: reading the output afterwards invites accepting whatever appears,
while a prediction that disagrees with the extraction is a defect that cannot be
talked away.

1. From the CR, write the values the definition should produce into a predictions
   file: the status, and per component instance the replica count and container
   names. Derive them from the CR's own numbers, never by reading them back out
   of an existing definition.
2. Run it, from `scripts/verify/`:

   ```bash
   go run . --karta <definition.yaml> --workload <real-cr.yaml> \
     --predict <predictions.yaml> --strict
   ```

3. Reconcile every mismatch and warning. A mismatch means either the path is
   wrong or the understanding of the CRD is wrong. Decide which before changing
   anything, and never edit the prediction just to make the run pass.

The definition is done when the command exits 0 with `--strict`: the status
resolved, every component declaring a spec pattern extracted a pod spec with
containers, every `instanceIdPath` produced the instance keys the CR contains,
and every predicted number matched.

Show the user the run output alongside the definition. Keep the predictions file
and any scratch copies out of the repository. When something comes back empty or
wrong, do not adjust the checklist; look the symptom up in
`reference/troubleshooting.md`, fix the path, and run again. If a second example
CR in a different state is available (completed or failed), run against it too to
confirm the other status rules fire.
