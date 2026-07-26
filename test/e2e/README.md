<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta end-to-end tests

Karta reads any workload from a catalog of definitions (`docs/catalog`). This
suite is how we guarantee those definitions stay correct: it brings up real
operators, runs real workloads, and records what they do. Karta itself is checked
offline, against those recordings, by `go test ./test/conformance` (see
`test/conformance`) - never in the live run, so the recorder can never validate
Karta by asking Karta.

## Online and offline

- Online, `make record-e2e`: against a live cluster, drive each workload through its states (firing
  actions like resume where the operator will not), and record what it saw - for each state, the CR
  (first CR, then a merge-patch per change), the state judged from the workload's own fields, and what
  Karta read of it (also a first value plus patches) - as one `<flow>.yaml`. `make test-e2e` drives the
  same workloads and checks the order, but does not check Karta and writes nothing.
- Offline, `go test ./test/conformance`: with no cluster, rebuild each CR and reading, run the CR
  through the current Karta, and check it matches the recorded state at every step, that its reading
  matches, and that the sequence is a legal transition ending at `want`. This is the only place Karta
  is checked. A change that misreads any recorded workload fails fast, for every operator and version
  captured. There is no sanitize denylist - the golden rebuilds the exact CR, so Karta reads the same
  bytes back and per-run volatile fields never change the result.

## Run

The suite is online. It needs a cluster with the operators already installed.

```sh
make e2e-up      # kind cluster + operators (once)
make test-e2e    # run the checks; writes nothing
make record-e2e  # run the checks and record fixtures under test/conformance/
make e2e-down    # tear it down
```

Install and test a subset with the same `WORKLOADS` list:

```sh
make e2e-up   WORKLOADS="nim"   # only the NIM operator
make test-e2e WORKLOADS="nim"   # only the NIMService case
```

While adding or fixing one case, `FLOW="<name>"` narrows a record to a single flow, so you
re-record just what changed instead of the whole operator:

```sh
make record-e2e WORKLOADS="kuberay" FLOW="suspended"  # just the RayCluster suspended flow
make record-e2e WORKLOADS="kubeflow" FLOW="failed"    # every failed flow under the kubeflow label
```

Operator versions are pinned in `hack/e2e/global.env`, each overridable from the
environment. To check Karta against a different version, edit the file or set the
variable for the run:

```sh
KUBERAY_VERSION=1.7.0 make e2e-up WORKLOADS="kuberay"
```

## What a run checks

For each case, in order:

1. Create the Karta CR and wait for the operator to mark it Ready.
2. Apply a real workload and let the operator drive it.
3. Decide what state the workload is in from its own fields (`status.phase`, a
   condition, a replica count), never from Karta. Asking Karta when to check
   Karta would just let it agree with itself.
4. Check the workload moved through its states in the declared order.
5. Record each state's CR and what Karta read of it. Karta is not checked here:
   whether it reads each state correctly, and extracts the same components, is
   asserted offline against the recording by `go test ./test/conformance`. The
   live run stays a pure recorder.

Only stable states are recorded, never a transient one, so the offline golden does
not flake.

## Add a case or a flow

A case is one `workloadCase` in `cases_<type>.go`. It names its states once (the
registry, each a check on the workload's own fields), then each flow is a journey
through those states, with a manifest under `testdata/<type>/<flow>.yaml`:

```go
states: []namedState{
    {initializing, phaseEq("Pending", "status", "phase")},
    {running, phaseEq("Running", "status", "phase")},
    {failed, phaseEq("Failed", "status", "phase")},
},
flows: []flow{
    {name: "failed", workloadFile: "testdata/pod/failed.yaml", journey: steps(initializing, running, failed)},
},
```

A step can carry an action, a mutation the operator will not make itself. A resume
flow reaches Suspended, resumes, then runs to completion:

```go
journey: []step{{state: suspended, action: unsuspend}, {state: running}, {state: completed}},
```

Add the flow, drop its manifest next to the others, run `make record-e2e` once, and
commit the fixtures. `want` is the last step's state.

## How it works

- Recorder: watches the workload from creation, classifies each settled CR from its own fields, and
  keeps one CR per state change (a return to an earlier state is kept, so a backwards jump is caught).
  Deduping on the state needs no denylist to tell a real change from resourceVersion churn.
- Recording: the first kept CR is stored in full; every later state is a merge-patch (RFC 7386) from
  the CR before it, tagged with the state it reaches and the action fired there. What Karta read of each
  CR is stored the same way (first reading + patches) as `expected`. The offline golden applies the
  patches to rebuild each CR and reading, re-runs Karta, and diffs. No sanitize - the golden rebuilds
  the exact CR, so the reading is stable without a denylist. Refresh the reading after an intended
  library change with `make regolden` (offline, no cluster).
- Actions: a step's action fires once when its state is reached, in journey order, to drive a
  transition the operator will not make itself (e.g. clear `spec.suspend` to resume). Put actions on
  states the operator holds at, like Suspended.

## Prerequisites

docker, kind, kubectl, helm, Go. No GPU.

## Coverage

| Workload type | Operator |
|---|---|
| Pod, BatchJob | built-in |
| JobSet | jobset |
| RayCluster, RayJob | kuberay |
| PyTorchJob | kubeflow |
| MPIJob | mpi-operator |
| LeaderWorkerSet | lws |
| KnativeService | knative |
| KServe InferenceService | kserve |
| Milvus | milvus |
| Grove PodCliqueSet | grove |
| DynamoGraphDeployment | dynamo |
| NIMService | nim |
