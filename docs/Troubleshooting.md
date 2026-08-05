<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# Troubleshooting Karta Definitions

This catalog maps the errors you are most likely to hit when authoring a Karta
definition to their cause and fix. Errors fall into four groups:

- Structure validation errors, raised by the Karta validator.
- jq path errors, raised when a path fails to parse, compile, or execute.
- Accessor errors, raised at runtime when code reads a definition that is
  missing a required part or a pod cannot be matched to an instance.
- Silent mistakes that pass validation but produce wrong behavior.

New to authoring? Start with [Authoring Your First Karta](./Authoring%20Your%20First%20Karta.md).
For the full field reference, see the [Technical Guide](./Technical%20Guide.md).

## Structure validation errors

These come from the validator that runs over the whole definition. Several can
be reported at once; the validator joins them, so fix each named component.

| Error message | Cause | Fix |
|---|---|---|
| `root component must have full kind (group, version, kind)` | The root component is missing one of group, version, or kind. | Provide all three under `kind`. Only the core `Pod` kind may omit the group. |
| `root component must have status definition` | The root component has no `statusDefinition`. | Add a `statusDefinition` to the root. It is required so consumers can read normalized status. |
| `root component cannot have owner ref` | An `ownerRef` was set on the root component. | Remove `ownerRef` from the root. Only child components have owners. |
| `child component '<name>' has no owner ref` | A child component is missing `ownerRef`, or it is empty. | Set `ownerRef` to the `name` of the parent component. |
| `child component '<name>' has owner ref to non-existing component '<owner>'` | `ownerRef` points at a name that is not defined in this Karta. | Correct the `ownerRef` to match an existing component `name`. Watch for typos and renamed components. |
| `component name <name> is not unique` | Two components share the same `name`. | Give every component a unique `name`. |
| `component name is empty` | A component has no `name`. | Add a non-empty `name`. |
| `component '<name>' has multiple pod spec definitions` | A `specDefinition` sets more than one of `podTemplateSpecPath`, `podSpecPath`, or `fragmentedPodSpecDefinition`. | Keep exactly one. These three patterns are mutually exclusive per component. |
| `component '<name>' has instance id path but no pod component instance selector` | `instanceIdPath` is set, but the component's `podSelector` has no `componentInstanceSelector`. | Add a `componentInstanceSelector` so pods can be matched to instances, or remove `instanceIdPath` if the component is not multi-instance. |
| `component '<name>' has pod component instance selector but no instance id path` | A `componentInstanceSelector` is set, but the component has no `instanceIdPath`. | Add `instanceIdPath` pointing at where instance names live, or remove the instance selector. |
| `ownership cycle detected involving component <name>` | Components form a loop through their `ownerRef` chain instead of reaching the root. | Break the cycle. Every child's owner chain must terminate at the root component. |
| `pod-group member component '<name>' is not defined (should be a root or child component)` | A `gangScheduling` pod-group member names a component that does not exist. | Make each `componentName` match a defined root or child component. |
| `karta is nil` | The validator was given no definition to validate. | Ensure the definition was parsed and loaded before validation. Usually indicates an empty or unreadable file. |

## jq path errors

Every path in a Karta is a jq expression. These errors name the exact
expression that failed, so search your definition for that string.

| Error message | Cause | Fix |
|---|---|---|
| `failed to parse JQ expression '<expr>': ...` | The expression is not syntactically valid jq. | Fix the syntax. Common causes: unbalanced brackets or quotes, a missing leading `.`, or using `["key"]` with the wrong quote style. |
| `failed to compile JQ expression '<expr>': ...` | The expression parses but cannot compile, for example it references an unknown function. | Use only standard jq builtins. Re-check function names and argument counts. |
| `JQ execution error for expression '<expr>': ...` | The expression compiled but failed while running against real data, often a null traversal. | Make the path null-safe. Supply defaults with `//`, for example `(.status.active // 0)` or `.spec.parallelism // 1`. |

