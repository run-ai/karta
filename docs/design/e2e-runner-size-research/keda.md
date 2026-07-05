<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# keda: e2e runner specs

Repo researched: `~/workspace/keda` (kedacore/keda), default branch `main`, origin
`https://github.com/kedacore/keda`. All claims below cite a path read locally.

## TL;DR

- The full e2e suite runs on a custom self-hosted runner label: `runs-on: e2e`
  (`.github/workflows/pr-e2e.yml:206`). This is a bare label, not a GitHub-hosted
  size and not the literal `self-hosted` keyword.
- The e2e job runs inside a container image with pre-baked tooling:
  `container: ghcr.io/kedacore/keda-tools:1.26.2`
  (`.github/workflows/pr-e2e.yml:208`). A comment states this image is built from
  `github.com/test-tools/tools/Dockerfile`
  (`.github/workflows/template-main-e2e-test.yml:10`).
- The e2e job does not spin up a kind cluster. It scales and targets a real,
  long-lived Azure AKS cluster via `make scale-node-pool` and `make e2e-test`
  (`.github/workflows/pr-e2e.yml:242,269`; `Makefile:99-115`).
- No CPU/core count, vCPU, VM SKU, or runner-group is declared for the `e2e`
  label anywhere under `.github/`. That sizing lives outside the repo (in the
  self-hosted runner registration and in AKS). Absence is a finding.
- The nightly reusable e2e workflow is different: it runs on a GitHub-hosted ARM
  runner `runs-on: ubuntu-24.04-arm` in the same `keda-tools` container, and
  also targets the real AKS cluster (`.github/workflows/template-main-e2e-test.yml:9,11`).
- Smoke tests (a lighter path, kind-based or s390x) use `ubuntu-24.04-arm` and a
  custom `s390x` self-hosted label, not the `e2e` label.

## How it works

There are two distinct real-cluster e2e paths plus lighter smoke paths.

1. PR-triggered full e2e (`.github/workflows/pr-e2e.yml`)
   - Triggered by an `issue_comment` of `/run-e2e` from a member of the
     `keda-e2e-test-executors` team (`pr-e2e.yml:3-4,17,24-29`).
   - `triage` and `build-test-images` jobs run on GitHub-hosted runners
     (`ubuntu-latest`, `ubuntu-24.04-arm`) inside the `keda-tools` container
     (`pr-e2e.yml:13-15,72-74`).
   - The actual e2e job `run-e2e-test` uses `runs-on: e2e` with
     `container: ghcr.io/kedacore/keda-tools:1.26.2` (`pr-e2e.yml:204-208`).
   - It scales a shared AKS cluster up (`make scale-node-pool`,
     `NODE_POOL_SIZE: 5`, `TEST_CLUSTER_NAME: keda-e2e-cluster-pr`), then runs
     `make e2e-test` (`pr-e2e.yml:241-269`).
   - `make e2e-test` depends on `get-cluster-context`, which does
     `az login` then `az aks get-credentials` (`Makefile:92-97,111-115`). So the
     `e2e` runner is a self-hosted machine that drives a real Azure AKS cluster;
     the runner itself is not the cluster.

2. Nightly / main e2e (`.github/workflows/template-main-e2e-test.yml`, called by
   `.github/workflows/nightly-e2e.yml`)
   - `runs-on: ubuntu-24.04-arm` (GitHub-hosted ARM), container
     `ghcr.io/kedacore/keda-tools:1.26.2` (`template-main-e2e-test.yml:9-11`).
   - Same pattern: scale node pool to 5, `make e2e-test`, clean up, scale back to
     1 (`template-main-e2e-test.yml:26-47`).

3. Smoke tests (lighter, not the label under study)
   - ARM smoke test uses `ubuntu-24.04-arm` and a real kind cluster via
     `helm/kind-action` (`pr-e2e.yml:383-385,419-423`).
   - s390x smoke uses `runs-on: s390x` (custom self-hosted arch label) and builds
     a kind cluster on the runner (`template-s390x-smoke-tests.yml:9,28-33`;
     `pr-e2e.yml:464-466`).

Container tooling: the same `ghcr.io/kedacore/keda-tools:1.26.2` image is reused
across triage, build, e2e, validation, main-build, and release jobs
(`.github/workflows/pr-validation.yml:14`, `.github/workflows/main-build.yml:19`,
`.github/workflows/release-build.yml:20`,
`.github/workflows/static-analysis-codeql.yml:20`). It is a pre-baked image (Go,
make, az CLI, kubectl, etc. are assumed present; the e2e job never installs them,
unlike the s390x job which apt-installs prerequisites at
`template-s390x-smoke-tests.yml:21-24`).

