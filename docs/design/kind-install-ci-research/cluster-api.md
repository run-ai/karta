<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# cluster-api: installing kind in CI

Repo: kubernetes-sigs/cluster-api. Default branch: main.
Links below point to files that exist locally at ~/workspace/cluster-api.

## TL;DR

Cluster API does not use `helm/kind-action`, `engineerd/setup-kind`, or
`go install sigs.k8s.io/kind`. It installs kind two different ways for two
different purposes.

1. A shell script downloads a prebuilt kind release binary with curl and pins
   the version via a single `MINIMUM_KIND_VERSION` variable. This is for local
   development and the Tilt workflow.
2. The e2e test framework imports kind as a Go library (`sigs.k8s.io/kind`,
   pinned in `test/go.mod`) and creates the management cluster in-process by
   calling `kind.NewProvider().Create(...)`. No kind binary is invoked.

Key finding: none of the GitHub Actions workflows in `.github/workflows/`
reference kind, kubectl, or helm. E2E runs on the Kubernetes project CI
(Prow / Google Cloud Build), not GitHub Actions. So there is no CI Action that
"sets up kind" at all.

## How it works

### kind binary via curl (dev / Tilt)

`hack/ensure-kind.sh` checks if `kind` is on PATH. If not, and the OS is linux
or darwin, it curls the release binary into `$(go env GOPATH)/bin`:

    curl --retry 5 --retry-all-errors -sLo "${GOPATH_BIN}/kind" \
      "https://github.com/kubernetes-sigs/kind/releases/download/${MINIMUM_KIND_VERSION}/kind-${goos}-${goarch}"

Version pin: `MINIMUM_KIND_VERSION=v0.32.0` at the top of that script. The
script also verifies an already-installed kind meets that minimum by sorting
version strings. No SHA checksum is verified on the downloaded binary.

### kind as a Go library (e2e)

The e2e bootstrap provider does not shell out to kind. It imports the kind
packages and creates the cluster programmatically:

- `test/framework/bootstrap/kind_provider.go` imports `sigs.k8s.io/kind/pkg/cluster`
  and calls `provider.Create(k.name, kindCreateOptions...)` inside
  `createKindCluster()`.
- Dispose calls `kind.NewProvider().Delete(...)`.
- The library version is pinned in `test/go.mod` (`sigs.k8s.io/kind v0.32.0`)
  and locked in `test/go.sum`.

### Cluster creation and local registry (dev)

`hack/kind-install.sh` is the shared cluster-creation logic for dev. It runs
`kind create cluster`, stands up a `registry:2` container, wires the containerd
registry config, and applies a `local-registry-hosting` ConfigMap with kubectl.
`hack/kind-install-for-capd.sh` and `hack/kind-install-for-capk.sh` wrap it with
CAPD/KubeVirt-specific kind `Cluster` config (docker socket mount, IP family).
`make kind-cluster` calls the CAPD wrapper; `make tilt-up` depends on it.

### kubectl and helm

- kubectl: not installed or pinned by the repo. The `Tiltfile` requires kubectl
  to already exist on PATH and fails fast if missing. Scripts assume kubectl is
  present. No version pin found.
- helm: no helm binary is downloaded or installed anywhere in `hack/`,
  `Makefile`, or `scripts/`. "Helm" appears only as kustomize `helmCharts`
  chart inflation under `hack/observability/*/kustomization.yaml`, which is a
  build-time concern, not a CI cluster tool.

### kindest/node version pinning

The Kubernetes version that runs inside kind is pinned separately by full image
SHA. `test/framework/bootstrap/kind_provider.go` sets
`DefaultNodeImageVersion = "v1.36.1@sha256:3489c7..."`. CAPD keeps a
kind-version-to-behavior mapping in `test/infrastructure/kind/mapper.go`;
`hack/ensure-kind.sh` warns that new SHAs must be added to `preBuiltMappings`
there when bumping the kind version.

## Relevance to Karta

Karta is a controller-runtime + Helm project that needs kind in e2e CI. Cluster
API is a strong reference for the in-process library approach because it, like
Karta, is a controller project that uses kind as a management cluster. The
library approach removes the "install a binary in CI" step entirely for the
test path. The curl-with-pinned-version script is the simpler pattern if Karta
prefers to keep using the kind CLI.

## Evidence

- hack/ensure-kind.sh: curl download, `MINIMUM_KIND_VERSION=v0.32.0`, version verify.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/hack/ensure-kind.sh
- hack/kind-install.sh: `kind create cluster` plus local registry wiring.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/hack/kind-install.sh
- hack/kind-install-for-capd.sh: CAPD kind Cluster config wrapper.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/hack/kind-install-for-capd.sh
- test/framework/bootstrap/kind_provider.go: `kind.NewProvider().Create(...)`, node image SHA.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/test/framework/bootstrap/kind_provider.go
- test/go.mod: `sigs.k8s.io/kind v0.32.0` library pin.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/test/go.mod
- test/infrastructure/kind/mapper.go: kind-version-to-behavior mapping.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/test/infrastructure/kind/mapper.go
- Tiltfile: requires kubectl on PATH, no install/pin.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/Tiltfile
- Makefile: `kind-cluster`, `clean-kind` targets wrap the hack scripts.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/Makefile
- .github/dependabot.yaml: kind is upgraded manually, dependabot ignores minor/major.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/.github/dependabot.yaml
- cloudbuild.yaml: release builds run on Google Cloud Build, not GitHub Actions.
  https://github.com/kubernetes-sigs/cluster-api/blob/main/cloudbuild.yaml
- .github/workflows/: no kind, kubectl, or helm references (verified by search).

## Lessons for Karta

- A controller project can create its kind management cluster in-process by
  importing `sigs.k8s.io/kind/pkg/cluster` and calling `provider.Create`. This
  makes the kind version a normal go.mod dependency, so `go.sum` locks it and
  Go module tooling controls the pin. No separate binary install step.
- Pin the Kubernetes node image by full digest (`kindest/node:vX@sha256:...`),
  not just a tag, for reproducible e2e.
- Keep the CLI-install path simple: one `MINIMUM_KIND_VERSION` variable plus a
  retrying curl into a bin dir, with a version-comparison guard.
- Pin kind deliberately, not automatically. dependabot is told to ignore kind
  version bumps because a bump must be coordinated with node-image SHA mappings.
- Provide a shared cluster-create script with a local registry, wrapped by
  thin per-scenario scripts, and expose it through Make targets.

## What NOT to copy

- Do not assume Cluster API shows a GitHub Actions kind setup. It does not. Its
  e2e runs on Kubernetes Prow / GCB. If Karta's CI is GitHub Actions, this repo
  gives no GitHub Actions kind-setup pattern to copy directly.
- The curl download in `hack/ensure-kind.sh` verifies no SHA checksum on the
  binary. If Karta downloads the kind binary in CI, add checksum verification.
- Do not treat kubectl or helm as pinned here. kubectl is assumed present with
  no pin, and helm is not installed as a CLI at all (only kustomize helmCharts
  inflation). Karta must decide its own kubectl/helm pinning.
- The in-process library approach ties Karta's test binary to kind internal Go
  packages, which have no stability guarantee across kind releases. That
  coupling is why Cluster API upgrades kind manually and maintains a version
  mapper. Adopt it only if you accept that maintenance cost.
