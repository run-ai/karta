<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# kueue: installing kind in CI

## TL;DR

Kueue installs kind with `go install sigs.k8s.io/kind@$(KIND_VERSION)`.
It does not use `helm/kind-action`, `engineerd/setup-kind`, or a curl of a
prebuilt release binary. The kind version is pinned in a dedicated tools Go
module (`hack/tools/go.mod`), and the Makefile reads it back with `go list -m`.
helm is installed the same way (`go install helm.sh/helm/v4/cmd/helm`), also
pinned in `hack/tools/go.mod`. kubectl is not installed by the project; it comes
from the ambient CI image. No GitHub Action creates the cluster. The project
creates the cluster itself in `hack/testing/e2e-common.sh` via `kind create
cluster`, wired up by Makefile e2e targets. Kueue e2e runs on Kubernetes Prow
plus Google Cloud Build, not GitHub Actions.

## How it works

Tool versions live in `hack/tools/go.mod`. `sigs.k8s.io/kind v0.32.0` and
`helm.sh/helm/v4` are listed as direct requirements, and `hack/tools/pinversion.go`
blank-imports them under a `//go:build tools` tag so `go mod` keeps them.

`Makefile-deps.mk` derives every tool version by querying that module. For kind:

```make
KIND_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' sigs.k8s.io/kind)
```

The install targets then `go install` each tool into a local `bin/` directory:

```make
kind: gomod-download-tools
	@$(NETWORK_INSTALL_RETRY) GOBIN=$(BIN_DIR) GO111MODULE=on $(GO_CMD) install sigs.k8s.io/kind@$(KIND_VERSION)

helm: gomod-download-tools
	@$(NETWORK_INSTALL_RETRY) GOBIN=$(BIN_DIR) GO111MODULE=on $(GO_CMD) install helm.sh/helm/v4/cmd/helm@$(HELM_VERSION)
```

Installs are wrapped in a retry helper (`hack/testing/retry.sh`) to tolerate
transient network failures.

The `setup-e2e-env` target lists the tools it needs as prerequisites, which
triggers the install targets above:

```make
setup-e2e-env: kustomize yq dep-crds kind helm ginkgo ginkgo-top
```

Every e2e target (for example `test-e2e-baseline`) depends on `setup-e2e-env`,
then invokes a per-k8s-version recipe that exports the kind node image and hands
off to the e2e shell scripts.

The Kubernetes version, and therefore the kind node image, is pinned in
`Makefile-test.mk`:

```make
E2E_K8S_VERSIONS ?= 1.34.8 1.35.5 1.36.1
E2E_K8S_VERSION ?= 1.36
E2E_KIND_VERSION ?= kindest/node:v$(E2E_K8S_FULL_VERSION)
```

Cluster creation is done by the project, not an action. `hack/testing/e2e-common.sh`
resolves `$KIND` to `bin/kind` and runs `kind create cluster` with the pinned
node image, a config file, and a retry wrapper:

```sh
export KIND="$ROOT_DIR"/bin/kind
...
$KIND create cluster --name "$cluster" --image "$E2E_KIND_VERSION" --config "$kind_config" --kubeconfig=... --wait 5m -v 5
```

The same script can build a custom node image with `kind build node-image` when
needed. After creation it uses `kubectl` (from the environment) to select the
context and wait for nodes.

kubectl is not provisioned by the repo. There is no kubectl install target, no
curl of a kubectl binary, and no `go install` of kubectl. `k8s.io/kubectl` in
`hack/tools/go.mod` is only an indirect library dependency, not an installed
CLI. Scripts call a bare `kubectl`, relying on the Prow container image to
supply it.

## Relevance to Karta

Karta is a controller-runtime + Helm project that will need kind-based e2e in
CI. Kueue shows a mature, network-pinned pattern for provisioning kind and helm
without third-party marketplace actions, and a clean split between tool install
(Makefile) and cluster lifecycle (hack scripts). This maps directly onto Karta's
`make`-driven workflow and its existing e2e work on the `e2e-karta` branch.

## Evidence

