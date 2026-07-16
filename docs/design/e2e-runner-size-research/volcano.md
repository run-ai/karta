<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# volcano: e2e runner specs

## TL;DR

Volcano runs all of its kind-based e2e suites on GitHub-hosted standard
runners labelled `ubuntu-24.04`. There are no larger runners and no
self-hosted runners anywhere in `.github/workflows/`. The runner is the
default 2-vCPU / 16 GB hosted Linux image; Volcano compensates for the
small disk by running a "Free Disk Space" step at the start of every e2e
job rather than by paying for a bigger machine.

Parallelism comes from splitting e2e into many separate reusable
workflows fanned out by `e2e.yaml`, not from one big matrix. There are
14 e2e child jobs, and one of them (`e2e_shard.yaml`) is itself a 7-entry
matrix, so a single e2e run schedules roughly 21 parallel e2e jobs, each
on its own `ubuntu-24.04` runner. Each kind cluster is 5 nodes
(1 control-plane + 4 workers), defined in `hack/e2e-kind-config.yaml`.

Evidence base: repo at commit 8bcdc6f, default branch `master`,
remote `github.com/volcano-sh/volcano`.

## How it works

Orchestration. `.github/workflows/e2e.yaml` is the top-level E2E Tests
workflow. It first runs a `build-images` job (reusing
`docker_images.yaml`) and then fans out to 14 sibling e2e jobs, each of
which `uses:` a dedicated reusable workflow (admission, cronjob, dra,
gangevict, hypernode, parallel-jobs, scheduling-actions,
scheduling-gates, scheduling-basic, sequence, spark, vcctl, shard). All
14 depend only on `build-images`, so once images are built they run in
parallel.

Runner label. Every e2e reusable workflow pins
`runs-on: ubuntu-24.04`. This is a GitHub-hosted standard runner (2 vCPU,
16 GB RAM, ~14 GB usable SSD on the public tier). No workflow uses a
`*-large`, `*-cpu`, `*-4-core`, or self-hosted label. The only other
labels in the repo are `ubuntu-latest` for non-e2e housekeeping jobs
(codeql, fossa, release, stale, scorecards, sync-apis, workflows-approve,
release_chart).

Disk-space workaround. Because the standard runner's disk is small,
every e2e job begins with `jlumbroso/free-disk-space` and clears
android, dotnet, haskell, large-packages, docker-images, and swap. The
`large-packages: true` value seen in grep is an input to that cleanup
step, not a runner-size selector.

Toolchain per job. Each e2e job installs Go 1.25.x, kind v0.31.0
(`go install sigs.k8s.io/kind@v0.31.0`), and kubectl for Kubernetes
v1.35.0, downloads the prebuilt `volcano-images` artifact, runs
`make load-images`, then runs a `make e2e-test-*` target with
`FORCE_REBUILD=false`.

Make targets to kind. The `make e2e-test-*` targets in `Makefile` all
shell out to `hack/run-e2e-kind.sh`, differing only by `E2E_TYPE` and
feature-gate env vars. `run-e2e-kind.sh` defaults
`KIND_OPT="--config hack/e2e-kind-config.yaml"` and calls
`kind create cluster` via `hack/lib/install.sh`.

Kind node count. `hack/e2e-kind-config.yaml` declares 5 nodes:
1 `control-plane` plus 4 `worker` nodes (the file comment says
"1 control plane node and 4 workers"). Every suite that uses the default
`KIND_OPT` therefore gets the same 5-node cluster on a single 2-vCPU
runner.

Matrix sharding. Only `e2e_shard.yaml` uses a GitHub matrix. It is a
7-entry `matrix.include` (agent-scheduler none/soft/hard, sharding
controller, scheduler-sharding none/soft/hard) with
`fail-fast: false`, each entry a separate `ubuntu-24.04` job. So the 14
child workflows expand to about 21 parallel e2e jobs per run
(13 single-job workflows + 7 shard matrix entries; `e2e_admission.yaml`
actually has 2 jobs, so the real count is ~22).

Spark exception. `e2e_spark.yaml` is the one e2e suite that does not use
kind. It runs on `ubuntu-24.04` too but starts minikube with
`minikube start --cpus max --memory max`, then drives Spark-on-K8s
integration tests.

## Relevance to Karta

Karta is a much smaller project than Volcano and its e2e story (the
`e2e-karta` branch, karta e2e infrastructure) is just getting started.
Volcano is a useful reference for the maximal end of "stay on hosted
runners": a mature CNCF scheduler project runs 20-plus parallel kind
e2e jobs entirely on free `ubuntu-24.04` runners, sharding by suite
rather than paying for bigger machines. That is directly applicable to
how Karta should size its own kind-based e2e as it grows: reach for more
parallel standard runners and a disk-cleanup step before reaching for
larger or self-hosted runners.

