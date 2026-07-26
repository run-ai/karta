<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta conformance

The offline golden test (package `conformance`) and the recordings it replays (`fixtures/`). The live
recorder that writes those recordings lives in `test/e2e`. This is the only place Karta is checked: the
e2e run just drives real workloads and records what they did, and the golden here asserts Karta reads
them correctly, with no cluster.

Each recording is one flow of a real workload, one file per operator, version, definition, and flow:

```text
fixtures/<operator>/<version>/<definition>/<flow>.yaml
```

The file holds the flow's metadata (`kartaFile`, `want`) and an ordered list of steps, one per state
the workload passed through. Each step carries three things:

- `state` - the state judged from the workload's own fields (`status.phase`, a condition, a replica
  count), never from Karta. This is the ground truth.
- the CR - the full object on the first step (`cr`), then a merge-patch (RFC 7386) from the step before
  it (`patch`) on every later step.
- `expected` - what Karta read of that CR (matched statuses, phase, conditions, and the per-component
  extraction), stored the same way: full on the first step (`expected`), then a merge-patch
  (`expectedPatch`) after.

Storing the CR and the reading as a first value plus per-step patches keeps each file to just what
changed between states.

## What the golden checks

`TestGolden` rebuilds each CR and each reading by applying the patches, runs the CR through the current
Karta library, and at every step asserts:

- Correctness anchor: Karta matches the recorded `state`. Because the state came from the workload's
  own fields, a refreshed golden can never quietly accept Karta drifting away from what the workload
  actually did.
- Golden: Karta's reading equals the recorded `expected`. This catches any change in what Karta reads,
  not only a wrong state.
- Legal transition: a terminal state (`Completed`, `Failed`) appears only as the last step, and the
  last step is `want`.

There is no sanitize and no denylist. The golden rebuilds the exact CR that was recorded, so Karta
reads the same bytes back and the diff is stable; per-run volatile fields (uids, timestamps, the node a
pod landed on) ride along in both the CR and the reading and never change the result. Only a re-record
against a live cluster changes them.

## Working with recordings

- Verify (every PR, no cluster): `go test ./test/conformance -run TestGolden`
- Refresh the reading after an intended library change (no cluster): `make regolden`. It replays each
  frozen CR through the current Karta and rewrites only `expected`; the CRs and states are untouched,
  and the correctness anchor still fails if Karta no longer reads a recorded state.
- Re-record from live workloads: `make record-e2e` (needs a cluster; see `test/e2e`).

Do not hand-edit recordings. The set of recordings is Karta's tested matrix - which workload types it
reads correctly, at which versions, through which states.
