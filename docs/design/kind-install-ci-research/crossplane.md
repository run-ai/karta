<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# crossplane: installing kind in CI

Repo: crossplane/crossplane. Default branch: main.
GitHub base: https://github.com/crossplane/crossplane/blob/main/

## TL;DR

- Crossplane does NOT use `helm/kind-action`, `engineerd/setup-kind`, a curl download, or `go install sigs.k8s.io/kind`.
- On the `main` branch, kind, kubectl, and helm are installed by Nix. The e2e job runs `nix run .#e2e`.
- The kind binary comes from the pinned nixpkgs input (`pkgs.kind`). Version pinning is transitive via `flake.lock`, not a `KIND_VERSION` variable.
- No GitHub Action creates the cluster. The Go e2e test binary creates and destroys the kind cluster itself using `sigs.k8s.io/e2e-framework/support/kind`.
- The classic Crossplane `build/` submodule is gone. `.gitmodules` is empty. There is no root `Makefile`. Older release branches used Earthly instead (see Evidence).

## How it works

CI flow for e2e (`.github/workflows/ci.yml`, job `e2e-tests`):

1. Checkout.
2. Install Nix via `cachix/install-nix-action`.
3. Setup Cachix binary cache.
4. Run `nix run .#e2e -- <flags>` wrapped in `nick-fields/retry` (45 min, 3 attempts).

There is no separate "create cluster" step in the workflow. The workflow only makes the `nix` toolchain available. Everything else happens inside the Nix app and the Go test binary.

The `.#e2e` Nix app (`nix/apps.nix`) is a `writeShellApplication` with `runtimeInputs` that include `pkgs.kind` and `pkgs.kubernetes-helm`. It sets `inheritPath = false`, so only the Nix-provided kind and helm are on PATH. It loads the Crossplane image into Docker, tags it, then runs the precompiled e2e binary through `gotestsum`.

The e2e binary is built separately (`nix/build.nix`, target `e2e`) via `go test -c -o e2e ./test/e2e`.

Cluster lifecycle is owned by the Go test `TestMain` (`test/e2e/main_test.go`):

- If running against kind, setup appends `envfuncs.CreateClusterWithConfig(kind.NewProvider(), <name>, "./test/e2e/manifests/kind/kind-config.yaml")`.
- Teardown appends `envfuncs.ExportClusterLogs(...)` (optional) and `envfuncs.DestroyCluster(...)` (only if the test created the cluster).
- The import is `sigs.k8s.io/e2e-framework/support/kind`. The framework shells out to the `kind` binary found on PATH, which is the Nix-pinned one.

Helm is used the same way: tests call `sigs.k8s.io/e2e-framework/third_party/helm`, which shells out to the `helm` binary on PATH. Chart source is `cluster/charts/crossplane` (`test/e2e/main_test.go`).

Kind cluster config (`test/e2e/manifests/kind/kind-config.yaml`) sets a single control-plane node with API server audit logging. It does NOT set a node `image:`, so the Kubernetes version follows the default baked into the pinned kind binary.

## Relevance to Karta

Karta also ships a Helm chart and needs e2e against a real cluster. The transferable idea is decoupling: the CI workflow only installs a toolchain, and the test binary owns cluster create and destroy. This keeps the same e2e path reproducible locally and in CI. Karta uses a Go / controller-runtime / Helm stack, so the `sigs.k8s.io/e2e-framework` kind + helm pattern maps directly, without needing to adopt Nix.

## Evidence

Every claim below maps to a path read in the local repo.

- E2E job installs only Nix, then runs `nix run .#e2e`. No kind-action, no setup-kind, no curl, no go install kind.
  `.github/workflows/ci.yml` lines 116-205.
  https://github.com/crossplane/crossplane/blob/main/.github/workflows/ci.yml

- The `.#e2e` app injects kind and helm via Nix runtimeInputs, with `inheritPath = false`.
  `nix/apps.nix` lines 167-205 (`pkgs.kind`, `pkgs.kubernetes-helm`).
  https://github.com/crossplane/crossplane/blob/main/nix/apps.nix

