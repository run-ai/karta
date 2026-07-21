<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# e2e-karta: real-data conformance record/replay for the Karta library

this branch builds a two-sided testing system for the Karta library. one side runs
each bundled Karta definition against a REAL workload driven by its real operator on a
kind cluster, waits for stable states, and records the raw CRs plus what Karta reads
from them. the other side replays those recorded CRs offline (no cluster, no operators)
as fast golden unit tests, so any change to the library that alters how a real workload
is read fails on a PR. it does NOT change any library or CRD code - it is test infra and
recorded data only.

- branch: `e2e-karta`   base commit: `4488160` (adds the provisioner + skeleton suite)
- net (on top of the skeleton suite): `test/conformance` (6 go files: the replay engine +
  tests), 42 recorded fixtures under `test/conformance/fixtures/`, `test/e2e` harness
  rewritten (17 per-type case files, 42 testdata manifests), `hack/resanitize` (new tool),
  Makefile `record-e2e`/`E2E_LABELS`, docs.

> review decisions already applied (were open questions, now done):
> 1. the definition-sha guard was DROPPED. golden no longer pins `docs/samples/<def>.yaml`
>    by sha; a definition edit that changes a reading now surfaces as a normal field diff
>    ("you meant to, re-record"), and one that changes nothing (a comment) no longer forces
>    a re-record. the whole test stays a pure library-regression guard.
> 2. the replay engine + fixtures MOVED under `test/`: code at `test/conformance/` (was
>    `internal/conformance/`, now a public package), data at `test/conformance/fixtures/`
>    (was the top-level `conformance/`). paths below reflect the final layout.

read part A to understand how the system works, part B for each meaningful decision (put
your review comments there), part C for what should go in the first small PR.

---

# PART A - how the system works

## A1. the two-sided design

the core problem: Karta reads ANY workload CR (Deployment, RayJob, NIMService, ...) and
maps it to a `ResourceStatus` (Running, Degraded, Completed, ...) plus extracted
components. to trust that mapping you need real operator output, but you cannot run 15
operators on every PR. so the system splits in two:

```
  RECORD (slow, needs a cluster - make record-e2e)          REPLAY (fast, offline - go test)
  ---------------------------------------------------       -------------------------------------
  real operator drives a real workload                      load test/conformance/fixtures/.../cr.yaml
        |                                                         |
  watch its CRs, judge state from the workload's OWN fields  run the CURRENT Karta library on it
        |                                                         |
  run Karta on each distinct CR, save (cr.yaml, expected)    diff current reading vs recorded expected
        |                                                         |
  test/conformance/fixtures/<operator>/<version>/<karta>/<flow>/  <----  same files
```

the state is ALWAYS judged from the workload's own status fields, never from Karta
(`classify`, recorder_test.go:69). that is deliberate: if we asked Karta "are you Running?"
and then recorded "Karta says Running", the golden test would be comparing Karta against
itself and could never catch a regression. instead the harness knows, independently, that
a Job with `active>0, ready>0` is running, and records whatever Karta happens to say.

## A2. the fixture format

`test/conformance/fixture.go`. one directory per operator, version, definition, flow:

```
test/conformance/fixtures/batch-job/v1.34.0/batch-v1-job/degraded/
  fixture.yaml            # index: schemaVersion, operator, version, kartaName, flow,
                          #        want, kartaFile, observedStates, snapshots[]
  00-Running/  cr.yaml    # the sanitized CR at this snapshot
               expected.yaml   # what Karta read: matchedStatuses, phase, components
  01-Running/  cr.yaml
               expected.yaml
  02-Degraded/ cr.yaml
               expected.yaml
```

- `Fixture` struct: fixture.go:77. `SchemaVersion = 3` (fixture.go:25) - golden refuses a
  stale-schema fixture loudly rather than silently mis-reading it.
- `KartaFile`: repo-relative path to the `docs/samples/<def>.yaml` this flow was recorded
  against (golden loads it to run the library; it is no longer pinned by sha - see the
  banner up top).
