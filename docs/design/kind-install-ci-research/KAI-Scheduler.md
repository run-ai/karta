<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# KAI-Scheduler: installing kind in CI

Repo: NVIDIA/KAI-Scheduler. Default branch: main.

## TL;DR

- CI installs kind and creates the cluster with the `helm/kind-action@v1.14.0` GitHub Action. There is no curl download, no `go install sigs.k8s.io/kind`, and no `engineerd/setup-kind`.
- The action both installs the kind binary and creates the cluster. It is pinned to action tag `v1.14.0`, kind binary `version: v0.32.0`, and a `node_image` (kindest/node tag) that is parameterized per job.
- kubectl and helm are not explicitly installed in CI. The workflows rely on the tools preinstalled on the `ubuntu-latest` runner (plus the kubeconfig that kind-action wires up). No `azure/setup-helm`, `azure/setup-kubectl`, or `get-helm-3` anywhere.
- Local developer flow is different from CI. The `hack/*.sh` scripts call the `kind` CLI directly (assumed already on PATH) and pin the Kubernetes node image via a shell default `KIND_K8S_TAG`.
- The k8s version tested is a release-time matrix in `on-release.yaml`; the PR flow uses the action default.

## How it works

Two distinct paths exist: CI (GitHub Actions) and local dev (hack scripts).

### CI path (the authoritative one)

The e2e jobs do not install kind inline. They delegate to a composite action.

- `.github/workflows/on-pr.yaml` job `e2e-tests` (line 232) and `e2e-upgrade-tests` (line 289) both call `uses: ./.github/actions/setup-e2e-cluster` (lines 244, 338).
- `.github/workflows/on-release.yaml` job `e2e-matrix` (line 78) calls the same composite action (line 117) once per matrix entry.

The composite action `.github/actions/setup-e2e-cluster/action.yaml` is where kind is installed and the cluster is created:

```yaml
- name: Create k8s Kind Cluster
  uses: helm/kind-action@v1.14.0
  with:
    cluster_name: kind
    version: v0.32.0
    config: /tmp/kind-config.yaml
    node_image: kindest/node:${{ inputs.k8s_version }}
```

That is lines 56-62 of the action. `helm/kind-action` downloads the pinned kind binary, then creates the cluster from the generated config. Before that step, the action generates the cluster topology with `hack/generate-kind-config.sh` (lines 48-54).

The cluster node image tag (`k8s_version` input) defaults to `v1.34.0` in the action (line 18). Callers override it:

- `on-pr.yaml` does not pass `k8s_version`, so PR e2e uses the action default `v1.34.0`.
- `on-release.yaml` passes `k8s_version: ${{ matrix.k8s_version }}` from an explicit matrix spanning `v1.28.13` through `v1.36.1` (lines 90-111).

kubectl and helm are used immediately after (kubectl apply, helm upgrade) inside the same action and in the workflow steps, but no step installs them. They come from the `ubuntu-latest` image and the kubeconfig context set by kind-action.

### Local dev path (not used by CI)

The `hack/` scripts drive kind through the plain CLI, which must already be installed on the developer machine.

- `hack/setup-e2e-cluster.sh` runs `kind create cluster --config ... --image ${KIND_IMAGE} --name ...` (lines 92-95). The node image defaults via `: ${KIND_K8S_TAG:="v1.35.0"}` and `: ${KIND_IMAGE:="kindest/node:${KIND_K8S_TAG}"}` (lines 18-19).
- `hack/run-e2e-kind.sh` wraps that script and later runs `kind delete cluster` (line 67).
- `hack/run-e2e-upgrade-kind.sh` reuses the same setup and also uses `kind load docker-image` as a fallback (line 133) and `kind delete cluster` (line 159).

These scripts do not install kind, kubectl, or helm. They assume the binaries exist. So the local pin (`v1.35.0` node image) is separate from and inconsistent with the CI pin (kind-action default `v1.34.0`, or the release matrix).

