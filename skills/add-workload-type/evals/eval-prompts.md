<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Eval prompts

Three prompts for evaluating the `add-workload-type` skill. Each states the user
input, whether the skill should drive the response, the expected behavior, and
pass criteria. They are model-agnostic and run without a live cluster. The full
NV-CARPS harness (`evals.json` plus `BENCHMARK.md`) is a separate deliverable.

## Eval 1: trigger and discoverability

User prompt:

> We run Argo Workflows on our cluster and want Karta to understand them. Add
> support for the Argo Workflow type.

Skill should drive: yes.

Expected behavior:

- Select the `add-workload-type` skill from the intent (adding a workload type).
- Gather the target facts before writing: the GVK
  (`argoproj.io/v1alpha1`, `Workflow`), how Argo reports status (the
  `.status.phase` string with values such as `Running`, `Succeeded`, `Failed`),
  and where pod templates live.
- Produce a Karta with a valid root component, a `phaseDefinition` mapped to
  Karta statuses via `byPhase`, and null-safe jq paths.
- Not invent condition types that Argo does not set.

Pass criteria:

- The skill is chosen without the user naming it.
- The output has a full GVK and a `statusDefinition` on the root.
- Status mapping uses the real Argo phase values.
- All jq paths are absolute and null-safe.
- The definition satisfies the checklist in the skill.

## Eval 2: correctness on an unseen workload

User prompt:

> Write a Karta definition for a batch/v1 CronJob. It should report status and
> expose the pod template.

Skill should drive: yes.

Expected behavior:

- Identify the CronJob shape from the sample index and start from the closest
  sample (`batch-cronjob-v1` or `batch-job-v1`).
- Model the Job as a child with an `ownerRef` to the CronJob, or justify a
  single-component model.
- Use `podTemplateSpecPath: .spec.jobTemplate.spec.template` (the correct nested
  path), the single valid spec pattern.
- Map status from real fields, for example suspended from `.spec.suspend` and
  active or scheduling state via `byExpression`, all null-safe.

Pass criteria:

- Exactly one spec pattern is set on each component.
- The pod template path is the nested `jobTemplate` path, not `.spec.template`.
- Every jq path is null-safe and reads the workload object, not a pod.
- The definition passes the validation checklist.
- Bonus: it loads cleanly through the offline quickstart validator.

## Eval 3: boundary and non-authoring intent

User prompt:

> What does the Karta offline quickstart print when I run it, and where is its
> source?

Skill should drive: no.

Expected behavior:

- Recognize this as a usage or navigation question, not an authoring task.
- Answer from the quickstart docs and source under `docs/examples/quickstart/`
  without producing a new Karta definition.
- Not walk the authoring workflow or emit a definition skeleton.

Pass criteria:

- The response does not author or scaffold a Karta definition.
- The answer points at `docs/examples/quickstart/` and describes its output.
- The skill does not misfire on an adjacent but distinct intent.
