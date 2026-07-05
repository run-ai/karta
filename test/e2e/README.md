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
./hack/e2e/up.sh --list kserve           # print the resolved install plan, then exit
```

The base (kind cluster, cert-manager, fake-gpu-operator, Karta operator) is
always installed. A subset auto-includes dependencies: kserve pulls in knative,
dynamo pulls in grove. Pair this with the Ginkgo `-run` filter below to exercise
just the matching cases.

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

Each case is one entry in `cases_test.go` plus a workload fixture under
`testdata/`. Every case is real operator-driven: the upstream operator is
installed and drives a real workload to a stable state; the test waits for that
state, then runs Karta against the live object. There is no status injection.

The one variation is built-in cases (BatchJob): the root type is a built-in
Kubernetes kind (no CRD), so the operator reports `CRDExists=False` (it only
inspects CustomResourceDefinitions) while the library still maps and extracts.

Assertions only ever target stable states (never a transient mid-state the
operator can pass through), which is what keeps the suite non-flaky.

## Coverage (13 sample types)

| Sample | Mode | Karta maps to |
|---|---|---|
| LeaderWorkerSet | real operator | Running |
| JobSet | real operator | Completed |
| RayCluster | real operator | Running |
| PyTorchJob | real operator | Running |
| MPIJob | real operator | Completed |
| NIMService | real operator (fictive CPU image) | Running |
| KnativeService | real operator (Knative Serving + Kourier) | Running |
| KServe InferenceService | real operator (Serverless, sklearn CPU model) | Running |
| Milvus | real operator (standalone) | Running |
| RayJob | real operator (KubeRay) | Completed |
| Grove PodCliqueSet | real operator (Grove + kai-scheduler) | Running |
| DynamoGraphDeployment | real operator (mocker backend, CPU) | Running |
| BatchJob | built-in | Completed |

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

Versions are pinned and overridable in `hack/e2e/versions.env`. Each operator's
install lives in its own module under `hack/e2e/operators/<name>/` (an
`install.sh` plus any co-located config and a `smoke.yaml`).

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

- A live running `kubeflow.org/v1 MPIJob` maps to `Undefined`: the sample's
  `running` mapping requires `Succeeded=False`/`Failed=False` conditions that the
  v1 training-operator never sets. The MPIJob case asserts `Completed` instead;
  see the note in `testdata/mpijob-workload.yaml`.
- Built-in kinds report `CRDExists=False` because the operator only inspects
  CRDs; the library still works (BatchJob case).
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

Install side (a self-contained module under `hack/e2e/operators/<name>/`):

1. Create `hack/e2e/operators/<name>/install.sh` defining `operator_install()`.
   Use the shared helpers from `_common.sh` (`rollout_wait`, `apply_with_retry`,
   `preload_image`, `build_and_load_image`, `ensure_secret`) and reference any
   co-located config via `${MODULE_DIR}`.
2. Optional fail-fast smoke: drop a `smoke.yaml` in the module dir and set
   `SMOKE_TARGET` and `SMOKE_WAIT` (and optional `SMOKE_TIMEOUT`/`SMOKE_NS`) at
   the top of `install.sh`.
3. Pin its version(s) in `hack/e2e/versions.env`.
4. Add `<name>` to `ALL_WORKLOADS` in `hack/e2e/up.sh` (in install order), and a
   `deps_of` entry only if it depends on another operator.

Test side (one entry, unchanged): add a `testdata/<name>-workload.yaml` fixture
and a `workloadCases` entry in `cases_test.go` (the sample path, the target
state predicate, the expected `ResourceStatus`, and optional `extracts`/`builtin`).
