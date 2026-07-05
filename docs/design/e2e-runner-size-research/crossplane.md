<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# crossplane: e2e runner specs

## TL;DR

Crossplane runs its e2e (kind-based, e2e-framework) tests on the standard
GitHub-hosted small runner `ubuntu-24.04`. This is hosted-small: a 2-vCPU / 7 GB
runner, not a larger GitHub runner and not self-hosted. There is no `cores`,
`large`, or `self-hosted` label anywhere in the workflows. Scale comes from a
matrix of jobs (each on its own small runner) plus a retry wrapper, not from a
bigger machine. The kind cluster is a single node (one control-plane, zero
workers).

Evidence: `.github/workflows/ci.yml:118` (`runs-on: ubuntu-24.04` for the
`e2e-tests` job) and `test/e2e/manifests/kind/kind-config.yaml` (one node).

## How it works

The e2e job lives in the `CI` workflow and is named `e2e-tests`. It runs the
e2e binary via Nix (`nix run .#e2e`) outside the Nix sandbox because it needs
Docker.

- Runner: `runs-on: ubuntu-24.04` (`.github/workflows/ci.yml:118`). This is the
  default GitHub-hosted Ubuntu runner size (2 vCPU, 7 GB RAM, ~14 GB SSD).
- Parallelism comes from a matrix, not a bigger box. `strategy.matrix`
  (`.github/workflows/ci.yml:119-144`) fans out over `test-area` values
  (apiextensions, apiextensions-legacy, pkg, protection, lifecycle) with a
  `base` test-suite, plus several `include` combinations (function-response-cache,
  package-dependency-updates, package-signature-verification, ops, mrap). Each
  matrix cell is a separate job on its own `ubuntu-24.04` runner.
- `fail-fast: false` (`ci.yml:120`) lets all matrix cells run to completion even
  if one fails.
- Flake tolerance: the actual test command is wrapped in `nick-fields/retry`
  with `timeout_minutes: 45` and `max_attempts: 3` (`ci.yml:178-188`). The
  command passes `-test.failfast -fail-fast` plus per-cell
  `--test-suite` and `-labels area=...` so each runner only executes its slice of
  the suite.
- Test framework: `sigs.k8s.io/e2e-framework v0.6.0` (`go.mod:30`), imported in
  `test/e2e/main_test.go:28-34`, including `support/kind` and `third_party/helm`.
- Cluster shape: the kind cluster is a single node. `kind-config.yaml` declares
  exactly one node with `role: control-plane` and no worker nodes
  (`test/e2e/manifests/kind/kind-config.yaml:4-5`). The config is wired in at
  `test/e2e/main_test.go:97-99` via `kind.NewProvider()` pointing at
  `./test/e2e/manifests/kind/kind-config.yaml`. The single node also mounts an
  audit-policy file and an audit-log directory for API-server auditing.
- Caching: `cachix/install-nix-action` and `cachix/cachix-action`
  (`ci.yml:150-157`) pull prebuilt Nix artifacts from the `crossplane` Cachix
  cache, which offsets the small runner by avoiding local rebuilds.

Other jobs in the same workflow (check-diff, lint, unit tests, build, publish)
also use `ubuntu-24.04` (`ci.yml:23,42,61,91,212,310,336`). The only exception
in the whole `.github/workflows/` tree is `renovate.yml:23`
(`runs-on: ubuntu-latest`), which is unrelated to e2e.

## Relevance to Karta

Karta is deciding how large its e2e (kind-based) CI runner should be. Crossplane
is a directly comparable data point: a mature, high-traffic Kubernetes project
using kind plus e2e-framework, and it ships on the free/standard small hosted
runner. That says a single-node kind cluster plus a controller and a moderate
test suite fits inside 2 vCPU / 7 GB when the work is split across a matrix and
protected by retries. Karta does not need a larger or self-hosted runner as a
starting point.

## Evidence

All paths are relative to the crossplane repo root
(`~/workspace/crossplane`). GitHub base:
`https://github.com/crossplane/crossplane` (default branch `main`).

- `.github/workflows/ci.yml:118` - `runs-on: ubuntu-24.04` for job `e2e-tests`.
  https://github.com/crossplane/crossplane/blob/main/.github/workflows/ci.yml
- `.github/workflows/ci.yml:116-205` - full e2e job: matrix, retry wrapper,
  `nix run .#e2e`, artifact/flake upload.
- `.github/workflows/ci.yml:119-144` - matrix over `test-area` / `test-suite`
  with `fail-fast: false`.
- `.github/workflows/ci.yml:178-188` - `nick-fields/retry`,
  `timeout_minutes: 45`, `max_attempts: 3`, e2e command.
- `test/e2e/manifests/kind/kind-config.yaml:1-30` - single control-plane node,
  no workers; audit-log mounts.
  https://github.com/crossplane/crossplane/blob/main/test/e2e/manifests/kind/kind-config.yaml
- `test/e2e/main_test.go:28-34` - imports `sigs.k8s.io/e2e-framework/...`
  (kind, helm support).
  https://github.com/crossplane/crossplane/blob/main/test/e2e/main_test.go
- `test/e2e/main_test.go:97-99` - `kind.NewProvider()` using
  `./test/e2e/manifests/kind/kind-config.yaml`.
- `go.mod:30` - `sigs.k8s.io/e2e-framework v0.6.0`.
  https://github.com/crossplane/crossplane/blob/main/go.mod
- Grep result: no `cores`, no `large`, no `self-hosted` label in any file under
  `.github/workflows/`. The only non-`ubuntu-24.04` runner in the tree is
  `renovate.yml:23` (`ubuntu-latest`), which is not the e2e job. Absence of a
  larger-runner label is itself a finding: crossplane deliberately stays on the
  small hosted runner.

## Lessons for Karta

- Start on the standard hosted runner (`ubuntu-24.04` or `ubuntu-latest`).
  Crossplane proves it is enough for kind plus e2e-framework at scale.
- Split the suite with a job matrix rather than buying a bigger machine. Each
  cell gets its own fresh 2-vCPU runner, so total throughput scales with the
  number of matrix cells at no per-runner cost.
- Wrap the flaky e2e step in a retry action with a per-attempt timeout
  (crossplane uses 45 min, 3 attempts) instead of raising job-level timeouts.
- Keep the kind cluster minimal. A single control-plane node is sufficient;
  worker nodes are not required for controller e2e tests.
- Use a build/artifact cache (crossplane uses Cachix for Nix) so the small
  runner spends its budget on the test, not on rebuilding.
- Set `fail-fast: false` so one flaky area does not mask results from the rest
  of the matrix.

## What NOT to copy

- Do not copy the Nix toolchain (`nix run .#e2e`, Cachix) unless Karta already
  commits to Nix. It is crossplane-specific plumbing, not a runner-sizing
  decision. Karta can get the same small-runner outcome with plain
  `go test` plus a kind action.
- Do not pin to `ubuntu-24.04` as a hard requirement. That exact version is a
  crossplane choice; `ubuntu-latest` is fine for Karta and tracks upgrades
  automatically.
- Do not copy the API-server audit-log mounts in the kind config unless Karta
  actually asserts on audit output. They add setup surface for no benefit
  otherwise.
- Do not assume the matrix `test-area` / `test-suite` names or the
  `-prior-crossplane-version` upgrade logic transfer. Those are crossplane
  domain specifics; only the matrix-plus-retry pattern is worth reusing.
