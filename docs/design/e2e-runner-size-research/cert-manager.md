<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# cert-manager: e2e runner specs

## TL;DR

cert-manager does not run its kind-based e2e tests on GitHub Actions. The only
two GitHub Actions workflows in the repo are `govulncheck` and `scorecards`, both
on `ubuntu-latest`. The e2e/kind tests run on an external Prow instance
(`prow.build-infra.jetstack.net`), as the job `pull-cert-manager-make-e2e-v1-XX`.
The Prow pod runs a single-node kind cluster and Ginkgo with 40 parallel
processes. This is neither hosted-small nor hosted-large GitHub runner; it is a
self-hosted Prow pod on a Kubernetes cluster. The concrete cpu/memory request,
node selector, or machineType for that Prow pod is not defined in this repo. It
lives in the external test-infra config, which is not present locally. Absence is
the finding here: the e2e runner spec is external.

The one Cloud Build file present is release-only, not e2e. It requests
`machineType: E2_HIGHCPU_32` (32 vCPU).

## How it works

There are only two GitHub Actions workflows, and neither is e2e:

- `.github/workflows/govulncheck.yaml` -> `runs-on: ubuntu-latest`
- `.github/workflows/scorecards.yml` -> `runs-on: ubuntu-latest`

The e2e entry point is the make target `e2e-ci` in `make/test.mk`. It sets up a
kind cluster (`e2e-setup-kind`), installs cert-manager, then runs
`make/e2e-ci.sh`. That script states directly that Prow is the CI runner:

  "Note: We set CI here, even though it should be set by Prow, which is the
  cert-manager CI test runner"

The kind cluster is created by `make/cluster.sh` using
`make/config/kind/cluster.yaml`. That config defines a single control-plane node
(`nodes: - role: control-plane`) with `unsafe-no-fsync: "true"` on etcd to speed
up the tests. There is no worker node and no per-container resource block in the
kind config.

The Ginkgo parallelism is set in `make/e2e.sh` with `nodes=40`. A tuning table in
the same file shows runs at 5, 10, 20, 30, 40, and 50 Ginkgo nodes, all executed
as single Prow jobs, with links to `prow.build-infra.jetstack.net`. The table
notes that at 50 nodes "the pod gets killed somehow", which confirms the whole
e2e run executes inside one Prow pod with a finite memory budget. The exact pod
resource request is set in the external Prow job config, not in this repo.

The Prow job name pattern seen in the links is
`pull-cert-manager-make-e2e-v1-23`. The supported Kubernetes versions come from
`make/kind_images.sh` (1.33 through 1.36).

The only Cloud Build config, `gcb/build_cert_manager.yaml`, is triggered on tag
push to build and sign a release. It is not e2e. It sets
`options.machineType: E2_HIGHCPU_32` and a 45m timeout.

## Relevance to Karta

Karta is deciding what size CI runner to request for its own kind-based e2e job.
cert-manager is a large, mature CNCF project with a heavy e2e suite, so it is a
useful upper-bound reference. The takeaway: cert-manager outgrew hosted GitHub
runners for e2e and moved that workload to a dedicated Prow pod on its own
Kubernetes cluster, where it runs one kind cluster and 40 parallel Ginkgo
processes. That is a much heavier footprint than a default `ubuntu-latest`
(2 vCPU / 7 GB) runner can comfortably serve.

## Evidence (path - what it shows + plain URL)

- `.github/workflows/govulncheck.yaml` line 18 - `runs-on: ubuntu-latest`; not
  e2e (govulncheck only).
  https://github.com/cert-manager/cert-manager/blob/master/.github/workflows/govulncheck.yaml

- `.github/workflows/scorecards.yml` line 16 - `runs-on: ubuntu-latest`; not e2e
  (OSSF scorecard only). These are the only two GitHub Actions workflows.
  https://github.com/cert-manager/cert-manager/blob/master/.github/workflows/scorecards.yml

