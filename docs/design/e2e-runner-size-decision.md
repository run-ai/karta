<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# E2E runner size: what the panel does, and what Karta should do

## Purpose and method

Question: should Karta's e2e move off `ubuntu-latest` (2 vCPU / 7 GB), where the
full 13-case suite is 25/26 with RayJob timing out on a slow Ray dashboard boot?
If so, to what runner? Panel of eight local repos, read no-network. Per-repo
notes in ./e2e-runner-size-research/.

## Comparison matrix

| Project | e2e platform | Runner | How it copes with size |
|---|---|---|---|
| cert-manager | Prow (external) | self-hosted Prow pod | one pod, single-node kind, Ginkgo nodes=40 (50 OOMs) |
| kueue | Prow (external) | self-hosted Prow | E2E_NPROCS 1-5, 2-8 worker kind, sharded |
| cluster-api | Prow (external) | self-hosted Prow | GINKGO_NODES=3; sizing lives in test-infra |
| KAI-Scheduler | GitHub Actions | ubuntu-latest (small) | 11-way k8s matrix, parallel runners |
| crossplane | GitHub Actions | ubuntu-24.04 (small) | test-area matrix + retry(3x, 45m), single-node kind |
| volcano | GitHub Actions | ubuntu-24.04 (small) | ~21 parallel e2e jobs, free-disk-space step, 5-node kind |
| grove | GitHub Actions | self-hosted prod-grove-e2e-v1 | pre-baked runner image, k3d 30 nodes, dind-memory-mode |
| keda | GitHub Actions | self-hosted "e2e" pool | pre-baked container, real AKS (not kind) |

Concrete labels and paths:
- KAI-Scheduler: runs-on ubuntu-latest, .github/workflows/on-pr.yaml:236 and
  on-release.yaml:81 (11-entry matrix).
- crossplane: runs-on ubuntu-24.04, .github/workflows/ci.yml:118 (matrix + retry).
- volcano: runs-on ubuntu-24.04, .github/workflows/e2e_shard.yaml:10 (~21 jobs).
- grove: runs-on prod-grove-e2e-v1, .github/workflows/build-check-test.yaml:112;
  pre-baked image named in .github/actions/e2e-setup/action.yaml:29.
- keda: runs-on "e2e", .github/workflows/pr-e2e.yml:206; container
  ghcr.io/kedacore/keda-tools at :208.
- cert-manager: no GHA e2e; Prow, make/e2e-ci.sh:21; nodes=40 make/e2e.sh:73.
- Only in-repo big machineTypes are release image builds, not e2e (cert-manager
  gcb E2_HIGHCPU_32, kueue/cluster-api E2_HIGHCPU_8/32).

## The two findings that matter

1. Nobody runs a big all-in-one suite on one small hosted runner. The projects
   that stay on hosted ubuntu (KAI, crossplane, volcano) SHARD: many parallel
   runners, each doing a subset (volcano ~21, KAI 11). That is how a 2 vCPU box
   is enough. Karta cramming 13 operators plus workloads onto one ubuntu-latest
   is the pattern none of them use, and it is exactly why RayJob starves.
2. When a project does want one heavy single-cluster e2e, it uses a self-hosted
   runner. The closest sibling, grove (same NVIDIA org), uses a pre-baked
   self-hosted label prod-grove-e2e-v1. keda uses a self-hosted "e2e" pool.

## Options, ranked

1. Shard across parallel ubuntu-latest jobs (KAI/volcano/crossplane pattern).
   Karta already has the mechanism: hack/e2e/up.sh takes a WORKLOADS subset and
   CLUSTER_NAME for isolated parallel clusters, and the workflow exposes
   workloads/focus inputs. A matrix of 3-4 shards (for example
   "jobset lws kuberay kubeflow", "knative kserve milvus", "grove dynamo nim")
   each fits a 2 vCPU runner. No new infra, no cost, proven by the panel. Best
   fit for now.
