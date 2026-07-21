<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Karta conformance

This package is the offline replay engine (`*.go`, package `conformance`) plus the
recorded fixtures it replays (`fixtures/`). The live recorder that produces the
fixtures lives in the sibling `test/e2e` module.

Recorded readings of real workloads, one directory per operator, version, bundled
definition, and flow:

```
fixtures/<operator>/<version>/<kartaDefinition>/<flow>/
  fixture.yaml              # index: schema version, observed states, snapshots
  NN-<State>/
    cr.yaml                # the sanitized workload CR at this state
    expected.yaml          # what the Karta library read from it (statuses, phase, components)
```

A flow drives a real workload through its states (for example happy, failed,
suspended, resumed). Every distinct settled CR the workload passes through is one
`NN-<State>` directory, in order, deduplicated by sanitized content: a state the
workload sits in across several CRs becomes several snapshots (`00-Running`,
`01-Running`, ...), so golden replays each intermediate, not just the terminal. The
`NN-<State>` label is judged from the workload's own fields, never from Karta, so the
recorded expected output is never compared against itself. Volatile fields
(resourceVersion, timestamps, uids) are stripped. A re-record is NOT byte-for-byte
reproducible (which intermediate CRs a watch delivers is timing dependent); a fixture
set is a frozen regression baseline, refreshed when the operator version bumps.

## Working with fixtures

- Record or refresh: `make record-e2e` against a cluster that has the operators
  installed (see `test/e2e/README.md`). Do not hand-edit fixtures.
- Verify offline: `go test ./test/conformance -run TestGolden` replays every
  fixture through the current library with no cluster and no operators, and fails if
  the library's reading of any recorded snapshot changed. This is the per-version
  guarantee, run on every PR.

The set of fixtures here is the tested operator matrix for a Karta release: which
workload types Karta is known to read correctly, at which operator versions, through
which lifecycle states.
