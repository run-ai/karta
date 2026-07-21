<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta end-to-end tests

This suite verifies Karta against a real cluster. For each bundled definition in
`docs/samples`, it confirms two things:

1. the Karta operator reconciles the definition (`Validated`, `CRDExists`,
   `Ready` conditions), and
2. the Karta library reads a live workload of that type correctly: its status
   maps to the expected `ResourceStatus` and its components extract.

The suite is its own Go module so the controller-runtime and client test
dependencies stay out of the published `github.com/run-ai/karta` library. It
connects to the current kube context; it does not create the cluster (the
cluster is provisioned by `hack/e2e/up.sh`, mirroring how Grove and cert-manager
separate cluster setup from the test binary).

## Run

```sh
make e2e-up      # provision a local kind cluster + all operators (one time)
make test-e2e    # run this suite against it
make e2e-down    # tear the cluster down
```

To bring up only a subset of operators (lighter and faster), pass `WORKLOADS`
or call the script directly:

```sh
make e2e-up WORKLOADS="jobset kuberay"   # base + only these operators
make e2e-up-jobset                       # base + a single operator (tab-completes)
./hack/e2e/up.sh dynamo lws              # same, calling the script directly
./hack/e2e/up.sh --list kserve           # print the resolved install plan, then exit
```

The base (kind cluster, cert-manager, fake-gpu-operator, Karta operator) is
always installed. A subset auto-includes dependencies: kserve pulls in knative,
dynamo pulls in grove. Pair this with the Ginkgo `-run` filter below to exercise
just the matching cases.

### Run only the cases for the operators you installed

Each case carries a Ginkgo label equal to its `hack/e2e` operator key (plus
`builtin` for the built-in kinds), so `E2E_LABELS` runs only the matching cases -
the same vocabulary as `WORKLOADS`:

```sh
make e2e-up  WORKLOADS="nim"    # bring up only the nim operator
make test-e2e E2E_LABELS="nim"  # run only the NIMService case (others are skipped)
```

Cases whose operator you did not install are simply not selected, so they never hit
the 1-minute "CRD does not exist" reconcile failure. Label set-expressions compose
(`E2E_LABELS="kuberay || nim"`, `E2E_LABELS="!builtin"`, `E2E_LABELS="builtin"` for
all built-in kinds on any base cluster), and `E2E_LABELS` intersects (AND) with the
`E2E_FOCUS` name regex.

### Parallel / isolated clusters

Set `CLUSTER_NAME` to bring up more than one cluster at a time, for example two
CI shards that install different operator subsets:

```sh
make e2e-up CLUSTER_NAME=shard-a WORKLOADS="jobset kuberay"   &
make e2e-up CLUSTER_NAME=shard-b WORKLOADS="knative kserve"   &
wait
make test-e2e CLUSTER_NAME=shard-a -run 'RayCluster'
make test-e2e CLUSTER_NAME=shard-b -run 'KServe'
make e2e-down CLUSTER_NAME=shard-a
make e2e-down CLUSTER_NAME=shard-b
```

Each non-default cluster gets its own kubeconfig at
`~/.kube/kind-<CLUSTER_NAME>.kubeconfig`, so parallel runs never race on the
shared current-context. The default cluster (`karta-e2e`) keeps using the
standard kubeconfig, so plain `kubectl` works after `make e2e-up`. Override
`KUBECONFIG` to place it elsewhere.

`make test-e2e` runs the Ginkgo specs with a 20-minute suite timeout and test
caching disabled (cluster-dependent results must never be cached). Filter with
standard Ginkgo flags, for example `go test -run 'NIMService'` from `test/e2e`.

## Prerequisites

docker, kind, kubectl, helm, and Go. No GPU is required: GPU-oriented workloads
either run CPU-only or use a fictive image (see NIMService below).

## How a case is tested

Each case is one entry in a `cases_<type>_test.go` file plus its workload
manifests under `testdata/<type>/<flow>.yaml`, one file per flow (for example
`testdata/pod/happy.yaml`, `testdata/pod/failed.yaml`). Every case is real
operator-driven: the upstream operator is
installed and drives a real workload to a stable state; the test waits for that
state, then runs Karta against the live object. There is no status injection.

