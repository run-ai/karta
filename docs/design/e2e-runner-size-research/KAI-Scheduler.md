<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# KAI-Scheduler: e2e runner specs

Repo researched locally: `~/workspace/KAI-Scheduler`
Remote: https://github.com/NVIDIA/KAI-Scheduler
Default branch: `main`

## TL;DR

- Every e2e (kind-based, helm/kind-action) job runs on `runs-on: ubuntu-latest`. This is a hosted-small standard GitHub runner. No larger runner, no self-hosted label, no custom label anywhere in `.github/`.
- The PR workflow has two e2e jobs: `e2e-tests` and `e2e-upgrade-tests`, both `ubuntu-latest` (`.github/workflows/on-pr.yaml:236` and `:293`).
- The release workflow runs an `e2e-matrix` job across 11 k8s/feature combinations, all on the same single `runs-on: ubuntu-latest` at the job level with a matrix (`.github/workflows/on-release.yaml:81`).
- The kind cluster comes from `helm/kind-action@v1.14.0` with kind `v0.32.0`; default node image `kindest/node:v1.34.0` (`.github/actions/setup-e2e-cluster/action.yaml:57` and `:18`).
- To fit the standard runner, the workflows relocate Docker data-root to `/mnt` for extra disk. That is the only sizing accommodation; the runner tier itself is unchanged.

## How it works

The e2e path is driven by a composite action plus per-workflow jobs.

Composite action `.github/actions/setup-e2e-cluster/action.yaml`:
- Moves Docker data to `/mnt/docker-data` for disk headroom (`:27`).
- Restores a cached image tarball from `/mnt/images` (`:42`).
- Generates the kind config via `hack/generate-kind-config.sh` (`:48`).
- Creates the cluster with `helm/kind-action@v1.14.0`, kind version `v0.32.0`, `node_image: kindest/node:${{ inputs.k8s_version }}` where `k8s_version` defaults to `v1.34.0` (`:56`, `:18`).
- Deploys a local registry, the fake-gpu-operator (`oci://ghcr.io/run-ai/fake-gpu-operator` v0.0.74), and the kube-prometheus-stack, then installs Go 1.26.1 and ginkgo v2.25.3.

The kind cluster is multi-node, not single-node. `hack/generate-kind-config.sh` writes 1 control-plane, 4 GPU worker nodes, and 1 CPU worker node by default; the `dra-enabled` config adds 2 more DRA worker nodes (`hack/generate-kind-config.sh:75-108`). All of these run inside one `ubuntu-latest` runner as Docker containers.

PR workflow `.github/workflows/on-pr.yaml`:
- `build` (`:158`, `ubuntu-latest`) builds images and the helm chart, saving them to the `/mnt/images` cache.
- `e2e-tests` (`:236`, `ubuntu-latest`) runs ginkgo with label filter `!autoscale && !scale && !upgrade` against `./test/e2e/suites` (`:266`).
- `e2e-upgrade-tests` (`:293`, `ubuntu-latest`) installs the previous release then runs ginkgo with label filter `upgrade` (`:358`).

Release workflow `.github/workflows/on-release.yaml`:
- `build-and-push` (`:17`, `ubuntu-latest`).
- `e2e-matrix` (`:81`, `ubuntu-latest`) with `fail-fast: false` and an 11-entry matrix spanning k8s v1.28.13 through v1.36.1, plus `dra-enabled` variants for v1.32.3 and v1.33.4 (`:82-111`). Each matrix leg is an independent `ubuntu-latest` runner.

## Relevance to Karta

Karta will need e2e coverage for a CRD that inspects and mutates workloads across a live cluster, which is the same kind-on-hosted-runner shape KAI-Scheduler uses. KAI-Scheduler is a direct proof that a nontrivial e2e suite (multi-node kind, helm install, GPU-operator faking, ginkgo suites, and an 11-way version matrix) runs on plain `ubuntu-latest` without paying for larger or self-hosted runners. That keeps CI cost low and avoids self-hosted maintenance, which matters for an Apache 2.0 OSS project where external contributors and forks must be able to run the same CI.

## Evidence

Grep for runner terms across `.github/` returned only `runs-on: ubuntu-latest` and unrelated `labels:`/`RUNNER_TEMP` matches. No `self-hosted`, no `cores`, no `large`, no custom runner label anywhere.