## Evidence

All paths relative to the volcano repo root
(github.com/volcano-sh/volcano, branch master, commit 8bcdc6f).

- `.github/workflows/e2e.yaml` lines 16-71: top-level fan-out;
  `build-images` plus 14 e2e child jobs, each via `uses:`.
- `.github/workflows/e2e_scheduling_basic.yaml` line 10:
  `runs-on: ubuntu-24.04` (representative single-job e2e suite).
- `.github/workflows/e2e_shard.yaml` lines 10-51: `runs-on: ubuntu-24.04`
  with a 7-entry `strategy.matrix.include`, `fail-fast: false`.
- `.github/workflows/e2e_admission.yaml` lines 10 and 65: two jobs, both
  `runs-on: ubuntu-24.04`.
- Grep of `runs-on` across `.github/workflows/`: all e2e_*.yaml files use
  `ubuntu-24.04`; only non-e2e jobs use `ubuntu-latest`. No `self-hosted`,
  `*-large`, or `*-cpu` label exists in any workflow.
- `.github/workflows/e2e_shard.yaml` line 79 (and same in every e2e
  suite): `go install sigs.k8s.io/kind@v0.31.0`; line 80 kubectl v1.35.0.
- `.github/workflows/e2e_scheduling_basic.yaml` lines 14-23: the
  `jlumbroso/free-disk-space` step with `large-packages: true` as an
  input (disk cleanup, not runner size).
- `Makefile` lines 199-248: `make e2e-test-*` targets all call
  `./hack/run-e2e-kind.sh` with different `E2E_TYPE` values.
- `hack/run-e2e-kind.sh` line 35:
  `export KIND_OPT=${KIND_OPT:="--config ${VK_ROOT}/hack/e2e-kind-config.yaml"}`.
- `hack/lib/install.sh` lines 21-22: `kind create cluster ... ${KIND_OPT}`.
- `hack/e2e-kind-config.yaml` lines 12-66: comment "1 control plane node
  and 4 workers"; one `role: control-plane` node plus four `role: worker`
  nodes = 5 nodes total.
- `.github/workflows/e2e_spark.yaml` line 11 `runs-on: ubuntu-24.04` and
  line 77 `minikube start --cpus max --memory max`: the lone non-kind
  e2e suite.

## Lessons for Karta

- Hosted standard runners are enough for real kind e2e at scale. A large
  CNCF project runs its entire kind e2e matrix on free `ubuntu-24.04`.
- Shard by suite across many jobs. Volcano gets parallelism from ~14
  reusable workflows fanned out by one parent, plus a small matrix for
  the shard suite, instead of one giant matrix. This keeps each job
  focused and lets `fail-fast: false` surface all failures at once.
- Build images once, load per job. A single `build-images` job uploads a
  `volcano-images` artifact; every e2e job downloads it and runs
  `make load-images`. This avoids rebuilding per suite and keeps each
  runner's work to cluster-up plus tests.
- Free disk space as a first step. On the small standard-runner disk, an
  explicit cleanup step (android, dotnet, docker-images, swap) is a
  cheaper fix than a larger runner.
- Keep the kind topology in one config file. `hack/e2e-kind-config.yaml`
  centralises the 5-node layout and feature gates, so every suite gets
  an identical cluster and the node count is easy to audit.
- Pin tool versions inline. kind v0.31.0 and kubectl v1.35.0 are pinned
  in each workflow, and third-party actions are pinned by commit SHA.

## What NOT to copy

- Do not assume you need self-hosted or larger runners for kind e2e.
  Volcano deliberately does not use them; start on `ubuntu-24.04` and
  only escalate if a specific suite proves it needs more.
- Do not fan every suite out into a separate workflow file blindly.
  Volcano has 15-plus e2e workflow files, which is a lot of duplicated
  boilerplate (each repeats free-disk-space, Go install, kind install,
  image download, cache). For a smaller project like Karta, one workflow
  with a matrix is likely cleaner than many near-identical files.
- Do not copy the minikube-with-`--cpus max --memory max` pattern from
  the spark suite as a general approach; it is a special case for
  Spark-on-K8s and is at odds with the rest of the kind-based suites.
- Do not treat `large-packages: true` as a runner-sizing knob. It is an
  input to the disk-cleanup action; copying it without the
  `free-disk-space` step around it does nothing.
- Do not hardcode a 5-node cluster if Karta's tests do not need it.
  Volcano's 4-worker layout suits a scheduler under test; size the kind
  cluster to what Karta actually exercises.
