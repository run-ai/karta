<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# grove: e2e runner specs

Repo researched: `~/workspace/grove` (local only, no network).
Remote: `github.com/ai-dynamo/grove`, default branch `main`.
Link base used below: `https://github.com/ai-dynamo/grove/blob/main/`.

## TL;DR

- The Grove e2e job runs on a custom self-hosted runner label: `runs-on: prod-grove-e2e-v1`
  (`.github/workflows/build-check-test.yaml:112`). This is not `ubuntu-latest` and not the
  generic `self-hosted` label; it is a single custom label that maps to a runner group
  configured outside the repo.
- The runner is pre-baked. A comment in the composite setup action names the runner image
  `eks-grove-e2e-runner` and states that Go, k3d, skaffold, helm, make, and python3 are
  pre-installed; each install step checks first and only installs if missing
  (`.github/actions/e2e-setup/action.yaml:29-31`).
- All other jobs (test, build, check, changes, release, stale, push-artifacts,
  validate-issue-references, scale-test) run on `ubuntu-latest`. Only the e2e matrix job
  uses the self-hosted label.
- The e2e cluster is k3d/k3s. Default cluster size is 30 worker nodes, k3s image
  `rancher/k3s:v1.35.5-k3s1`, 150m memory per node
  (`operator/hack/e2e-cluster/create-e2e-cluster.py:128-130,93`). CI passes
  `--dind-memory-mode`, so the cluster runs Docker-in-Docker and emulates per-node memory
  via kubelet system-reserved (`create-e2e-cluster.py:284-306`).
- The weekly scale test runs on `ubuntu-latest` (not the self-hosted runner) and uses a
  KWOK-backed preset: 0 real worker nodes plus 100 KWOK nodes
  (`.github/workflows/scale-test-ci.yaml:19`, `operator/hack/scale.yaml:5,20`).

## How it works

The e2e job is defined as a matrix in `.github/workflows/build-check-test.yaml`
(job `e2e`, lines 103-180). It is gated: it only runs on pull requests when e2e-relevant
files changed, and either the PR is non-draft or carries the `run-e2e` label
(lines 107-110). The job pins `runs-on: prod-grove-e2e-v1` (line 112), with a
`timeout-minutes: 60` (line 113) and a 9-entry matrix of test suites (lines 116-137).

A sibling job `e2e-skip` (lines 185-199) exists to satisfy required branch-protection
checks on doc-only PRs; it runs on `ubuntu-latest` (line 191) and does no real work.

Tool setup is a composite action at `.github/actions/e2e-setup/action.yaml`. Its header
comment (lines 29-31) says most tools are pre-installed in the runner image
`eks-grove-e2e-runner`, and each step (make, Go, k3d, skaffold, helm, python3) uses a
`command -v ... && ... || install` pattern so it is a no-op on the baked runner and a
fallback installer on a vanilla runner. Go is pinned to 1.25.7 by default (line 24).

The e2e job first prints runner specs (`nproc`, `free -h`) for the record
(build-check-test.yaml:141-144), then pulls a registry image from GHCR to dodge Docker Hub
rate limits (lines 149-152), runs `e2e-setup`, and invokes
`make run-e2e-full ... E2E_CREATE_FLAGS='--dind-memory-mode'` (line 159).

The cluster is created by `operator/hack/e2e-cluster/create-e2e-cluster.py`. Defaults
(lines 128-130): `worker_nodes` = 30 (allowed range 1-100), `worker_memory` = 150m
(constant `DEFAULT_WORKER_MEMORY`, line 93), `k3s_image` = `rancher/k3s:v1.35.5-k3s1`.
Because CI uses `--dind-memory-mode`, `--agents-memory` cannot be used (broken
/proc/meminfo bind-mount in DinD), so the script emulates the per-node memory cap via
`--kubelet-arg=system-reserved=memory=...Mi@agent:*` (lines 284-306). The script also
pre-pulls images in parallel into a local k3d registry to speed startup (lines 187-258).

The weekly scale test (`.github/workflows/scale-test-ci.yaml`) is separate. It runs on
`ubuntu-latest` (line 19), 120-minute timeout, and calls
`make scale-cluster-up` with the preset `operator/hack/scale.yaml`, which sets 0 real
worker nodes and 100 KWOK (simulated) nodes (scale.yaml:5,20). KWOK lets the scale test
fake a large node count on a hosted runner without real kubelets.

## Relevance to Karta

Karta is adding e2e infra (see recent commits `feat(e2e): karta e2e testing infrastructure`
and the CI e2e work on branch `e2e-karta`). Grove is a Karta sibling and shows one concrete
pattern for running real-cluster e2e: a pre-baked self-hosted runner referenced by a single
custom label, with a composite action that degrades gracefully to a hosted runner. Grove
splits real e2e (self-hosted, k3d, 30 nodes) from scale testing (hosted, KWOK, 100 fake
nodes), which is a useful separation if Karta wants both a per-PR gate and a heavy periodic
job.

## Evidence

- `.github/workflows/build-check-test.yaml:112` -> `runs-on: prod-grove-e2e-v1` (the e2e job label).
  https://github.com/ai-dynamo/grove/blob/main/.github/workflows/build-check-test.yaml