## Relevance to Karta

Karta's e2e is currently kind-based in CI (per repo commit history:
`feat(e2e): karta e2e testing infrastructure`, `ci(e2e): install and test all
operators in CI`). KEDA shows a second model: a persistent cloud cluster driven
from a self-hosted labeled runner, plus a pre-baked tools container so no per-job
tool install is needed. The label `runs-on: e2e` is the concrete pattern to
compare against if Karta ever needs real-cluster e2e with cloud identity tests.

## Evidence

- `.github/workflows/pr-e2e.yml:206` -> `runs-on: e2e` (the full e2e job).
- `.github/workflows/pr-e2e.yml:208` -> `container: ghcr.io/kedacore/keda-tools:1.26.2`.
- `.github/workflows/pr-e2e.yml:242,269` -> `make scale-node-pool`, `make e2e-test`.
- `.github/workflows/pr-e2e.yml:244` -> `NODE_POOL_SIZE: 5`,
  `TEST_CLUSTER_NAME: keda-e2e-cluster-pr` (`:257`).
- `.github/workflows/template-main-e2e-test.yml:9-11` -> nightly e2e on
  `ubuntu-24.04-arm` in `keda-tools` container; comment: image built from
  `github.com/test-tools/tools/Dockerfile` (`:10`).
- `.github/workflows/nightly-e2e.yml:10-18` -> nightly calls the main e2e,
  s390x, and k8s-versions reusable workflows.
- `.github/workflows/template-s390x-smoke-tests.yml:9` -> `runs-on: s390x`
  (custom self-hosted arch label), installs tools + kind at runtime (`:21-33`).
- `.github/workflows/pr-e2e.yml:385` -> ARM smoke on `ubuntu-24.04-arm`;
  `:419-423` creates a kind cluster.
- `Makefile:92-97` -> `get-cluster-context` does `az aks get-credentials`.
- `Makefile:99-105` -> `scale-node-pool` does `az aks scale --node-count`.
- `Makefile:111-115` -> `e2e-test` runs `go run -tags e2e ./tests/run-all.go`
  against the Azure cluster.
- Absence: grep of `.github/` for `runner-group`, `group:` (as a runner group),
  `cores`, `vcpu`, VM SKUs (`n2-`, `Standard_D`, `m5.`, `c5.`) returns nothing;
  the only `group:` hits are `concurrency.group`
  (`pr-e2e-checker.yml:16`, `pr-validation.yml:6`, `fossa.yml:16`,
  `static-analysis-codeql.yml:13`, `pr-e2e-creator.yml:15`). No core count or VM
  size is declared in-repo for the `e2e` label.

GitHub links (paths verified present locally):
- https://github.com/kedacore/keda/blob/main/.github/workflows/pr-e2e.yml
- https://github.com/kedacore/keda/blob/main/.github/workflows/template-main-e2e-test.yml
- https://github.com/kedacore/keda/blob/main/.github/workflows/nightly-e2e.yml
- https://github.com/kedacore/keda/blob/main/.github/workflows/template-s390x-smoke-tests.yml
- https://github.com/kedacore/keda/blob/main/Makefile

## Lessons for Karta

- A bare custom label (`runs-on: e2e`) is enough to route heavy e2e to dedicated
  self-hosted capacity without a runner-group; the size decision is made where the
  runner is registered, keeping CI YAML simple.
- A single pre-baked tools container (`keda-tools`) reused across all jobs removes
  per-job tool installs and pins tool versions once. Karta could adopt one image
  for kind + kubectl + helm + go instead of installing per job.
- Real-cluster e2e is decoupled from the runner: the runner only needs cloud CLI
  credentials and drives a separate, scalable AKS node pool (scale up to 5 before
  tests, back to 1 after) to control cost.
- Gating e2e behind a team-membership check on a `/run-e2e` comment
  (`pr-e2e.yml:17,24-29`) protects cloud credentials from untrusted PRs.

## What NOT to copy

- Do not assume a portable VM size: the `e2e` label points at KEDA-owned
  self-hosted infrastructure with no spec in-repo. Karta cannot reuse the label;
  it must define its own runner and size.
- Do not couple e2e to a single cloud (Azure AKS + `az` CLI baked into the flow,
  `Makefile:90-105`). Karta's kind-based e2e is more portable and cheaper for a
  young project; only move to a persistent cloud cluster if identity/cloud-integration
  tests demand it.
- Do not depend on an external, separately maintained tools image
  (`github.com/test-tools/tools/Dockerfile`, referenced only by comment at
  `template-main-e2e-test.yml:10`) unless you own and pin it; it is an
  out-of-repo dependency that is invisible in the workflow diff.