Karta also validates every expression statically before running it. The
validator walks the parsed jq AST and rejects anything that mutates or can
produce unbounded output: assignment and update operators (`=`, `|=`, `+=`,
and the rest), the `del` function, the recursive descent operator `..`, and
unbounded builtins such as `range`, `paths`, `recurse`, `walk`, and `repeat`.

| Error message | Cause | Fix |
|---|---|---|
| `JQ expression '<expr>' at '<path>' failed validation: modifying operator '<op>' is not allowed` | The expression uses an assignment or update operator. | Karta paths read state; they never write. Rewrite the expression so it only reads. |
| `... failed validation: del function is not allowed` | The expression deletes fields. | Same as above: read, do not modify. |
| `... failed validation: recursive descent operator '..' is not allowed` | The expression uses `..`, which walks the whole document. | Spell out the absolute path to the field instead. |
| `... failed validation: function '<name>' may produce excessive output and is not allowed` | The expression uses an unbounded builtin (`range`, `paths`, `recurse`, `walk`, `repeat`). | Use a bounded expression that addresses the fields directly. |

Tip: test an expression against a real workload manifest with the `jq` CLI
before putting it in a definition. `kubectl get <resource> -o json | jq '<expr>'`
reproduces what Karta evaluates. The official jq playground at
[play.jqlang.org](https://play.jqlang.org/) works too, without leaving the
browser.

## Accessor errors at runtime

These are raised by the Go Component API when code reads a part of a definition
that is not present, or when a pod cannot be tied to a component instance.

| Error type | Example message | Cause | Fix |
|---|---|---|---|
| `DefinitionNotFoundError` | `component <name> does not have suspendDefinition` | Code asked for a part (spec, scale, status, pod template, pod metadata, fragmented pod spec, or suspend) that the component does not define. | Add the missing definition to the component, or guard the call. Use `errors.As` against `DefinitionNotFoundError` to treat "not defined" as an expected case rather than a failure. |
| `InstanceNotFoundError` | `could not match instance id "<id>". existing instance ids [...]` | A pod's extracted instance id does not match any instance id produced by the component's `instanceIdPath`. | Confirm the `componentInstanceSelector` reads the same id that `instanceIdPath` produces. A mismatch usually means the selector points at the wrong pod label or the id path extracts a different field. |

## Silent mistakes (valid but wrong)

These pass validation but lead to wrong status, missing pods, or no effect. They
are worth checking first when a definition "works" but behaves incorrectly.

- Using `ownerName` instead of `ownerRef`. The API field is `ownerRef`. A child
  written with `ownerName` decodes with no owner set. The validator rejects it
  with `child component '<name>' has no owner ref`, but validation runs only
  where `KartaValidator` is invoked, so a code path that skips validation sees
  a silently unowned child. Use `ownerRef`, and run the validator when loading
  definitions.
- Expecting a `referencedComponents` field. The structure has only
  `rootComponent`, `childComponents`, and `additionalChildKinds`. There is no
  `referencedComponents`. Model owned resources as child components and list
  other managed kinds under `additionalChildKinds`.
- Status conditions that do not match the workload's real API. The definition
  validates, but status never resolves because the controller never sets those
  condition types. Verify condition types and field names against the workload's
  CRD source or documentation.
- Paths evaluated against the wrong resource. Paths in `specDefinition`,
  `scaleDefinition`, and `statusDefinition` run against the workload object.
  Paths in `podSelector` and `optimizationInstructions` run against pod
  manifests. A selector that points at a field on the workload object instead of
  the pod will match nothing.
- Listing an explicitly defined component's kind under `additionalChildKinds`.
  `additionalChildKinds` is only for managed kinds that are not already a child
  component. Duplicating a defined kind is redundant and flagged by the
  "no duplicated child kinds" check.

## Still stuck

- Re-run the [validation checklist](./Technical%20Guide.md#validation-checklist)
  in the Technical Guide.
- Compare your definition against the closest one in
  [`docs/catalog/`](./catalog/).
- Exercise the definition with the offline quickstart at
  [`docs/examples/quickstart/`](./examples/quickstart/) to see what the uniform
  API reads back.