2. Self-hosted runner for the full suite (grove/keda pattern). Request a
   pre-baked self-hosted label like grove's prod-grove-e2e-v1, or a GitHub
   larger runner (~8 vCPU / 32 GB). Karta's workflow already plumbs this via the
   runner input, so it is a label change plus provisioning. Right when you want
   one node holding everything (closest to how a user really runs Karta).
3. Prow / Cloud Build (cert-manager/kueue/cluster-api). External k8s CI at
   test-infra scale. Overkill for Karta; do not adopt.

## Decision for Karta

Do both, in order:
- Now: keep the hosted runner but SHARD the full suite into a small matrix using
  the WORKLOADS/CLUSTER_NAME support that already exists. Each shard is well
  within 2 vCPU, RayJob included, and shards run in parallel so wall-clock stays
  low. This removes the contention that starves the Ray dashboard without asking
  anyone for hardware.
- When a single full-node run is wanted (or GPU-adjacent cases grow): ask for a
  self-hosted runner mirroring grove (prod-grove-e2e-v1), or a GitHub larger
  runner around 8 vCPU / 32 GB, and pass it through the existing runner input.

Either path beats raising timeouts. The RayJob timeout bump already committed is
a stopgap so the current all-in-one run can pass; it is not the structural fix.

## What NOT to do

- Do not keep cramming all 13 operators plus workloads onto one ubuntu-latest.
  No comparable project does this; it is the root of the RayJob starvation.
- Do not keep inflating per-case timeouts as the primary strategy. It hides
  contention and makes every run slow.
- Do not jump to Prow/Cloud Build. That is k8s-test-infra scale, far past what
  Karta needs.
- Do not request an oversized always-on self-hosted fleet. Grove uses one
  pre-baked label; a single larger runner or a small shard matrix is enough.

## What to request (concrete)

A self-hosted GitHub Actions runner for the Karta repo, mirroring grove's
prod-grove-e2e-v1:

- Label: prod-karta-e2e-v1
- OS / arch: Linux, amd64 (x86_64). amd64 is required: the Ray image is
  arch-native and crash-loops under qemu emulation.
- Size: 8 vCPU, 32 GB RAM, 150 GB SSD (minimum 4 vCPU / 16 GB).
- Docker: required (the suite builds the operator image and runs kind, which
  needs docker-in-docker or a docker daemon on the host).
- Tools: a vanilla Docker-capable runner is fine; the workflow installs kind,
  kubectl, and helm itself. Pre-baking them (grove's approach) only saves setup
  time, it is not required.
- Sizing rationale: the full 10-operator install already fit in 2 vCPU / 7 GB, so
  memory was not the blocker; the Ray dashboard boot was CPU-starved on 2 shared
  vCPU. 8 vCPU removes that; 32 GB is headroom for future operators.

Once provisioned, run the full suite with the runner input set to the label:
gh workflow run e2e.yaml -f runner=prod-karta-e2e-v1 (after the workflow is on
the default branch), or set it in the workflow_dispatch form. ubuntu-latest stays
the fallback for subset runs.

## Per-repo index

- cert-manager: Prow, single pod, Ginkgo nodes=40 - ./e2e-runner-size-research/cert-manager.md
- kueue: Prow, E2E_NPROCS sharding - ./e2e-runner-size-research/kueue.md
- KAI-Scheduler: ubuntu-latest, 11-way matrix - ./e2e-runner-size-research/KAI-Scheduler.md
- grove: self-hosted prod-grove-e2e-v1, pre-baked - ./e2e-runner-size-research/grove.md
- cluster-api: Prow, GINKGO_NODES=3 - ./e2e-runner-size-research/cluster-api.md
- crossplane: ubuntu-24.04, matrix + retry - ./e2e-runner-size-research/crossplane.md
- volcano: ubuntu-24.04, ~21 parallel jobs - ./e2e-runner-size-research/volcano.md
- keda: self-hosted "e2e" pool + AKS - ./e2e-runner-size-research/keda.md