- `make/e2e-ci.sh` lines 21-27 - states "Prow ... is the cert-manager CI test
  runner"; invokes `make e2e FLAKE_ATTEMPTS=2 CI=true K8S_VERSION=...`.
  https://github.com/cert-manager/cert-manager/blob/master/make/e2e-ci.sh

- `make/test.mk` lines 144-148 - the `e2e-ci` target: runs `e2e-setup-kind
  e2e-setup` then `make/e2e-ci.sh`.
  https://github.com/cert-manager/cert-manager/blob/master/make/test.mk

- `make/config/kind/cluster.yaml` - single control-plane kind node,
  `unsafe-no-fsync: "true"` on etcd, no worker nodes, no resource requests.
  https://github.com/cert-manager/cert-manager/blob/master/make/config/kind/cluster.yaml

- `make/e2e.sh` lines 45-73 - Ginkgo tuning table (5..50 nodes) with links to
  `prow.build-infra.jetstack.net`; `nodes=40` default; note "at 50 nodes the pod
  gets killed somehow" confirms a single Prow pod runs the whole suite.
  https://github.com/cert-manager/cert-manager/blob/master/make/e2e.sh

- `make/kind_images.sh` lines 17-20 - supported Kubernetes versions 1.33 to 1.36
  for e2e kind images.
  https://github.com/cert-manager/cert-manager/blob/master/make/kind_images.sh

- `gcb/build_cert_manager.yaml` lines 9, 36-39 - release-only Cloud Build; 45m
  timeout; `options.machineType: E2_HIGHCPU_32` (32 vCPU). Not e2e.
  https://github.com/cert-manager/cert-manager/blob/master/gcb/build_cert_manager.yaml

- No ProwJob YAML, no testgrid config, and no `.prow.yaml` exist anywhere in the
  repo (searched for `*prow*`, `.prow.yaml`, `testgrid`, `*cloudbuild*`,
  `test-infra`). The Prow job config that sets the pod cpu/memory request lives
  in the external cert-manager test-infra repo, which is not present locally.

## Lessons for Karta

- Hosted GitHub runners have a ceiling. A heavy kind e2e suite with high Ginkgo
  parallelism can exhaust the default 2-vCPU / 7-GB `ubuntu-latest` runner.
  cert-manager runs its e2e as a single pod on its own cluster instead.

- Keep the e2e orchestration in-repo as a plain make target and shell script
  (`make e2e-ci` -> `make/e2e-ci.sh`), and keep the runner sizing out-of-repo.
  This lets the same command run locally and in CI, and lets the runner size be
  tuned without code changes.

- Speed tricks matter more than raw runner size at small scale: single-node kind
  with `unsafe-no-fsync: "true"` on etcd, and tuning Ginkgo parallelism to the
  pod's memory budget. cert-manager settled on a mid value (their comment says 10
  is a "good number", though the default is 40) because higher parallelism hit
  the pod memory limit.

- Track supported Kubernetes versions in one file (`make/kind_images.sh` with
  pinned kind node digests) so the e2e matrix is explicit and reproducible.

## What NOT to copy

- Do not adopt Prow just to size an e2e runner. Prow requires a dedicated
  Kubernetes cluster and the external test-infra tooling. For a young project
  like Karta, a GitHub Actions larger runner (for example a 4-core hosted
  runner) is far simpler than standing up Prow.

- Do not treat cert-manager's release Cloud Build machine (`E2_HIGHCPU_32`) as an
  e2e sizing signal. That 32-vCPU machine is for building and signing a full
  release, not for running e2e.

- Do not expect to find the actual e2e cpu/memory request in the product repo.
  cert-manager keeps it in a separate test-infra repo, so copying "their e2e
  runner spec" is not possible from this repo alone.

- Do not blindly copy `nodes=40` Ginkgo parallelism. That value is tuned to
  cert-manager's specific Prow pod memory budget; on a different runner it can get
  the process OOM-killed, exactly as their own comment about 50 nodes warns.
