<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta end-to-end tests

Karta reads any workload from a catalog of definitions (`docs/catalog`). This
suite is how we guarantee those definitions stay correct: it brings up real
operators, runs real workloads, and checks Karta reads each one the way the
catalog says it should.

## Online, offline, regolden

- Online, `make test-e2e`: against a live cluster, drive each workload and check Karta reads it the
  way the catalog says. `make record-e2e` does the same and freezes what it saw as fixtures.
- Offline, `go test ./test/conformance`: replay those fixtures through the current Karta with no
  cluster, so a change that misreads any recorded workload fails fast, for every operator and
  version captured. This is how a new Karta cannot quietly break the catalog.
- regolden, `go run ./hack/regolden`: when you intentionally change what Karta reads (say you refine
  the jobset `running` mapping), offline goes red on purpose. regolden re-reads the frozen CRs and
  rewrites the expected reading for every fixture at once, all jobset versions and flows, no cluster.
  It rewrites only what Karta says, never the recorded CRs or their `NN-<State>` labels.

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
5. Run Karta on each of those states and check it reads the same one, not only
   the last, then read the final object and check the components extract.

Only stable states are checked, never a transient one, so a run does not flake.

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

- Recorder: watches the workload from creation and keeps every distinct CR it
  settles in, not one per state, so the offline replay also catches a misread of
  a middle CR. Identical back-to-back CRs are dropped as churn.
- Sanitize: strips fields Karta never reads (`resourceVersion`, timestamps,
  `nodeName`, ...) before a CR is frozen, so re-recording is byte-stable and a
  diff means a real change.
- Actions: a step's action fires once when its state is reached, in journey order,
  to drive a transition the operator will not make itself (e.g. clear `spec.suspend`
  to resume). Put actions on states the operator holds at, like Suspended.

## Prerequisites

docker, kind, kubectl, helm, Go. No GPU.

## Coverage

| Sample | Karta maps to |
|---|---|
| Pod (built-in) | Initializing, Running, Completed, Failed |
| BatchJob (built-in) | Initializing, Running, Completed, Failed, Suspended |
| JobSet | Initializing, Running, Completed, Failed, Suspended |

More types land as this infrastructure grows.