e2e and release `runs-on` values (all `ubuntu-latest`):
- `.github/workflows/on-pr.yaml:236` job `e2e-tests`
- `.github/workflows/on-pr.yaml:293` job `e2e-upgrade-tests`
- `.github/workflows/on-pr.yaml:158` job `build`
- `.github/workflows/on-release.yaml:81` job `e2e-matrix`
- `.github/workflows/on-release.yaml:17` job `build-and-push`

kind / k8s version:
- `.github/actions/setup-e2e-cluster/action.yaml:57` `uses: helm/kind-action@v1.14.0`
- `.github/actions/setup-e2e-cluster/action.yaml:58-62` `version: v0.32.0`, `node_image: kindest/node:${{ inputs.k8s_version }}`
- `.github/actions/setup-e2e-cluster/action.yaml:15-18` `k8s_version` default `v1.34.0`
- `.github/workflows/on-release.yaml:90-111` release matrix k8s versions v1.36.1, v1.35.0, v1.34.0, v1.33.4, v1.32.3, v1.31.6, v1.30.4, v1.29.8, v1.28.13

Sharding / parallelism:
- PR: no matrix shards; two separate e2e jobs (`e2e-tests`, `e2e-upgrade-tests`) run in parallel, each a single runner.
- Release: `e2e-matrix` with `fail-fast: false` and 11 include entries = 11 parallel `ubuntu-latest` runners (`.github/workflows/on-release.yaml:82-111`).

Disk accommodation on the standard runner:
- `.github/actions/setup-e2e-cluster/action.yaml:27-34` and `.github/workflows/on-pr.yaml:190-196` move Docker data-root to `/mnt`.

Multi-node kind cluster (all inside one runner):
- `hack/generate-kind-config.sh:75-108` 1 control-plane + 4 GPU workers + 1 CPU worker (default); +2 DRA workers for `dra-enabled`.

Relevant GitHub links (paths verified to exist locally):
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/.github/workflows/on-pr.yaml
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/.github/workflows/on-release.yaml
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/.github/actions/setup-e2e-cluster/action.yaml
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/hack/generate-kind-config.sh

## Lessons for Karta

- `ubuntu-latest` is enough for a real multi-node kind e2e suite. KAI-Scheduler proves you do not need a larger or self-hosted runner to stand up 6+ kind nodes, install via helm, and run ginkgo suites.
- Push version-matrix breadth to the release workflow, not the PR workflow. PR e2e stays a single default k8s version (v1.34.0) for fast feedback; the wide k8s matrix runs only on release (`.github/workflows/on-release.yaml:82-111`).
- Use `fail-fast: false` on the version matrix so one k8s version failing does not hide results for the others (`.github/workflows/on-release.yaml:83`).
- Factor cluster setup into a composite action so PR and release share one code path (`.github/actions/setup-e2e-cluster/action.yaml`). This keeps the kind version, node image default, and prerequisite installs in one place.
- Reclaim runner disk by moving Docker data-root to `/mnt` before building images and creating the cluster; this is the cheap way to stay on the standard runner.
- Build once, load into a local in-cluster registry. KAI-Scheduler builds images in the `build` job, caches the tarball, and pushes into a kind-hosted registry in the e2e job rather than rebuilding.

## What NOT to copy

- Do not copy the k8s version list verbatim. It is tuned to KAI-Scheduler's support window and DRA behavior (DRA beta only relevant on v1.32/v1.33, GA on v1.34+; see the comment at `.github/workflows/on-release.yaml:86-89`). Karta should pick versions matching its own controller-runtime and supported-k8s policy.
- Do not copy the fake-gpu-operator, GPU node labels, or DRA node config (`hack/generate-kind-config.sh:87-104`, `.github/actions/setup-e2e-cluster/action.yaml:83-91`). Those are GPU-scheduler-specific and irrelevant to a generic workload CRD.
- Do not assume the standard runner is free of size limits. KAI-Scheduler only fits because it moves Docker to `/mnt` and deletes image tarballs mid-run (`.github/actions/setup-e2e-cluster/action.yaml:115-121`). If Karta's images or node count grow, revisit disk and possibly runner size rather than blindly reusing `ubuntu-latest`.
- Do not copy the FOSSA push-only API key pattern (`.github/workflows/on-pr.yaml:366-368`) into Karta's e2e; it is unrelated to e2e and Karta has its own license-check flow (`make validate` / go-licenses per AGENTS.md).