- `SnapshotDir(idx, state)` = `NN-<State>` (fixture.go:113). the index makes repeated
  states unique: `00-Running`, `01-Running`, `02-Degraded`.
- `Write` (fixture.go:121) starts with `os.RemoveAll(root)` (fixture.go:124) so a re-record
  never leaves an orphan dir from a previous, longer run.
- fixtures are YAML, marshaled through `sigs.k8s.io/yaml` (JSON tags, sorted keys), so they
  are diffable and stable key-order.

## A3. the record path, end to end

`test/e2e/predicates_test.go` + `runner_test.go` + `recorder_test.go`. one Ginkgo `Describe` per `workloadCase`,
`Ordered`, labeled by operator (A6). for each flow:

1. create the Karta definition + the workload manifest, wait for the operator to reconcile.
2. `observeTransitions` (recorder_test.go:126) watches the workload with a `RetryWatcher`
   (resumes if the API server expires the watch) and captures snapshots.
3. `assertObservedOrder` (recorder_test.go:223) checks the observed states form a valid
   progression.
4. run the Karta library on the final live object, assert `MatchedStatuses` contains the
   flow's declared `want`, assert component extraction.
5. only under `make record-e2e` (`KARTA_RECORD=1`): `writeFixture` (recorder_test.go:243)
   projects each captured CR through Karta and writes the fixture.

the harness runs the SAME `observeTransitions` in both record and non-record mode
(runner_test.go:147-382); only step 5 is gated on `recordEnabled()` (runner_test.go:177).
so a plain `make test-e2e` also verifies the ordered transition live - see B2.

## A4. the replay path (the actual unit test)

`test/conformance/golden_test.go`, `TestGolden`. walks every `fixture.yaml`, and for
each:

1. guard the schema version (re-record if it drifted).
2. load the definition, then for EACH snapshot: `Replay(karta, cr)` (replay.go:19) and
   `cmp.Diff(recorded expected, current reading)` (golden_test.go:88-96). any difference
   fails with operator/flow/state context.
3. check the snapshot dirs are `NN-<State>` in order and that collapsing their states
   reproduces `observedStates` (golden_test.go:71-80).
4. check the terminal snapshot reads as the flow's `want` (golden_test.go:101-106).

no cluster, no operators, ~1 second. this is what runs on every PR.

---

# PART B - the meaningful changes (review here)

## B1. the recorder records EVERY distinct CR, not one per state

this is the most important decision. review it first.

### what changed

```diff
--- before (one snapshot per declared state)
- seenState := map[string]bool{}
- if statusSettled(u) {
-   if state != "" && !seenState[state] {
-     seenState[state] = true
-     rec.snapshots = append(rec.snapshots, captured{state, u.DeepCopy()})
-   }
- }

+++ after (every distinct sanitized CR)
+ seenContent := map[string]bool{}
+ if statusSettled(u) && state != "" {
+   if key := sanitizedKey(u); !seenContent[key] {   // dedup by sanitized content
+     seenContent[key] = true
+     rec.snapshots = append(rec.snapshots, captured{state, u.DeepCopy()})
+   }
+ }
```

`sanitizedKey` (recorder_test.go:97) = the CR with `Sanitize` applied, marshaled to JSON
(Go sorts map keys, so it is deterministic). `seenContent` dedups on that.

### why

golden replays every recorded snapshot. if the recorder keeps only ONE `Running` snapshot,
then a workload that sits in `Running` across several CRs is under-covered: a library
change that would read a MIDDLE `Running` CR as `Degraded` is never replayed, so golden
stays green and the regression ships. the whole point of recording real data is to catch
exactly that. dedup-by-content keeps every CR whose meaningful fields differ, drops only
volatile churn (resourceVersion, timestamps - the same fields `Sanitize` strips).

### how it works now

