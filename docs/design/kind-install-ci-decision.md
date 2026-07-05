<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Installing kind in CI: what the panel does, and what Karta should do

## Purpose and method

Question: is there a more robust or standard way to install the kind binary (and
kubectl, helm) in our GitHub Actions e2e workflow than the current
`go install sigs.k8s.io/kind@v0.30.0` plus curl-kubectl plus get-helm-3?

Constraint that shapes the answer: our `hack/e2e/up.sh` runs `kind create cluster`
itself, so the workflow needs only the kind BINARY, not cluster creation.

Method: a local-only panel of six cloned repos read under `~/workspace/`. Full
per-repo notes are in `./kind-install-ci-research/`.

## Comparison matrix

| Repo | kind binary install | version pin | who creates the cluster | kubectl / helm |
|---|---|---|---|---|
| cert-manager | curl prebuilt release binary + SHA256 verify | `tools += kind=v0.32.0` + renovate + SHA | a script (`make/cluster.sh`), not an action | both curl + SHA-verified |
| kueue | `go install sigs.k8s.io/kind` into `bin/` | tools go.mod (v0.32.0) | a script (`hack/testing/e2e-common.sh`) | helm via go install; kubectl ambient |
| KAI-Scheduler | `helm/kind-action@v1.14.0` | action tag + kind v0.32.0 + node image | the action creates it | not pinned (runner preinstalls) |
| grove | curl official release (kind is local-dev only) | `KIND_VERSION` make var | a script / python (CI uses k3d, not kind) | helm get-helm-3, unpinned |
| cluster-api | curl prebuilt (dev) or import kind as a Go library (e2e) | `MINIMUM_KIND_VERSION` / `test/go.mod`; node by SHA | in-process from the test binary | kubectl ambient; no helm CLI |
| crossplane | Nix (`pkgs.kind`) | nixpkgs `flake.lock` | in-process (`e2e-framework/support/kind`) | Nix |

Two cross-cutting facts. First, most of these run their real e2e on Prow or
Cloud Build, not GitHub Actions (cert-manager, kueue, cluster-api have no e2e
GHA at all), so GHA-specific kind recipes are thinner than expected. Second,
every project pins the kind version, and the strongest ones also pin the node
image by SHA digest.

## Options, ranked

1. Curl the pinned prebuilt kind release binary. What cert-manager
   (`make/_shared/tools/00_mod.mk`), grove (`hack/tools.mk`), and cluster-api's
   dev path (`hack/ensure-kind.sh`) do. No source compile, no Go needed for the
   kind step, fully reproducible from a version pin, and it leaves cluster
   creation to our script. Best fit.
2. `go install sigs.k8s.io/kind@VERSION` (current, kueue's pattern). Works and is
   already pinned, but it compiles kind from source on every run (slower) and
   needs Go on the runner. No real benefit over option 1 for us.
3. `helm/kind-action` (KAI-Scheduler). The one project that uses a Marketplace
   action, and it CREATES a cluster. That is the opposite of what we need: our
   `up.sh` already creates the cluster with our own config and untaint logic.
   Using kind-action would either create a throwaway cluster we ignore or force
   us to move cluster creation out of `up.sh`, breaking the "same path locally
   and in CI" property. Poor fit.
4. Import kind as a Go library and create the cluster in-process (cluster-api,
   crossplane). Elegant when the test binary owns the cluster, but it couples to
   kind internal packages (cluster-api pins kind manually for exactly this
   reason) and does not match our bash `up.sh`. Over-engineered for us.
5. Nix (crossplane). Reproducible but a large toolchain to adopt for one binary.
   Rejected.

## Decision for Karta

Adopt option 1. Replace the `go install` in `.github/workflows/e2e.yaml` with a
curl of the pinned prebuilt kind release binary, and move the version pins into
`hack/e2e/versions.env` so the workflow and local runs read one source of truth
(the pattern cert-manager and kueue both follow with a central version list).
Keep cluster creation in `up.sh` unchanged: the whole panel agrees that keeping
binary-install separate from cluster-creation is what lets the same command run
locally and in CI.

Concretely:
- Add `KIND_VERSION`, `KUBECTL_VERSION`, `HELM_VERSION` to `versions.env`
  (overridable, next to the existing `KIND_NODE_IMAGE`).
- The install step sources `versions.env`, then curls kind, kubectl, and helm at
  those pinned versions. kubectl tracks the cluster's Kubernetes version
  (`KIND_NODE_IMAGE`), which removes the current unpinned `stable.txt` lookup and
  the unpinned get-helm-3 script.

This is a small, low-risk change. It does not touch `up.sh` or the tests.

## What NOT to do

- Do not use `helm/kind-action`: it creates the cluster, which `up.sh` already
  does; adopting it would break the local/CI single path.
- Do not import kind as a Go library to create the cluster in-process: it couples
  to kind internals and does not fit a bash provisioner.
- Do not adopt Nix or a Prow harness for this: far more than the question needs.
- Do not leave kubectl/helm unpinned (KAI-Scheduler and grove do; it is not
  reproducible). Pin them alongside kind.
- Do not SHA256-verify every binary yet (cert-manager does): worth it later, but
  version pinning is the high-value 80 percent; skip the checksum bookkeeping for
  now to avoid over-engineering.

## Per-repo index

- cert-manager: curl prebuilt + SHA, central tool list, script creates cluster - ./kind-install-ci-research/cert-manager.md
- kueue: go install into bin/, tools go.mod pin, script creates cluster - ./kind-install-ci-research/kueue.md
- KAI-Scheduler: helm/kind-action creates the cluster, pins three coordinates - ./kind-install-ci-research/KAI-Scheduler.md
- grove: curl release for local kind, k3d in CI, single make var pin - ./kind-install-ci-research/grove.md
- cluster-api: curl for dev, kind-as-Go-library for e2e, node pinned by SHA - ./kind-install-ci-research/cluster-api.md
- crossplane: Nix toolchain, in-process cluster via e2e-framework - ./kind-install-ci-research/crossplane.md
