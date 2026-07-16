<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# kueue: e2e runner specs

## TL;DR

kueue (kubernetes-sigs/kueue) does NOT run its kind-based e2e suite on GitHub
Actions. The four workflows under `.github/workflows/` are all housekeeping
(krew release, openvex, sbom, dependabot sync) and use hosted-small
`ubuntu-latest` / `ubuntu-24.04`. The real e2e/kind/integration jobs run on
Prow (prow.k8s.io) via the out-of-repo kubernetes/test-infra config at
`config/jobs/kubernetes-sigs/kueue/`, referenced by name from
`hack/tools/prow-runtimes/prow_runtimes.py`. The concrete CI runner resource
requests (cpu/memory, prow build cluster) are therefore NOT in this repo; that
is a finding, not an omission. The only in-repo compute sizing signal is
Cloud Build for image pushing, which requests machineType `E2_HIGHCPU_32`
(32 vCPU), and the Makefile e2e parallelism knobs (E2E_NPROCS up to 5,
INTEGRATION_NPROCS 4) plus kind cluster shapes (2 workers baseline, 8 workers
for topology-aware scheduling). Classification: self-hosted / Prow for e2e;
the in-repo GitHub Actions are hosted-small.

## How it works

CI split by surface:

- Housekeeping automation runs on GitHub Actions hosted runners. All four
  workflows in `.github/workflows/` set `runs-on: ubuntu-latest` or
  `ubuntu-24.04`. None of them build a kind cluster or run e2e.
- Image publishing runs on Google Cloud Build. `cloudbuild.yaml` (postsubmit)
  and `cloudbuild-periodic.yaml` (periodic) both build with
  `options.machineType: 'E2_HIGHCPU_32'`. These only push images; they do not
  run e2e tests.
- e2e / kind / integration run on Prow. The tooling in the repo assumes Prow:
  `hack/testing/get-build-logs.sh` transforms `https://prow.k8s.io/view/gs/`
  URLs and points at the `kubernetes-sigs_kueue` GCS bucket, and
  `hack/tools/prow-runtimes/prow_runtimes.py` fetches the Prow job definitions
  from kubernetes/test-infra:
  `config/jobs/kubernetes-sigs/kueue/kueue-periodics-main.yaml` and
  `kueue-presubmits-main.yaml`. Job names follow the Prow convention
  (`pull-kueue-*` presubmits, `periodic-kueue-*` periodics). The pod-level
  resource requests (cpu/memory) and the prow build cluster live in those
  test-infra files, which are outside this repo and were not read (no-network
  constraint), so exact per-job cpu/memory cannot be quoted from local
  evidence.

What the repo does control is the shape and parallelism of the workload the
runner must handle:

- kind topology. `hack/testing/kind-cluster.yaml` = 1 control-plane + 2 workers
  (labeled on-demand / spot). `hack/testing/kind-cluster-tas.yaml` = 1
  control-plane + 8 workers (topology blocks/racks for topology-aware
  scheduling). MultiKueue spins up multiple kind clusters
  (`hack/testing/multikueue/manager-cluster.kind.yaml` and
  `worker-cluster.kind.yaml`).
- ginkgo parallelism. `Makefile-test.mk` sets `E2E_NPROCS ?= 1` by default,
  but individual targets raise it: baseline and extended e2e use
  `E2E_NPROCS := 4`, MultiKueue e2e uses `E2E_NPROCS := 5`, and one kuberay
  extended shard drops to `E2E_NPROCS := 2`. Integration tests use
  `INTEGRATION_NPROCS ?= 4` (MultiKueue integration `?= 3`).
- sharding. e2e and unit/integration suites shard across parallel CI jobs
  (`INTEGRATION_TOTAL_SHARDS`, `UNIT_TOTAL_SHARDS`, and the many
  `*-shard-0/1/2` Make targets), which is how kueue keeps each individual Prow
  pod within its runtime/resource budget rather than sizing one giant runner.

## Relevance to Karta

Karta is also a kubernetes-sigs-style project (Apache 2.0, controller-runtime,
Helm) and its e2e branch (`e2e-karta`) is standing up kind-based e2e in GitHub
Actions. kueue is the closest large-scale reference for a Kubernetes workload
controller, so its choices matter, but the direct comparison is limited: kueue
offloads e2e to Prow, which Karta (a standalone GitHub repo, not yet in
test-infra) cannot use. The transferable parts are the workload-sizing signals:
parallelism knobs, sharding, and kind cluster shapes, not a copyable
`runs-on:` label.

## Evidence

- `/Users/ahayumi/workspace/kueue/.github/workflows/krew-release.yml:7`
  `runs-on: ubuntu-24.04` (housekeeping, not e2e).
- `/Users/ahayumi/workspace/kueue/.github/workflows/sync-dependabot.yaml:11`
  `runs-on: ubuntu-latest`.
- `/Users/ahayumi/workspace/kueue/.github/workflows/openvex.yaml:14`
  `runs-on: ubuntu-latest`.
- `/Users/ahayumi/workspace/kueue/.github/workflows/sbom.yaml:12`
  `runs-on: ubuntu-latest`.
- Directory listing of `/Users/ahayumi/workspace/kueue/.github/workflows/`
  contains only: OWNERS, krew-release.yml, openvex.yaml, sbom.yaml,
  sync-dependabot.yaml. No e2e/kind/integration workflow exists here
  (absence is a finding).
- `/Users/ahayumi/workspace/kueue/cloudbuild.yaml:23`
  `machineType: 'E2_HIGHCPU_32'` (image-pushing-postsubmit, not e2e).