`batch-job/degraded` records `Running, Running, Degraded` - two distinct `Running` CRs
then `Degraded`. `pod/completed` records `Initializing x4, Running x2, Completed`. golden
replays all of them. 42 flows now hold 90 snapshots (was 42). the tradeoff, accepted
deliberately: a re-record is NOT byte-reproducible (which intermediate CRs a watch delivers
is timing dependent), so a fixture set is a frozen regression baseline refreshed on operator
version bumps, not a byte-stable artifact. `assertObservedOrder` and `ObservedStates` collapse
consecutive repeats (`CollapseConsecutive`, fixture.go:93) so the order checks still work.

## B2. the harness verifies the ordered transition in BOTH modes

### what changed

```diff
--- before: record watched transitions; plain test-e2e polled only the terminal
- if recordEnabled() && tc.operator != "" {
-   rec = observeTransitions(tc, fl, obj, timeout)
- } else {
-   for i, st := range fl.states {
-     if action == nil && i != len(fl.states)-1 { continue }  // skip intermediates
-     Eventually(... st.ready(live) ...)
-   }
- }

+++ after: both modes watch the full ordered progression
+ rec := observeTransitions(tc, fl, obj, timeout)
+ assertObservedOrder(fl, rec.order)
+ ...
+ if recordEnabled() && tc.operator != "" { writeFixture(tc, fl, karta, rec) }
```

### why

before, only the recording path checked that a workload went `Initializing -> Running ->
Completed` in order; a plain `make test-e2e` just waited for the terminal and skipped the
middle states. so an out-of-order regression (or a workload that reached the terminal
without passing through the expected states) was invisible off the record path.

### how it works now

`assertObservedOrder` (recorder_test.go:223) collapses the observed states and checks they
are a monotonic subsequence of the flow's declared states, ending at the terminal. a fast
workload may skip a settled intermediate (subsequence, not equality), but it can never
regress or end on the wrong state. verified: a non-record `make test-e2e E2E_LABELS=builtin`
run passes 21/21 and writes no fixtures.

## B3. sanitize: a denylist plus a normalize list

`test/conformance/sanitize.go`. `Sanitize` (sanitize.go:88) recursively strips
`volatileKeys` (fields that differ between two recordings of the same real state:
resourceVersion, uids, timestamps, per-run pod/replicaset names in `message`, Ray job ids,
Kubeflow `replicaStatuses`, ...) and replaces `normalizeKeys` values (fields Karta reads
for PRESENCE but whose value is a per-run timestamp, e.g. a CronJob `lastScheduleTime`)
with a fixed placeholder.

### why

it is a denylist, not an allowlist: a field the library reads is never silently dropped.
the recorder additionally proves `reading(raw) == reading(sanitized)` before persisting
(recorder_test.go, inside `writeFixture`), which is what makes the denylist safe rather than
merely convenient - if stripping a field changed what Karta reads, the record fails.

### how it works now

