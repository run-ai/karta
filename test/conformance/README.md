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

## What a fixture holds

The header names the recording and tells the golden what to load:

- `operator`, `version` - the workload type and the operator version installed when it was recorded.
- `kartaName`, `kartaFile` - the definition's name and its path in the repo; the golden loads the file.
- `flow` - the scenario name (running, completed, scaled, ...).
- `want` - the state the flow must end at.
- `schemaVersion` - the file format version; a bump makes stale fixtures fail fast instead of mis-reading.

`steps` is an ordered list, one entry per distinct CR the workload moved through (so a scale flow can
list the same state more than once). Each step carries:

- `state` - the state judged from the workload's own fields (a condition, `status.phase`, a replica
  count), never from Karta. This is the ground truth the golden anchors on.
- `action` - the action fired here (resume, scale, ...), for provenance; empty if none.
- the CR - the full object on the first step (`cr`), then a merge-patch (RFC 7386) from the step before
  it (`patch`) on every later step. This is the input Karta reads.
- `expected` - what Karta read of that CR (matched statuses, phase, conditions, the per-component
  extraction), stored the same way: full on the first step (`expected`), then `expectedPatch` after.
  This is Karta's saved answer.

The definition (the rules) and the Karta library are not stored - only `kartaFile` points at the
definition, and the golden runs the current library. So the fixture is the input (`cr`) plus Karta's
answer (`expected`), and the golden checks the current library still produces that answer.

## Example

A two-step `batch-job/running` fixture, the Job going Initializing then Running (the pod template in
`cr` is elided):

```yaml
operator: batch-job
version: v1.34.0
kartaName: batch-job-v1
kartaFile: docs/catalog/batch-job-v1.yaml
flow: running
want: Running
schemaVersion: 6
steps:
  - state: Initializing
    cr: { ...Job..., status: {active: 1, ready: 0} }  # full CR, once
    expected: { matchedStatuses: [Initializing] }     # Karta's reading, once
  - state: Running
    patch: { status: {ready: 1} }                     # only what changed in the CR
    expectedPatch: { matchedStatuses: [Running] }     # only what changed in the reading
```

Step one stores the whole Job and Karta's whole reading. Step two stores only the delta: `ready` went
to 1, and Karta's answer became Running. Storing a first value plus per-step patches keeps each file to
just what changed between states.

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
