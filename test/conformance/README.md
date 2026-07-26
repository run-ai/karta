<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta conformance

The offline transition test (package `conformance`) and the recordings it replays
(`fixtures/`). The live recorder that writes those recordings lives in `test/e2e`.

Each recording is one flow of a real workload, one file per operator, version, definition,
and flow:

```text
fixtures/<operator>/<version>/<definition>/<flow>/
  recording.yaml
```

`recording.yaml` holds the flow's metadata (`kartaFile`, `want`) and an ordered list of steps,
one per state the workload passed through. Step 0 carries the first CR in full; every later step
carries a merge-patch (RFC 7386) from the CR before it, tagged with the state it reaches (judged
from the workload's own fields, not from Karta) and any action fired there. There is no sanitize:
`TestTransitions` rebuilds each CR by applying the patches, runs it through Karta, and checks Karta
reads the recorded state at every step and that the sequence is a legal path ending at `want`.
Because the checks are on states, per-run volatile fields (uids, timestamps, the node a pod landed
on) ride along in the CR and never change the result.

## Working with recordings

- Verify (every PR, no cluster): `go test ./test/conformance -run TestTransitions`
- Re-record from live workloads: `make record-e2e` (needs a cluster; see `test/e2e`)

Do not hand-edit recordings. The set of recordings is Karta's tested matrix - which workload types
it reads correctly, at which versions, through which states.