`message` is stripped because condition messages carry live rollout text ("0 out of 1 new
replicas...") that churns; Karta reads a condition's type/status/reason, never its message.
this is also what let operator init states (NIMService `NotReady`, the Kubeflow `Created`
condition) record cleanly.

## B4. two offline unit tests: golden + the intermediate-coverage guard

### golden (existing, described in A4)
replays every snapshot with a per-snapshot `cmp.Diff`. this is the regression guard.

### the guard test (new)

```diff
+++ test/conformance/intermediate_test.go  (new)
+ func TestFixturesRecordIntermediateCRs(t *testing.T) {
+   // fails if no recorded flow has a state repeated across DISTINCT CRs
+   // (which is what a revert to one-snapshot-per-state would produce),
+   // and checks repeated same-state snapshots are genuinely distinct CRs
+ }
```

### why

B1 is only valuable if the fixtures actually contain intermediate CRs. this test locks
that in: if someone later "optimizes" the recorder back to one-per-state, the fixtures lose
their repeated states and this test fails on the PR, naming the regression. it currently
reports 8 flows capturing intermediate CRs.

## B5. testdata grouped by type

`git mv test/e2e/testdata/<type>-<flow>-workload.yaml -> testdata/<type>/<flow>.yaml`. 42
manifests, one directory per workload type: `testdata/pod/happy.yaml`,
`testdata/batch-job/degraded.yaml`, `testdata/cronjob/initializing.yaml`. paths updated in
the cases. `git mv` preserved history.

### why + how

the flat directory had 42 files with a `<type>-<flow>-workload.yaml` naming convention
doing the grouping by string prefix. subdirectories make the type grouping structural and
the flow the filename. verified: pod + batch-job re-recorded byte-identical after the move
(only the path changed, not the content).

## B6. cases split into one file per type

```diff
--- before: one cases_test.go with a 500-line var workloadCases = []workloadCase{...}
+++ after: 17 files cases_<type>_test.go, each `var <type>Case = workloadCase{...}`,
+++        and cases_test.go = the aggregation `var workloadCases = []workloadCase{ podCase, ... }`
```

### why + how

adding an operator is now a new `cases_<type>_test.go` plus one line in the aggregation.
correctness note worth a comment: the aggregation is a package-level `var`, NOT an `init()`
append. `var _ = Describe(...)` (runner_test.go) iterates `workloadCases` at package-var
init time, which runs BEFORE `init()` funcs - so an `init()` append would have left the
Describe tree empty. var aggregation initializes the per-type vars first (dependency order).

## B7. progression enrichment - showing Initializing -> Running, per type

flows were enriched to record the progression rather than jump to the terminal, using
mechanisms matched to how Karta classifies each kind:

- pod phase: an init container holds the Pod `Pending` (`phaseEq Pending -> Running`).
- Job `active`/`ready`: a readiness probe holds the pod active but `ready==0`; Karta maps
  `active>0,ready==0 -> initializing`, `active>0,ready>0 -> running`. batch-job, jobset
  (`jobsetInitializing`/`jobsetRunning`, runner_test.go:195/169).
- the `Created` condition: an init container delays the pods becoming Running so the
  operator's `Created` condition lingers (mpijob, pytorch).
- operator phase: dynamo (`state=pending` naturally, `phaseAny`, runner_test.go:71),
  grove (`availableReplicas < replicas` via `intBelow`, runner_test.go:109), nim
  (`NotReady -> Ready`).

new predicates: `phaseAny` (one Karta state from several phase strings),
`intBelow` (present-and-below, the initializing counterpart of `intAtLeast`).

### why + how

the readiness/init-container tricks make an intermediate state LINGER long enough for the
watch to observe it; with the per-CR recorder (B1) a lingering state also yields several
intermediate snapshots. some inits are not cleanly capturable and are documented in the
case comments (lws leaves `readyReplicas` absent + churns conditions; milvus etcd is too
slow on a loaded cluster; knative maps only `Ready`; raycluster has no initializing mapping).

## B8. hack/resanitize - realigning a fixture to a newer sanitize rule

```diff
+++ hack/resanitize/main.go  (new tool)
+ // load a recorded fixture, apply the CURRENT conformance.Sanitize to each cr.yaml,
+ // rewrite. preserves expected.yaml; TestGolden proves the reading is unchanged.
```

### why + how

the milvus fixture predated the `message`-strip rule, so it carried condition `message`
fields the current sanitize drops. milvus could not be live-re-recorded (its etcd/standalone
would not reach Healthy on this loaded cluster - three 12-minute timeouts). this tool applies
the current sanitize to the frozen CR in place, so milvus is consistent with the 41 fresh
flows without a cluster. it is a maintenance utility for "a sanitize rule was added, old
fixtures need realigning, live re-record not possible".

## B9. Makefile + per-operator selection

- `E2E_LABELS` forwarded to `-ginkgo.label-filter`; each case carries `Label(tc.operator)`
  plus `Label("builtin")` (runner_test.go:85). `E2E_LABELS="nim"` runs only the nim case;
  set expressions compose (`"kuberay || nim"`, `"!builtin"`). cases you did not install are
  not selected, so they never hit a 1-minute "CRD does not exist" reconcile failure.
- `record-e2e` = `test-e2e` with `KARTA_RECORD=1`, forwarding `E2E_LABELS` and `E2E_FOCUS`.

---

# PART C - what to push in the FIRST small PR

goal: land the reusable infra + ONE worked example + the offline unit tests. defer the
breadth (the other operators and the enrichment matrix) to follow-ups, so the first PR is
reviewable.

## include

| area | files | why |
|---|---|---|
| replay/fixture library | `test/conformance/{fixture,sanitize,replay}.go` | the offline engine + format |
| offline unit tests | `test/conformance/{golden_test,intermediate_test,conformance_test}.go` | the actual PR-time guard |
| record harness | `test/e2e/{suite,harness,record}_test.go` + `go.mod` | records real data, judged from workload fields |
| ONE example case | `test/e2e/cases_pod_test.go` + `test/e2e/cases_test.go` (aggregation with just podCase) | pod is built-in (no operator to install for a reviewer), and it is the richest example: `Initializing -> Running -> {Completed,Failed}` with intermediate CRs |
| that example's data | `test/e2e/testdata/pod/*.yaml` + `test/conformance/fixtures/pod/**` | the recorded fixtures the golden test replays |
| docs | `test/e2e/README.md`, `conformance/README.md` (trimmed to the shipped example) | how to record/replay, the format |
| make targets | `Makefile` `record-e2e` / `test-e2e` / `E2E_LABELS` | run it |

that is a complete vertical slice: record pod -> sanitize -> replay -> golden + guard, plus
the harness and library that make it reusable.

## why pod as the example

- built-in kind: a reviewer needs no operator install to run `make record-e2e E2E_LABELS=builtin -ginkgo.focus=Pod`.
- exercises every part: `Initializing` (init container), multiple `Running` intermediate CRs,
  three terminals (Completed/Failed/still-Pending), phase-based classification.
- proves the B1 intermediate-CR point (`pod/completed` = `Initializing x4, Running x2, Completed`).

if you would rather show the real-operator path in PR 1, `jobset` is the lightest real
operator (one small manifest, already in the provisioner) and shows `jobsetInitializing`/
`jobsetRunning`. it costs the reviewer a `jobset` install. recommend pod for PR 1, jobset
early in PR 2.

## defer to follow-up PRs

- the other 14 operators: their `cases_<type>_test.go`, `testdata/<type>/`, and
  `test/conformance/fixtures/<operator>/**`. group them (builtins; kubeflow; kuberay; the nvidia
  operators) so each follow-up PR is one coherent operator family.
- `hack/resanitize` - only needed once the milvus fixture ships; send it with milvus.
- the enrichment breadth (readiness probes, Created-condition inits) - the mechanisms live
  in the harness (shipped), but the enriched non-pod flows ship with their operators.
- the operator install machinery under `hack/e2e` is already on this branch from `4488160`;
  confirm it is on `main` or include the subset the shipped example needs.

## review checklist (things to comment on)

1. B1 tradeoff: are we OK that fixtures are a frozen baseline, not byte-reproducible?
2. B1 dedup key: dedup on sanitized content - is JSON-marshal-of-sanitized the right key?
3. B3 sanitize denylist: any field stripped that a future mapping might want to read?
4. B6 var-aggregation vs init(): fine, or prefer an explicit registry?
5. B8 resanitize: acceptable as a committed tool, or keep it out of the tree and re-record
   milvus on a fresh cluster instead?
6. schema guard (A2): the schema-version re-record-on-drift is all that is left after the
   sha was dropped - fine as is?

(resolved, see banner: the definition sha is dropped; the engine + fixtures moved under
`test/conformance`.)

---

# PART D - follow-ups already known

- milvus: live re-record on a fresh (unloaded) cluster and add the `Initializing` (Pending)
  state back; then drop the `hack/resanitize` shim for it.
- the cordon dance: statefulset-degraded needs worker2 cordoned; rayjob needs it uncordoned
  for memory. documented in the e2e progress notes; a CI shard split would remove the dance.
- Suspending / Resuming `ResourceStatus`: no bundled sample maps them, so they are unreached.
- operator version bumps (#140): the schema/sha guards already fail loudly on drift; the
  bump automation re-records and reviews the new baseline.
