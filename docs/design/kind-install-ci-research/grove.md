<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# grove: installing kind in CI

Repo: ai-dynamo/grove. Default branch: main.
GitHub base: https://github.com/ai-dynamo/grove/blob/main

## TL;DR

Grove has two separate paths, and the important one is not kind.

- CI e2e clusters use k3d, not kind. The GitHub Actions e2e jobs build a
  k3d cluster through a Python framework (`infra_manager`), driven by
  Makefile targets like `run-e2e-full` and `scale-cluster-up`.
- kind exists only for local developer use (`make kind-up`). The kind
  binary is downloaded with `curl` from the official release URL and
  pinned to `KIND_VERSION ?= v0.30.0` in `hack/tools.mk`. Grove does not
  use `helm/kind-action`, `engineerd/setup-kind`, or
  `go install sigs.k8s.io/kind`.
- In CI, kind, kubectl, and helm are not installed by a marketplace
  action. A composite action (`.github/actions/e2e-setup`) installs k3d,
  skaffold, and helm with `curl` install scripts, but only if missing,
  because the self-hosted runner image already has them baked in.
- Version pinning is split: kind is pinned in `hack/tools.mk`; k3s image
  is pinned in Python constants; helm and k3d in CI are unpinned
  (installed from `main`/`latest`).

## How it works

### The kind path (local dev only)

`hack/tools.mk` declares the kind binary target. It downloads a prebuilt
release binary with curl and pins the version.

- `KIND_VERSION ?= v0.30.0` (line 36).
- Download rule (lines 76-78):
  `curl -Lo $(KIND) https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-$(SYSTEM_NAME)-$(SYSTEM_ARCH)`.
- `SYSTEM_NAME` and `SYSTEM_ARCH` are derived from `uname` (lines 17-18),
  with x86_64 mapped to amd64 and aarch64 to arm64.

The Makefile wires kind into `make kind-up` / `make kind-down`, which
depend on the `$(KIND)` and `$(YQ)` binary targets, so the binaries are
fetched on demand (`operator/Makefile` lines 248-255).

`operator/hack/kind-up.sh` is the cluster creator for that path. It does
not call any GitHub action. It shells out to `kind create cluster` with a
generated config (lines 130-158). The kind node image is pinned inline in
the generated config: `image: kindest/node:v1.35.1` (line 116). It also
sets up a local docker registry and optional KWOK fake nodes.

No GitHub Actions workflow calls `make kind-up`, `kind-up.sh`, or
`kind create cluster`. The kind path is invoked by humans, not CI.

### The k3d path (actual CI e2e)

The real e2e job in `.github/workflows/build-check-test.yaml` (the `e2e`
job, lines 103-179) runs on a self-hosted runner (`prod-grove-e2e-v1`).
It does the tool setup with a local composite action and then runs
`make run-e2e-full`, which creates a k3d cluster.

- Tool setup: `uses: ./.github/actions/e2e-setup` (line 154).
- Cluster creation and test run:
  `make ${{ matrix.make_target || 'run-e2e-full' }} ...` (line 159).
- Cleanup: `make e2e-cluster-down` (line 169), which the diagnostics call
  "k3d cluster".

`make run-e2e-full` depends on `e2e-cluster-up`, then runs tests, then
`e2e-cluster-down` (`operator/Makefile` lines 150-156). `e2e-cluster-down`
runs `infra-manager.py delete k3d-cluster` (line 143). Multiple Makefile
comments say "k3d cluster" (for example lines 134, 140, 179, 204).

Cluster creation itself is Python. `operator/hack/infra_manager/cluster.py`
calls `sh.k3d("cluster", "create", ...)` (lines 245-279). The k3s node
image is pinned in `operator/hack/infra_manager/constants.py`:
`DEFAULT_K3S_IMAGE = "rancher/k3s:v1.35.5-k3s1"` (with k3d defaults like
`DEFAULT_WORKER_NODES = 30`).

The `scale-test-ci.yaml` workflow follows the same pattern: `e2e-setup`
action, then `make scale-cluster-up`, then a compiled scale test, then
`make scale-cluster-down`. Its failure diagnostics list k3d clusters and
helm releases, confirming k3d is the runtime, not kind (lines 44-98).

### How the e2e-setup composite action installs tools

`.github/actions/e2e-setup/action.yaml` installs Go, k3d, skaffold, helm,
and Python. Each step checks `command -v` first and installs only if the
tool is missing, because the runner image ships them pre-baked. It never
installs kind or kubectl.

- k3d: `curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash`
  (from `main`, unpinned).
- helm: `curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash`
  (the get-helm-3 script from `main`, unpinned, latest helm).
- skaffold: curl of `.../skaffold/releases/latest/...` (latest, unpinned).
- Go: version-checked against an input default and installed via
  `actions/setup-go@v4` only as a fallback.
- kubectl: not installed by this action. It is assumed present (k3d/k3s
  and the runner image provide it; Go e2e tests use client-go, not the
  kubectl CLI, and only diagnostics steps call the kubectl binary).

### Version pinning summary

- kind binary: pinned, `KIND_VERSION ?= v0.30.0` in `hack/tools.mk`.
- kind node image: pinned inline, `kindest/node:v1.35.1` in
  `operator/hack/kind-up.sh`.
- k3s image (CI e2e): pinned, `rancher/k3s:v1.35.5-k3s1` in
  `operator/hack/infra_manager/constants.py`.
