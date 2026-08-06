<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# verify

Proves a Karta definition against a real workload object, offline.

`KartaValidator` proves a definition is well formed. It does not prove that any
jq path resolves against a real object, so a definition can pass validation and
still extract nothing. This command validates the definition, runs it against a
real custom resource, and reports what came out.

## Predict first

Run it with `--predict` and it checks the extraction against values written
before the run. Reading the output after the fact invites rationalizing whatever
appears; committing to the numbers first does not.

```bash
go run . --karta ./mydef.yaml --workload ./real-cr.yaml --predict ./predict.yaml
```

Predictions are partial. Only the fields present are compared, so a prediction
may cover one component or one field.

```yaml
status: [Running]
components:
- key: group          # "name", or "name[instanceId]" for a multi-instance component,
  replicas: 3         # prefixed by the owner path when nested, e.g. "group/leader"
- key: group/leader
  replicas: 3
  containers: [nginx2]
  podSpec: true
- key: group/worker
  replicas: 9
```

## Flags

| Flag | Purpose |
|---|---|
| `--karta` | Path to the Karta definition. Required. |
| `--workload` | Path to a real workload manifest. Required. |
| `--predict` | Predictions file to check the extraction against. |
| `--dump` | Write the observed extraction, in predictions format. |
| `--strict` | Exit non-zero when the run reports warnings. |

Exit codes: 0 success, 1 load or validation failure, 2 prediction mismatch,
3 warnings under `--strict`. Note that `go run` reports a non-zero program exit
as `exit status N` on stderr and itself exits 1. To branch on the exact code,
build first: `go build -o verify . && ./verify ...`.

## Warnings

Every warning is a valid-but-empty extraction, which is the failure class the
validator cannot see:

- The status is unresolved, so no rule in `statusDefinition` matched the object.
- A component declares a `specDefinition` but extracted no pod spec, so the spec
  path missed.
- A component extracted a pod spec with no containers.
- A component produced no instances, so `instanceIdPath` matched nothing.

## Example

Run against a definition and a manifest already in the repository:

```bash
go run . \
  --karta ../../../../docs/catalog/leaderworkerset-x-k8s-io-leaderworkerset-v1.yaml \
  --workload ../../../../docs/examples/quickstart/lws.yaml
```

The manifest declares `replicas: 3` and `size: 4`. The `group` component reports
4 while its sibling `leader` reports 3, which is the sibling-consistency failure
described in the scale section of `reference/technical-guide.md`.