- `/Users/ahayumi/workspace/kueue/cloudbuild-periodic.yaml:23`
  `machineType: 'E2_HIGHCPU_32'` (image-pushing-periodic, not e2e).
- `/Users/ahayumi/workspace/kueue/hack/testing/get-build-logs.sh:31`
  `local prefix="https://prow.k8s.io/view/gs/"` and line 51 checks for
  `kubernetes-sigs_kueue` bucket, confirming Prow-hosted CI.
- `/Users/ahayumi/workspace/kueue/hack/tools/prow-runtimes/prow_runtimes.py:36-43`
  points at test-infra `config/jobs/kubernetes-sigs/kueue/`
  `kueue-periodics-main.yaml` and `kueue-presubmits-main.yaml` (the actual e2e
  job/resource definitions, out of repo).
- `/Users/ahayumi/workspace/kueue/hack/tools/prow-runtimes/prow_runtimes.py:266-269`
  presubmit jobs named `pull-kueue*`; periodics `periodic-kueue*`.
- `/Users/ahayumi/workspace/kueue/Makefile-test.mk:73`
  `E2E_NPROCS ?= 1`; lines 170, 189, 193, 199 set `E2E_NPROCS := 5` for
  MultiKueue; lines 246, 250, 255, 270 set `E2E_NPROCS := 4`; line 260 sets
  `E2E_NPROCS := 2`.
- `/Users/ahayumi/workspace/kueue/Makefile-test.mk:35-36`
  `INTEGRATION_NPROCS ?= 4`, `INTEGRATION_NPROCS_MULTIKUEUE ?= 3`.
- `/Users/ahayumi/workspace/kueue/Makefile-test.mk:38-45` integration sharding
  via `INTEGRATION_TOTAL_SHARDS` / `hack/testing/shard-integration-tests.sh`;
  lines 84-88 unit sharding via `UNIT_TOTAL_SHARDS`.
- `/Users/ahayumi/workspace/kueue/hack/testing/kind-cluster.yaml:3-54`
  1 control-plane + 2 workers.
- `/Users/ahayumi/workspace/kueue/hack/testing/kind-cluster-tas.yaml:4-89`
  1 control-plane + 8 workers (topology blocks/racks).
- No `resources:` / `requests:` / `cpu:` / `memory:` cpu-memory sizing for CI
  runners exists anywhere in the repo (the matches under `test/` and `site/`
  are workload/pod manifests, not runner specs). Absence is a finding.

GitHub links (remote github.com/kubernetes-sigs/kueue, default branch main;
paths verified to exist locally):

- https://github.com/kubernetes-sigs/kueue/blob/main/.github/workflows/krew-release.yml
- https://github.com/kubernetes-sigs/kueue/blob/main/.github/workflows/sbom.yaml
- https://github.com/kubernetes-sigs/kueue/blob/main/cloudbuild.yaml
- https://github.com/kubernetes-sigs/kueue/blob/main/cloudbuild-periodic.yaml
- https://github.com/kubernetes-sigs/kueue/blob/main/hack/testing/get-build-logs.sh
- https://github.com/kubernetes-sigs/kueue/blob/main/hack/tools/prow-runtimes/prow_runtimes.py
- https://github.com/kubernetes-sigs/kueue/blob/main/Makefile-test.mk
- https://github.com/kubernetes-sigs/kueue/blob/main/hack/testing/kind-cluster.yaml
- https://github.com/kubernetes-sigs/kueue/blob/main/hack/testing/kind-cluster-tas.yaml

## Lessons for Karta

- Separate concerns by surface: kueue keeps GitHub Actions for cheap
  housekeeping and pushes the heavy kind e2e to a beefier build system. Karta
  can mirror this idea even without Prow by giving e2e its own workflow/job
  with a larger runner rather than overloading the default hosted-small
  runner.
- Make parallelism an env knob (E2E_NPROCS / INTEGRATION_NPROCS) with a safe
  default of 1 and per-target overrides. This lets the same Makefile run
  laptop-serial and CI-parallel, and it makes runner sizing a tuning dial
  rather than a rewrite.
- Shard when a single job gets too big. kueue splits e2e/integration/unit into
  shard-0/1/2 targets so each CI pod stays small. For Karta this is cheaper and
  more scalable than requesting one very large runner.
- Keep the kind cluster shape in a checked-in file (kind-cluster.yaml) so the
  worker count that drives runner memory is explicit and reviewable. A 2-worker
  baseline is a reasonable, modest footprint.
- Build a runtime-tracking habit early. kueue's prow_runtimes.py measures job
  durations to set thresholds and find optimization targets; an equivalent
  lightweight timing check helps right-size runners over time.

## What NOT to copy

- Do not assume a `runs-on:` label for e2e exists to copy. kueue has none in
  repo; its e2e runner sizing lives in kubernetes/test-infra, which Karta
  cannot use unless/until it onboards to Prow.
- Do not copy `E2_HIGHCPU_32`. That is a Google Cloud Build machineType for
  image building, unrelated to GitHub Actions e2e runners, and 32 vCPU is
  overkill for Karta's current needs.
- Do not adopt the multi-cluster MultiKueue kind setup or the 8-worker TAS
  cluster as a default. Those exist for kueue-specific features
  (cross-cluster dispatch, topology-aware scheduling) and would inflate runner
  memory for no Karta benefit.
- Do not depend on the k8s CI GCS log bucket / prow.k8s.io tooling
  (get-build-logs.sh, prow_runtimes.py). Those are bound to kubernetes-sigs
  Prow infrastructure and will not work for a standalone GitHub repo.