- k3d binary (CI): not pinned, installed from `main` install script.
- helm (CI): not pinned, installed from get-helm-3 on `main`.
- kubectl: not installed or pinned by CI.
- App-level deps (kai-scheduler, cert-manager, kwok, pyroscope) are
  pinned centrally in `operator/hack/infra_manager/dependencies.yaml`.

## Relevance to Karta

Grove is a Karta sibling. It shows a Kubernetes operator project that has
outgrown kind for CI and moved to k3d driven by a Python e2e framework,
while keeping a curl-pinned kind path for local dev. The pinning
technique (curl of the official release URL with a Makefile variable) is
directly reusable for Karta if Karta stays on kind. The centralized
`dependencies.yaml` pattern for app-level versions is also transferable
regardless of the cluster tool.

## Evidence

- `hack/tools.mk` lines 17-18, 23, 36, 76-78: kind binary is a curl
  download of the release URL, pinned to `v0.30.0`, arch derived from
  `uname`.
  https://github.com/ai-dynamo/grove/blob/main/hack/tools.mk
- `operator/hack/kind-up.sh` lines 47-60, 108-158: script checks for
  kind/yq/docker, generates config, runs `kind create cluster`; node
  image pinned `kindest/node:v1.35.1` at line 116.
  https://github.com/ai-dynamo/grove/blob/main/operator/hack/kind-up.sh
- `operator/Makefile` lines 134-256: k3d targets (`e2e-cluster-up/down`,
  `run-e2e-full`, `scale-cluster-up`) call `infra-manager.py`; kind
  targets (`kind-up`, `kind-down`) call the shell scripts and depend on
  the `$(KIND)` binary target.
  https://github.com/ai-dynamo/grove/blob/main/operator/Makefile
- `.github/workflows/build-check-test.yaml` lines 103-179: e2e job uses
  the local `e2e-setup` action, runs `make run-e2e-full`, cleans up with
  `make e2e-cluster-down`. No kind, no marketplace kind action.
  https://github.com/ai-dynamo/grove/blob/main/.github/workflows/build-check-test.yaml
- `.github/workflows/scale-test-ci.yaml` lines 32-98: same e2e-setup
  action, `make scale-cluster-up/down`, diagnostics reference k3d and
  helm.
  https://github.com/ai-dynamo/grove/blob/main/.github/workflows/scale-test-ci.yaml
- `.github/actions/e2e-setup/action.yaml`: composite action installs Go,
  k3d (from main), skaffold (latest), helm (get-helm-3 from main),
  Python; all install-if-missing. No kind, no kubectl.
  https://github.com/ai-dynamo/grove/blob/main/.github/actions/e2e-setup/action.yaml
- `operator/hack/infra_manager/cluster.py` lines 240-279: cluster is
  created with `sh.k3d("cluster", "create", ...)` using the pinned k3s
  image.
  https://github.com/ai-dynamo/grove/blob/main/operator/hack/infra_manager/cluster.py
- `operator/hack/infra_manager/constants.py`: `DEFAULT_K3S_IMAGE =
  "rancher/k3s:v1.35.5-k3s1"`, k3d defaults, KWOK version.
  https://github.com/ai-dynamo/grove/blob/main/operator/hack/infra_manager/constants.py
- `operator/hack/infra_manager/dependencies.yaml`: centralized app-level
  version pins (kai-scheduler, cert-manager, kwok, pyroscope).
  https://github.com/ai-dynamo/grove/blob/main/operator/hack/infra_manager/dependencies.yaml

## Lessons for Karta

- Pin the cluster tool binary by downloading the official release with
  curl and a single Makefile variable. Grove does exactly this for kind
  with `KIND_VERSION ?= v0.30.0`. It is simple, cache-friendly, and does
  not depend on a third-party marketplace action.
- Pin the node/server image explicitly (`kindest/node:vX` or
  `rancher/k3s:vX`). This controls the Kubernetes version under test and
  is easy to bump deliberately.
- Centralize app-level dependency versions in one file
  (`dependencies.yaml`) so test code, cluster setup, and image pre-pull
  all read the same source of truth.
- Let a script or framework create the cluster, not a GitHub action. This
  keeps local dev and CI on the same code path (a developer runs the same
  `make` target CI runs), which is a strong reproducibility property.
- Make tool installs idempotent (install-if-missing) so the same setup
  step works on both a baked self-hosted runner and a vanilla runner.

## What NOT to copy

- Do not copy the unpinned helm and k3d installs. The e2e-setup action
  pulls k3d, helm (get-helm-3), and skaffold from `main`/`latest` with
  no version pin. On a vanilla runner this makes CI non-reproducible and
  exposes it to upstream breakage and a curl-pipe-to-bash supply-chain
  path. If Karta adopts curl installs, pin every version.
- Do not assume the self-hosted runner convenience transfers. Grove masks
  the unpinned installs because tools are pre-baked into
  `eks-grove-e2e-runner`. Karta on hosted GitHub runners would actually
  execute those unpinned installs on every run.
- Do not carry two parallel cluster stacks (kind for local, k3d for CI)
  unless justified. Grove has a legacy kind path and a newer k3d path;
  they can drift (different Kubernetes versions, different registry
  setup). Pick one cluster tool for both local and CI if possible.
- Do not rely on kubectl being implicitly present. Grove never installs
  or pins kubectl; it leans on the runner image and client-go. That is a
  hidden dependency Karta should make explicit.