Assertions only ever target stable states (never a transient mid-state the
operator can pass through), which is what keeps the suite non-flaky.

A case can drive its workload through more than one flow: the happy path, a
failure, a suspend, a degraded state. Flows are declared as `flows` in
`cases_test.go`, each with its own workload manifest, the ordered states it passes
through, and the terminal `ResourceStatus` Karta should report.

## Conformance fixtures (record and replay)

`make record-e2e` runs the suite in recording mode. For each flow it watches the
workload and captures every DISTINCT settled CR it passes through: the sanitized CR plus
what the Karta library reads from it, written under
`test/conformance/fixtures/<operator>/<version>/<karta>/<flow>/NN-<State>/`. Snapshots are
deduplicated by sanitized content, so pure resourceVersion churn is dropped but any real
status change is its own snapshot - even when it classifies to the same state as its
neighbour, so a single `Running` window that the workload evolves through is several
`NN-Running` snapshots. The state (the `NN-<State>` label) is judged from the workload's
own fields, never from Karta, so the recorded expected output is never compared against
itself.

Recording every distinct CR - not one per declared state - is what makes the offline
test a real regression guard. If the workload sits in `Running` across several CRs that
today all read `Running`, a later library change that would read one of those middle CRs
as `Degraded` only shows up if that middle CR was recorded. Collapsing to one snapshot
per state would hide it. The cost is that a re-record is not byte-for-byte reproducible
(which intermediate CRs a watch delivers is timing dependent); a fixture set is a frozen
regression baseline, re-recorded when the operator version bumps, not on every run.

A flow reaches `Initializing` by holding the workload not-ready long enough to observe:
an init container keeps a Pod (and an operator's `Created` condition) pending; a
readiness probe keeps a Job pod active but `ready==0`, which Karta maps to `initializing`
before `running`.

the `test/conformance` package replays those fixtures through the library with no cluster
and no operators. `go test ./test/conformance -run TestGolden` loads every
recorded CR - each intermediate, not just the terminal - runs Karta against it, and
asserts the reading still matches. This is the fast offline guard that a library change
has not silently altered how any real, recorded state is read.

Golden also checks the transition sequence, not just each state in isolation: the
snapshot directories are `NN-<State>` in order, collapsing them reproduces the
fixture's `observedStates`, and the terminal snapshot reads as the flow's declared
`want`. The recorder additionally asserts, at record time, that the observed states
are a monotonic subsequence of the flow's declared order (a fast workload may skip a
settled intermediate, but it never regresses or ends on the wrong state).

## Coverage (17 sample types)

| Sample | Mode | Karta maps to |
|---|---|---|
| LeaderWorkerSet | real operator | Running |
| JobSet | real operator | Initializing, Running, Completed, Failed, Suspended |
| RayCluster | real operator | Running, Suspended |
| PyTorchJob | real operator | Initializing, Running, Completed, Failed, Suspended |
| MPIJob | real operator | Initializing, Running, Completed, Failed, Suspended |
| NIMService | real operator (fictive CPU image) | Initializing, Running |
| KnativeService | real operator (Knative Serving + Kourier) | Running |
| KServe InferenceService | real operator (Serverless, sklearn CPU model) | Running, Failed |
| Milvus | real operator (standalone) | Running |
| RayJob | real operator (KubeRay) | Running, Completed, Failed, Suspended |
| Grove PodCliqueSet | real operator (Grove + kai-scheduler) | Initializing, Running |
| DynamoGraphDeployment | real operator (mocker backend, CPU) | Initializing, Running |
| Pod | built-in | Initializing, Running, Completed, Failed |
| BatchJob | built-in | Initializing, Running, Completed, Failed, Degraded, Suspended |
| Deployment | built-in | Running, Failed |
| StatefulSet | built-in | Running, Degraded |
| CronJob | built-in | Initializing, Running, Suspended |

Most cases carry more than one flow, so a single kind exercises several
`ResourceStatus` values (a Job initializes, runs, then completes, fails, degrades, or is
suspended). Across all cases the suite records six of them: Running, Completed, Failed,
Degraded, Suspended, and Initializing. Several flows record the full
`Initializing -> Running -> <terminal>` progression rather than jumping to the terminal.
Suspending and Resuming are not covered because no bundled sample maps them.