## Relevance to Karta

Karta is adding e2e testing infrastructure (branch `e2e-karta`). The choice of how to get kind into CI is directly reusable. KAI-Scheduler shows a clean split: a single composite action owns cluster provisioning for every workflow that needs a cluster, and a generator script owns cluster topology. Karta can copy this shape so PR and release e2e share one provisioning path.

## Evidence

- `.github/actions/setup-e2e-cluster/action.yaml` lines 56-62: `helm/kind-action@v1.14.0` with `version: v0.32.0` and `node_image: kindest/node:${{ inputs.k8s_version }}`.
- `.github/actions/setup-e2e-cluster/action.yaml` line 18: `k8s_version` input default `'v1.34.0'`.
- `.github/actions/setup-e2e-cluster/action.yaml` lines 48-54: generate kind config via `hack/generate-kind-config.sh`.
- `.github/actions/setup-e2e-cluster/action.yaml` lines 64-102: kubectl and helm used without any install step.
- `.github/workflows/on-pr.yaml` lines 244, 338: e2e jobs use the composite action; no `k8s_version` passed.
- `.github/workflows/on-release.yaml` lines 90-121: explicit k8s version matrix passed to the action.
- `hack/setup-e2e-cluster.sh` lines 18-19, 85-95: local `kind create cluster`, node image default `v1.35.0`.
- `hack/run-e2e-kind.sh` lines 56, 64, 67: wraps setup script, runs ginkgo, deletes cluster.
- `hack/generate-kind-config.sh` lines 45-108: builds the multi-node cluster topology YAML.
- `Makefile` (grep for kind/kubectl/helm): no kind/kubectl/helm install target; only a helm-unittest docker run (lines 30-31).
- Repo-wide grep for `setup-helm`, `setup-kubectl`, `get-helm`, `curl ... kind`, `azure/setup`: no matches (absence finding).
- `go.mod`: no `sigs.k8s.io/kind` dependency (absence finding).

Verified links (paths exist locally):

- https://github.com/NVIDIA/KAI-Scheduler/blob/main/.github/actions/setup-e2e-cluster/action.yaml
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/.github/workflows/on-pr.yaml
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/.github/workflows/on-release.yaml
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/hack/setup-e2e-cluster.sh
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/hack/run-e2e-kind.sh
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/hack/generate-kind-config.sh
- https://github.com/NVIDIA/KAI-Scheduler/blob/main/Makefile

## Lessons for Karta

- Use `helm/kind-action` for CI. It installs the kind binary and creates the cluster in one pinned step, so no manual curl or checksum handling.
- Pin all three coordinates: the action tag (`v1.14.0`), the kind binary version (`v0.32.0`), and the node image tag (kindest/node). Pinning only the action is not enough to reproduce a cluster.
- Put provisioning in one composite action under `.github/actions/`. Every workflow (PR, release, upgrade) reuses it, so there is a single place to bump versions.
- Separate cluster topology from provisioning. A generator script (`generate-kind-config.sh`) keeps node counts and feature gates out of YAML that the action consumes.
- Drive the supported-k8s window with a release-time matrix, not the PR flow. PR e2e can use one default version for speed; release e2e sweeps the full range.

## What NOT to copy

- Do not rely on the runner having kubectl and helm preinstalled without pinning them. KAI-Scheduler never installs kubectl or helm, so their versions float with the `ubuntu-latest` image and are not reproducible. Karta should add `azure/setup-helm` and `azure/setup-kubectl` (or equivalents) with pinned versions.
- Do not let the local pin and the CI pin drift. The hack scripts pin node image `v1.35.0` while the CI action defaults to `v1.34.0`. Reproducing a CI failure locally can silently run a different k8s version. Source both from one shared value.
- Do not put topology decisions inline in the action YAML. KAI-Scheduler already avoids this with a generator script; a naive copy that inlines nodes would be harder to extend.