- Kind installed via `go install`, version from tools module:
  `Makefile-deps.mk` lines 24 (KIND_VERSION), 107-109 (kind target).
  https://github.com/kubernetes-sigs/kueue/blob/main/Makefile-deps.mk
- Helm installed via `go install`, version from tools module:
  `Makefile-deps.mk` lines 26 (HELM_VERSION), 115-117 (helm target).
- Tool versions pinned in a dedicated tools module:
  `hack/tools/go.mod` line 44 (`sigs.k8s.io/kind v0.32.0`), plus helm.sh/helm/v4.
  https://github.com/kubernetes-sigs/kueue/blob/main/hack/tools/go.mod
- Tools kept by blank import under a build tag:
  `hack/tools/pinversion.go` line 37 (`_ "sigs.k8s.io/kind"`), line 32 (helm).
  https://github.com/kubernetes-sigs/kueue/blob/main/hack/tools/pinversion.go
- Project creates the cluster itself:
  `hack/testing/e2e-common.sh` line 19 (`export KIND=...bin/kind`),
  line 552 (`$KIND create cluster ...`), line 62 (`kind build node-image`).
  https://github.com/kubernetes-sigs/kueue/blob/main/hack/testing/e2e-common.sh
- Kubernetes / kind node image versions pinned:
  `Makefile-test.mk` lines 53-58 (E2E_K8S_VERSIONS, E2E_KIND_VERSION).
  https://github.com/kubernetes-sigs/kueue/blob/main/Makefile-test.mk
- e2e env wired through Make prerequisites:
  `Makefile-test.mk` lines 768-769 (`setup-e2e-env: ... kind helm ...`).
- No GitHub Actions e2e workflow. `.github/workflows/` holds only
  `krew-release.yml`, `openvex.yaml`, `sbom.yaml`, `sync-dependabot.yaml`.
  https://github.com/kubernetes-sigs/kueue/tree/main/.github/workflows
- e2e runs under Prow / Cloud Build: `cloudbuild-periodic.yaml` (top of file).
  https://github.com/kubernetes-sigs/kueue/blob/main/cloudbuild-periodic.yaml
- kubectl is not installed by the repo. No install target in `Makefile-deps.mk`;
  `k8s.io/kubectl` appears only as an indirect dep in `hack/tools/go.mod` line 522.

## Lessons for Karta

- Pin kind and helm in one place. A tools Go module plus `go list -m` gives a
  single source of truth and lets Dependabot bump the CLI versions like any
  other dependency. Karta already keeps tool versions in Go modules, so this
  extends naturally.
- Install into a repo-local `bin/` and reference tools by absolute path in
  scripts. This makes local `make test-e2e` and CI behave identically.
- Keep cluster lifecycle in your own scripts (`kind create cluster`) rather than
  delegating to a marketplace action. You control config, retries, node-image
  builds, and teardown. Kueue wraps create in a retry that only re-tries known
  transient errors, so real failures still fail fast.
- Pin the Kubernetes node image (kindest/node:vX.Y.Z), and support a matrix of
  k8s versions if you need multi-version coverage.
- Wrap network installs in a retry helper to reduce CI flakes.

## What NOT to copy

- Do not assume `go install sigs.k8s.io/kind` is the only option. It requires a
  Go toolchain in CI and compiles kind from source, which is slower than a
  prebuilt release download. For Karta's GitHub Actions runners, a pinned curl of
  a kind release binary, or `helm/kind-action` pinned by SHA, may be simpler.
  Choose based on whether Go is already on the runner.
- Do not copy the Prow / Cloud Build setup. Kueue runs e2e on Kubernetes
  test-infra Prow, not GitHub Actions. Karta uses GitHub Actions, so the CI
  harness differs even if the Makefile tool-install pattern transfers.
- Do not rely on kubectl being ambient. Kueue leans on the Prow image to provide
  kubectl. On a GitHub Actions runner you should install kubectl explicitly
  (pinned), for example with `azure/setup-kubectl` or a pinned release download.
- Kueue pins helm to v4. Confirm Karta's chart tooling targets the same major
  before copying the version.