## What `make e2e-up` installs

Build + load the Karta operator image, create the kind cluster, then:

- cert-manager and the fake-gpu-operator (the latter also provides the DRA
  `ComputeDomain` CRD that the NIM operator watches).
- Real operators: LeaderWorkerSet, JobSet, KubeRay, Kubeflow training-operator
  (which also serves `kubeflow.org/v1 MPIJob`), the k8s-nim-operator, Knative
  Serving with the Kourier networking layer, KServe (Serverless mode on top of
  Knative + Kourier, no Istio), the milvus-operator, Grove with its kai-scheduler
  backend, and the dynamo-platform (operator + etcd + NATS) - all from published
  multi-arch charts.
- The Karta operator (Helm), with its memory limit raised to 512Mi.

Every sample type now runs against a real operator, so there are no vendored
CRD-only schemas left.

up.sh untaints the control-plane node so its capacity is schedulable: the single
worker cannot hold every operator plus a real Milvus/Knative/KServe workload.

The Ray image is pre-loaded into kind as `ray-e2e:local` (the arch-native variant)
so the RayCluster and RayJob cases do not pull ~3GB from Docker Hub mid-test. The
arch-native pull matters: the amd64 Ray image under qemu emulation crash-loops
when a RayJob runs real work.

After installing an operator, up.sh smoke-tests it (Grove's pattern): it creates a
throwaway resource and waits for the operator to drive it Ready, failing fast if
the operator is broken rather than partway through the suite.

Versions are pinned and overridable in `hack/e2e/global.env`. Each operator lives
in its own folder under `hack/e2e/operators/<name>/`: a standalone `install.sh`
plus any co-located config, and a `verify.sh` with a `smoke.yaml`. up.sh runs
`install.sh` then `verify.sh` as subprocesses, so install and smoke stay separate
and each script also runs on its own.

### The fictive NIM image

NIMService is a real operator-driven case without a GPU or a real NGC token. The
k8s-nim-operator runs a fictive CPU image (`hack/e2e/operators/nim/image`, a small
server answering `/v1/health/ready`); the operator drives the NIMService to
`state=Ready`. A dummy `ngc-secret` (`NGC_API_KEY`) satisfies the operator; the
image ignores it.

### The Dynamo mocker backend

DynamoGraphDeployment is a real operator-driven case without a GPU. The
dynamo-operator runs a Frontend plus a decode worker that executes
`python3 -m dynamo.mocker` (from the public, multi-arch `dynamo-planner` image),
which simulates inference with no model load and no GPU. The operator reports
`state=successful` once the pods are ready and registered through etcd + NATS. A
dummy `hf-token-secret` satisfies the worker; the mocker downloads nothing.

## Known findings (surfaced by this suite)

- The operator can OOM listing all CustomResourceDefinitions when large upstream
  CRDs are installed (`crdExistsForGVK` lists every CRD rather than using its
  index); the chart limit is raised to 512Mi as a workaround.
- `docs/samples/milvus.yaml` omits `metadata.name`; the harness assigns one.
- Real RayJob submission needed three things to be reliable on local kind, all
  handled by up.sh and the fixture: an arch-native Ray image (the amd64 image
  under qemu emulation crash-loops once a job runs real work), pre-loading that
  image into kind (the ~3GB Docker Hub pull was slow and flaky mid-test), and
  KubeRay 1.6.2 (1.2.x recreated the RayCluster on transient submitter hiccups).

## Adding an operator

Install side (the operator folder under `hack/e2e/operators/<name>/`, its version
pin in `global.env`, and the make target) is documented in `hack/e2e/README.md`.

Test side: add a `cases_<type>_test.go` file (or extend an existing one) that
appends a `workloadCase` to `workloadCases`, plus a `testdata/<type>/<flow>.yaml`
manifest per flow. For a single happy path set `workloadFile`, `ready`,
and `want`; for several flows (failure, suspend, degraded) set `flows`, each with
its own workload manifest, ordered state predicates, and expected `ResourceStatus`.
Run `make record-e2e` once to write the conformance fixtures for the new flows.