- kind, kubectl, helm all sourced from nixpkgs.
  `flake.nix` lines 205-207 (`pkgs.kubectl`, `pkgs.kubernetes-helm`, `pkgs.kind`).
  https://github.com/crossplane/crossplane/blob/main/flake.nix

- Version pinning is transitive through the nixpkgs input, locked in `flake.lock`.
  `flake.nix` line 8 (`nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11"`).
  `flake.lock` lines 43-58 (nixpkgs rev `d04d8548aed39902419f14a8537006426dc1e4fa`).
  https://github.com/crossplane/crossplane/blob/main/flake.lock

- The Go test creates and destroys the kind cluster; no Action does it.
  `test/e2e/main_test.go` lines 95-141 (`CreateClusterWithConfig`, `DestroyCluster`).
  Import `sigs.k8s.io/e2e-framework/support/kind` at line 33.
  https://github.com/crossplane/crossplane/blob/main/test/e2e/main_test.go

- kind cluster config sets no node image, so k8s version follows the kind binary default.
  `test/e2e/manifests/kind/kind-config.yaml` (no `image:` key).
  https://github.com/crossplane/crossplane/blob/main/test/e2e/manifests/kind/kind-config.yaml

- Cluster name and kind-vs-external logic live in the test config, not CI.
  `test/e2e/config/environment.go` lines 109-187 (`kind-cluster-name` flag, `IsKindCluster`, `GetKindClusterName`).
  https://github.com/crossplane/crossplane/blob/main/test/e2e/config/environment.go

- Legacy Earthly path used an explicit `ARG KIND_VERSION`. Only on old release branches, not main.
  `.github/renovate-earthly.json5` lines 1-3 and 67-90 (comment: "Main branch uses Nix"; renovate rule for `ARG KIND_VERSION` in `Earthfile`).
  `packageRules` lines 132-135 scope Earthly to `release-1.*` and `release-2.[0-1]`.
  https://github.com/crossplane/crossplane/blob/main/.github/renovate-earthly.json5

- Absence findings: no root `Makefile`, no `build/` directory, empty `.gitmodules`, no `Earthfile` on the main branch. Confirmed by directory listing at repo root and `find`.

## Lessons for Karta

- Let the test binary own the cluster. Putting `CreateCluster` / `DestroyCluster` in Go (via `sigs.k8s.io/e2e-framework`) makes the exact same e2e run locally and in CI. CI only needs a toolchain.
- Pin the toolchain in one place. Crossplane pins kind, kubectl, and helm through a single locked dependency graph (`flake.lock`), so all three move together and are reproducible.
- Keep kind cluster config in-repo as a versioned file (`kind-config.yaml`) rather than encoding node flags in workflow YAML.
- Guard teardown: only destroy the cluster if the test created it (`ShouldDestroyKindCluster`). This protects a developer's shared or external cluster.
- Use `e2e-framework`'s `helm` and `kind` third-party wrappers instead of hand-rolled shell, since they already shell out to pinned binaries.

## What NOT to copy

- Do not adopt Nix just for tool pinning unless the team wants it. Nix is Crossplane's whole build system here (checks, images, chart, e2e all run through `nix`). For Karta a lighter approach fits AGENTS.md better: pin kind, kubectl, and helm as Makefile variables and install prebuilt release binaries, or use `helm/kind-action` pinned by SHA. The lesson to copy is the decoupling, not the Nix machinery.
- Do not follow the Earthly path in `renovate-earthly.json5`. It is legacy, scoped to old release branches, and the referenced `Earthfile` does not exist on main.
- Do not leave the kind node image unpinned if reproducibility across kind upgrades matters. Crossplane relies on the kind binary default, which shifts when the pinned kind version changes. Karta may prefer an explicit `image: kindest/node:vX.Y.Z` in its kind config.
