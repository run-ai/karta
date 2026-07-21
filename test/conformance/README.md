<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta conformance

The offline replay engine (package `conformance`) and the fixtures it replays
(`fixtures/`). The live recorder that writes those fixtures lives in `test/e2e`.

Each fixture is a recorded reading of a real workload, one tree per operator, version,
definition, and flow:

```text
fixtures/<operator>/<version>/<definition>/<flow>/
  fixture.yaml              # index: schema version, observed states, snapshots
  NN-<State>/
    cr.yaml                # the sanitized workload CR at this state
    expected.yaml          # what Karta read from it, scrubbed
```

`NN-<State>` is one settled CR the workload passed through, in order (a state it holds
across several CRs becomes several snapshots, so golden checks each, not just the last).
The state is judged from the workload's own fields, not from Karta. `expected.yaml` is the
full reading - status, phase, conditions, and the whole per-instance extraction - with
per-run volatile fields (uids, timestamps, the node a pod landed on) stripped, so any
change to how Karta reads a field surfaces in the diff.

## Working with fixtures

- Verify (every PR, no cluster): `go test ./test/conformance -run TestGolden`
- Refresh offline after changing what `Replay` extracts: `go run ./hack/regolden`
- Re-record from live workloads: `make record-e2e` (needs a cluster; see `test/e2e`)

Do not hand-edit fixtures. The set of fixtures is Karta's tested matrix - which workload
types it reads correctly, at which versions, through which states.