- `.github/workflows/build-check-test.yaml:111` -> comment "use NVIDIA self-hosted runner setting is on Velonix repository".
- `.github/workflows/build-check-test.yaml:107-113` -> e2e gating (`if:`), `timeout-minutes: 60`.
- `.github/workflows/build-check-test.yaml:116-137` -> 9-suite matrix.
- `.github/workflows/build-check-test.yaml:141-144` -> "Print runner specs" step (nproc, free -h).
- `.github/workflows/build-check-test.yaml:159` -> `make run-e2e-full ... E2E_CREATE_FLAGS='--dind-memory-mode'`.
- `.github/workflows/build-check-test.yaml:191` -> `e2e-skip` job on `ubuntu-latest`.
- `.github/actions/e2e-setup/action.yaml:29-31` -> runner image `eks-grove-e2e-runner`; tools pre-installed, install-if-missing.
  https://github.com/ai-dynamo/grove/blob/main/.github/actions/e2e-setup/action.yaml
- `.github/actions/e2e-setup/action.yaml:24` -> default Go 1.25.7.
- `operator/hack/e2e-cluster/create-e2e-cluster.py:128` -> `worker_nodes: int = Field(default=30, ge=1, le=100)`.
  https://github.com/ai-dynamo/grove/blob/main/operator/hack/e2e-cluster/create-e2e-cluster.py
- `operator/hack/e2e-cluster/create-e2e-cluster.py:130` -> `k3s_image = "rancher/k3s:v1.35.5-k3s1"`.
- `operator/hack/e2e-cluster/create-e2e-cluster.py:93` -> `DEFAULT_WORKER_MEMORY = "150m"`.
- `operator/hack/e2e-cluster/create-e2e-cluster.py:284-306` -> DinD memory emulation via kubelet system-reserved.
- `.github/workflows/scale-test-ci.yaml:19` -> scale test on `ubuntu-latest`, 120-min timeout.
  https://github.com/ai-dynamo/grove/blob/main/.github/workflows/scale-test-ci.yaml
- `.github/workflows/scale-test-ci.yaml:47` -> `make scale-cluster-up E2E_CREATE_FLAGS='--dind-memory-mode ...'`.
- `operator/hack/scale.yaml:5,20` -> `worker_nodes: 0`, `kwok.nodes: 100`.
  https://github.com/ai-dynamo/grove/blob/main/operator/hack/scale.yaml
- `operator/Makefile:151,208-209` -> `run-e2e-full` and `scale-cluster-up` targets calling `infra-manager.py`.
  https://github.com/ai-dynamo/grove/blob/main/operator/Makefile

Absence findings:
- No `.github/actions/e2e-setup` step declares CPU/core counts or a runner group name in-repo.
  The runner group/hardware spec for `prod-grove-e2e-v1` is configured outside this repo
  (a comment points to the "Velonix repository" GitHub Actions self-hosted runner settings;
  no such config file exists locally).
- No use of the generic `self-hosted` label anywhere; the only match for "self-hosted" is
  the comment on build-check-test.yaml:111. No `runs-on:` uses a `[self-hosted, ...]` array
  or a `group:` runner-group key.

## Lessons for Karta

- A single custom label (`prod-grove-e2e-v1`) is enough to route a job to a pre-baked
  self-hosted runner; the hardware and tool baking live in the runner image / runner-group
  config, not in the workflow. Keep the workflow thin.
- The install-if-missing composite action (`e2e-setup`) is a good portability trick: the
  same action works on the baked runner (fast, no-op installs) and on a vanilla runner
  (self-heals). Karta could copy this pattern so contributors can run e2e locally.
- Printing `nproc`/`free -h` at job start gives a cheap audit trail of what the runner
  actually was when a failure happens. Worth copying.
- Separate the per-PR real-cluster gate from the heavy periodic scale job. Grove gates the
  real e2e behind file-path changes plus a `run-e2e` label, and moves the 100-node scale
  run to a weekly KWOK job on hosted runners. This keeps PR cost bounded.
- KWOK is how Grove fakes large node counts on cheap hosted runners; real e2e uses only 30
  k3d nodes at 150m each. Pick the tool to the goal: correctness (real k3d) vs scale (KWOK).

## What NOT to copy

- Do not hardcode an NVIDIA-internal runner label like `prod-grove-e2e-v1` into Karta,
  and do not reference the internal "Velonix repository" or its runner-group settings in
  Karta code, commits, or docs. Karta is a public Apache 2.0 project; internal
  infrastructure names should not leak into it.
- Do not assume the runner-group hardware (cores, RAM, disk) from Grove. The spec is not in
  the Grove repo, so it cannot be cited or reused. Size Karta's runner from its own e2e
  needs, not from an unstated Grove number.
- Do not depend on a pre-baked custom runner image (`eks-grove-e2e-runner`) as a hard
  requirement. Grove keeps its setup action self-healing so it also runs on hosted runners.
  If Karta bakes an image, keep the same fallback so the pipeline is not tied to private
  infra.
