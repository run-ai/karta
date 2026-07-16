<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# E2E Testing Infrastructure for Karta: Design and Plan

Status: proposal (for discussion)

This document proposes an end-to-end (e2e) testing infrastructure for Karta. It
surveys how large open-source Kubernetes projects build e2e infrastructure,
extracts the patterns that fit Karta, and lays out a concrete, phased plan with
the work items needed to implement it.

It maps onto two roadmap "Now" items: the Karta controller and its end-to-end
tests (issues #34, #67-72) and the validated, non-rotting examples (issue #79).

## Contents

- [Why Karta needs e2e](#why-karta-needs-e2e)
- [What Karta is, and what "e2e" means here](#what-karta-is-and-what-e2e-means-here)
- [Three assets: import vs CRD vs running operator](#three-assets-import-vs-crd-vs-running-operator)
- [Current state and test inventory](#current-state-and-test-inventory)
- [Survey of OSS approaches](#survey-of-oss-approaches)
- [The full menu of options](#the-full-menu-of-options)
- [Recommendation for Karta](#recommendation-for-karta)
- [What to install](#what-to-install)
- [Action items](#action-items)
- [Phased plan (narrative)](#phased-plan-narrative)
- [Open questions](#open-questions)
- [References](#references)

## Why Karta needs e2e

Karta's value is a single promise: a Karta definition correctly describes the
structure of a real workload, so that any consumer can extract pods, read
status, and update specs without per-CRD code. That promise is only credible if
it is continuously verified against the real CRDs the definitions target.

Two failure modes are unique to Karta and are not covered by the current unit
and in-memory tests:

1. Example rot. The bundled definitions in `docs/samples/` (JobSet, PyTorchJob,
   RayCluster, KServe, Knative, MPIJob, LeaderWorkerSet, Milvus, Dynamo, Grove
   PodCliqueSet, and more) target upstream CRDs that evolve. A renamed or moved
   field silently breaks a JQ path expression. Unit tests with hand-written
   fixtures cannot catch this because the fixture is also hand-written.
2. Real-object semantics. Karta operates on `unstructured` objects fetched from
   a live API server: server-side defaulting, CRD OpenAPI validation, admission
   webhooks, and status subresource behavior all shape what Karta actually sees.
   In-memory fixtures bypass all of it.

E2e closes both gaps by running Karta against real CRDs (and, where it adds
signal, real operators) on a real cluster.

## What Karta is, and what "e2e" means here

Karta today is a Go library plus a CRD (shipped as a Helm chart). There is no
controller binary yet; one is planned (roadmap "Now", #34). This shapes the e2e
scope into three distinct concerns, which should be built in this order:

- Library-level e2e. The `pkg/resource`, `pkg/jq`, and `pkg/instructions`
  packages run against real workload objects created through a real API server.
  This is the layer that exists today and benefits first.
- Definition conformance. Every `docs/samples/*.yaml` Karta is validated against
  the upstream CRD it targets, and its path expressions are exercised against a
  real instance of that workload. This is the non-rotting-examples work (#79).
- Controller e2e. Once the controller lands (#34, #67-72), reconcile behavior is
  verified against a real cluster: it observes workloads, applies the Karta
  contract, and updates status.

A note on terminology used throughout, borrowed from the wider ecosystem:

- Integration test: runs against `envtest` (a real `kube-apiserver` + `etcd`,
  no kubelet, no nodes, no pods actually scheduled). Fast, hermetic, exercises
  API semantics, CRD validation, and reconcile logic.
- E2e test: runs against a real cluster (kind), where pods are scheduled and
  real operators run. Slower, higher fidelity.

## Three assets: import vs CRD vs running operator

A common point of confusion: when a test needs to work with a third-party
workload type (say RayCluster), there are three independent assets from the
upstream project, and they are used at different layers:

1. Go types. The `RayCluster` Go struct. Pulled via a `go.mod` import, used at
   compile time only.
2. CRD schema. The CRD YAML that teaches an API server to accept and validate
   `kind: RayCluster` objects. Installed into envtest or a cluster.
3. Running operator. The KubeRay controller pod that actually reconciles a
   RayCluster into pods and fills in its status. Only meaningful on a real
   cluster.

These are decoupled: the CRD can be installed without running the operator, and
the Go types can be imported without either.

Karta is special here, and it collapses the list. Because Karta is
schema-agnostic by design (it operates on `unstructured` objects via JQ paths,
never `rayv1.RayCluster{}`), Karta does not need to import the Go types at all.
That is the opposite of a project like Kueue or the KAI scheduler, whose
controllers reference the typed structs directly. For Karta the assets reduce
to:

- CRD schema: needed, so a test cluster accepts a sample workload and validates
  it. This is the main thing Karta installs.
- Running operator: needed only to assert against operator-produced state (status
  conditions it fills in, child resources it creates). For most Karta checks
  (extract pods, replicas, spec fields, status mappings) a crafted object is
  enough and the operator can be skipped.

The practical consequence: Karta's test install list is dominated by CRD
schemas, not operators, and Karta does not need to actually schedule pods (so no
GPU operator or scheduling machinery) for the large majority of its checks.

## Current state and test inventory

All current tests run in-process: no API server, no cluster. Two styles exist,
both worth keeping: pure unit tests (some using generated mocks) and an
in-memory "blackbox" suite that wires real components together over a fake
object.

What exists today (roughly 400 specs):

| Layer | Location | Covers | Fixtures |
|---|---|---|---|
| JQ engine | `pkg/jq`, `pkg/jq/execution` | path evaluation, validation | none |
| Core extraction/update | `pkg/resource` (accessor, component, pod_querier, factory) | extract pods, replicas, status; update specs | hand-built objects + real JQ runner; some `MockRunner` |
| Instructions | `pkg/instructions` | gang scheduling, summary | hand-built |
| Karta CRD validation | `pkg/api/runai/v1alpha1/validation_test.go` | Karta struct rules | none |
| Sample sanity | `pkg/api/runai/v1alpha1/examples_test.go` | each `docs/samples/*.yaml` is a structurally valid Karta | the samples |
| Blackbox integration | `test/blackbox/` | real components wired together (suspend/resume) | fake in-memory object |

CI: a single `ci.yaml` job on `ubuntu-latest` runs `make check` (generate +
validate + unit tests), `golangci-lint`, and helm lint/validate. No cluster is
provisioned. The Makefile has `install-crd` / `uninstall-crd` (plain
`kubectl apply`) but no `setup-envtest`, no kind targets, no e2e target.

The headline gap. `examples_test.go` is the only test that touches the samples,
and it only checks that each sample is a structurally valid Karta (correct
apiVersion/kind, passes Karta's own validator). It does not check the sample
against the upstream CRD it targets: it never verifies that, for example,
`.spec.workerGroupSpecs[].replicas` actually exists on a real RayCluster. So
example rot is entirely untested. The other gaps follow from there: nothing runs
against a real API server (no server-side defaulting, CRD validation, or status
subresource behavior), and nothing exercises real operators. The bundled
`docs/samples/` set targets 12 distinct upstream CRDs (plus the built-in Batch
Job) from different operators, all currently unverified against their real
schemas.

## Survey of OSS approaches

Four reference points, chosen because each isolates a pattern Karta needs.

### Kueue (the closest analog)

Kueue is the strongest model for Karta because it has the same core problem:
it must understand many third-party workload CRDs from different operators
(JobSet, Kubeflow training/trainer, KubeRay, MPI, LeaderWorkerSet, AppWrapper).
Its defining technique is a single source of truth for every third-party
dependency: `go.mod`.

- Every operator is a real `require` in `go.mod` (the controller imports its Go
  types anyway). Example: `sigs.k8s.io/jobset`, `github.com/ray-project/kuberay/ray-operator`,
  `github.com/kubeflow/training-operator`.
- The Makefile derives both the version pin and the on-disk CRD path from
  `go.mod`, so there is exactly one place to update:
  `JOBSET_VERSION = $(shell go list -m -f "{{.Version}}" sigs.k8s.io/jobset)` and
  `JOBSET_ROOT = $(shell go list -m -mod=readonly -f "{{.Dir}}" sigs.k8s.io/jobset)`.
- Integration tier: per-operator targets copy the CRD YAMLs out of the resolved
  module cache into a `dep-crds/` directory, which is fed to `envtest` via
  `CRDDirectoryPaths`. Third-party Go types are registered into the test scheme.
- E2e tier: `hack/testing/e2e-common.sh` installs the same pinned versions as
  running operators into kind, via a mix of release-URL manifests, `kubectl -k`
  kustomize, and Helm, all keyed on the same `*_VERSION` variables.
- Rot prevention: Dependabot bumps the single `go.mod` source weekly; a
  `DEPENDENCY_LIFECYCLE.md` documents the policy. A schema drift fails CI in the
  exact bumping PR.
- Test selection: structured Ginkgo labels (`job:pytorch`, `job:ray`,
  `feature:tas`) let CI run an "only Ray" slice. Integration packages are
  sharded round-robin across parallel CI jobs (`shard-integration-tests.sh`).
- CI: a Kubernetes-version matrix (e.g. 1.34 / 1.35 / 1.36).

The single most transferable idea: derive third-party CRD versions and paths
from `go.mod`, so version pins cannot rot silently.

### KAI Scheduler (in-org reference; the install-but-skip pattern)

The KAI scheduler (NVIDIA's open-source Kubernetes AI scheduler) is a useful
in-ecosystem reference. It does both import and install, decoupled:

- It imports the workload operator Go types in `go.mod` (KubeRay, Kubeflow
  training, MPI, JobSet, LWS, Knative) because its specs construct typed objects.
- It installs the real running operators into kind only behind an opt-in flag
  (`--test-third-party-integrations`), via a mix of Helm, remote kustomize, and
  release manifests, with versions pinned inline in per-operator shell scripts
  under `hack/third_party_integrations/`.
- Tests self-skip when a CRD is absent (a `SkipIfRayNotInstalled`-style helper).
- Critically, the PR CI does not pass that flag, so the third-party integration
  specs are skipped on PRs; real-operator coverage is a local/manual or
  out-of-band concern.

Two lessons for Karta. First, the install-but-skip tiering is sound: a cheap
default that needs no operators, with real-operator runs as a heavier opt-in.
Karta should adopt it for the kind tier. Second, two anti-patterns to avoid:
KAI's version pins live scattered across shell scripts (Kueue's go.mod-derived
single source is better), and "skip if not installed" means a broken integration
contributes zero CI signal (it silently skips). For Karta's non-rotting goal the
opposite is required: a missing or changed CRD must fail, not skip, which is why
the always-on offline validation tier (below) matters.

### Volcano (the counter-example)

Volcano is a scheduler that consumes plain pods/podgroups, so it does not need
foreign CRD schemas at compile time and largely sidesteps the multi-operator
problem. Its e2e is real-kind-only (no envtest tier), with suites split by topic
and a GitHub Actions fan-out (one workflow per suite, built on a single shared
image artifact). It uses KWOK to fake large/topology clusters cheaply.

The lesson for Karta is the inverse of Volcano: because Karta must understand
foreign CRD schemas (it inspects and mutates arbitrary workload types), the
Volcano "ignore the schemas" model does not apply. Karta is a Kueue-shaped
problem, not a Volcano-shaped one. (KWOK remains a useful option for scaling
pod-heavy scenarios without real compute.)

### cert-manager and the kubebuilder scaffold (the baseline mechanics)

These give the standard two-tier mechanics every controller-runtime project
uses:

- `envtest` for controller/integration tests: `setup-envtest` downloads and
  version-manages the `kube-apiserver` + `etcd` + `kubectl` binaries into
  `bin/k8s/<ver>`. CRDs are loaded from disk via `CRDDirectoryPaths`.
  Limitations to design around: no pods are scheduled, no garbage collection
  (owner references do not cascade), namespaces never finish deleting (use fresh
  namespace names rather than relying on cleanup).
- kind for true e2e: a `setup-test-e2e` target creates the cluster idempotently,
  the suite loads the controller image via `kind load docker-image`, and a
  `cleanup-test-e2e` target tears it down. On failure, an `AfterEach` dumps
  controller logs, events, and `kubectl describe`.
- cert-manager adds the production-grade extras: a pinned kind node image per
  k8s version, an addon stack installed via Helm, images pre-loaded as tarballs
  (`crane` + `kind load image-archive`) to avoid in-test registry pulls, Ginkgo
  parallelism (`--procs`, dropped to 1 when focused), `--flake-attempts`, JUnit
  output to `$ARTIFACTS`, and a Kubernetes-version matrix in CI.
- Framework: Ginkgo v2 + Gomega everywhere, `Eventually`/`Consistently` for
  polling, `Ordered` containers with `BeforeAll`/`AfterAll`. Karta already uses
  Ginkgo + Gomega, so this is a natural fit.

### Cluster API and Gateway API (framework reuse and conformance)

Two forward-looking ideas for later phases:

- Cluster API ships `test/framework`, a reusable Go library that downstream
  provider repos import to re-run a shared suite against their own
  infrastructure. Its best practices apply directly: write specs that mirror
  real user workflows; never hardcode wait intervals (look them up from config);
  prefer typed `WaitFor...` helpers over scattered `Eventually`; collect
  artifacts on failure.
- Gateway API is the conformance-suite model: the project owns a contract, and
  independent implementations prove they satisfy it by running one shared,
  versioned `go test` suite (runnable as a CLI or vendored as a library),
  declaring supported features (Core, mandatory; Extended, opt-in), and
  publishing a reproducible report. This maps cleanly onto Karta's "one CRD,
  many consumers" shape and is the natural endpoint for definition conformance:
  a Karta definition (or a Karta-aware tool) is "conformant" if it passes the
  shared suite for the features it claims.

For keeping examples non-rotting, the documented tooling is:

- `kubeconform -strict` for fast offline validation of YAML against JSON
  schemas generated from the CRDs (via `openapi2jsonschema` / `controller-gen`);
  `-strict` is what catches a renamed or removed field.
- `kubectl apply --dry-run=server` against a cluster with the pinned upstream
  CRDs installed, for full-fidelity validation including CEL
  `x-kubernetes-validations` and webhooks (CRD must be installed first).
- Pin the upstream CRD version, regenerate schemas from the pinned CRDs, and run
  a re-validation job both on PRs and on a schedule so a new upstream version
  surfaces as a failing scheduled run rather than silent drift.

## The full menu of options

Laid out so the recommendation is a choice among explicit alternatives.

Cluster substrate:

- `envtest` (apiserver + etcd, no nodes). Fast, hermetic. Cannot schedule pods
  or run real operators. Best for API-semantics and controller-logic coverage.
- kind (Kubernetes in Docker). Real nodes, real pods, real operators. The
  default e2e substrate across the ecosystem.
- k3d / k3s. Lighter than kind; less common in this ecosystem; fewer pinned
  node-image guarantees.
- KWOK (fake nodes). Schedules pods without real compute. Useful to scale
  pod-heavy scenarios cheaply; pairs with kind.
- Real/managed clusters (EKS/GKE). Highest fidelity, highest cost; in
  cert-manager and CAPI these run as non-blocking periodics, not PR gates.

How to supply the third-party CRDs the definitions target:

- `go.mod` as source of truth (Kueue). Add each operator as a require, derive
  version and CRD path with `go list -m`. Best rot protection; couples the test
  matrix to Go-importable modules.
- Vendored CRD YAML at a pinned tag. Simple, no Go dependency, but a second
  place to bump and easier to forget.
- Remote schema catalog (Datree CRDs-catalog, FluxCD schemas) pinned by version,
  for offline `kubeconform` validation. Good for the lightweight tier.
- Install full operators via Helm / release manifests. Highest fidelity (real
  defaulting, real status), heaviest setup.

Definition validation strength (cheapest to strongest):

- `kubeconform -strict` against generated JSON schemas (offline, seconds).
- `kubectl apply --dry-run=server` against pinned CRDs on a cluster (catches
  CEL/webhook rules).
- Create a real workload instance and run Karta's extract/update/status against
  it (catches path-expression errors end to end).
- Install the real operator and assert the workload reaches a real running
  state, then run Karta against it (highest fidelity, slowest).

Test framework and selection:

- Ginkgo v2 + Gomega (already in use). Structured labels for per-integration
  slices (`workload:ray`); `Ordered`/`BeforeAll`; `Eventually` polling.
- Standard `go test` + `sigs.k8s.io/e2e-framework` (Crossplane core). Viable but
  diverges from the existing Ginkgo investment.
- Declarative manifest-driven (chainsaw/kuttl, as in Crossplane uptest). Good
  for example-lifecycle tests with little Go; weaker for library assertions.

CI shape:

- Single Kubernetes version vs a version matrix.
- PR-blocking smoke slice plus heavier nightly/periodic full matrix (the
  cert-manager and CAPI model).
- Parallelism by Ginkgo `--procs`, by package sharding, or by one CI job per
  workload (the Volcano fan-out).

## Recommendation for Karta

Adopt a Kueue-shaped, three-tier design, built on the Ginkgo + Gomega stack
already in the repo, and grow toward a Gateway-API-style conformance suite as
the contract stabilizes.

Tier 1, integration (`envtest`). Run `pkg/resource`, `pkg/jq`, and
`pkg/instructions` against a real API server. Install the Karta CRD plus the
pinned upstream CRDs from `CRDDirectoryPaths`, create real workload objects, and
assert Karta's extract/update/status behavior. This is fast, hermetic, and the
first thing to land. (Pods are not scheduled here; that is fine for the library,
which reads and mutates specs, not running pods.)

Tier 2, definition conformance (the #79 work). For every `docs/samples/*.yaml`:

1. Offline gate on every PR: `kubeconform -strict` against JSON schemas
   generated from the pinned upstream CRDs. Fast, catches renamed/removed fields.
2. Cluster gate: in `envtest` (or kind), `kubectl apply --dry-run=server` of a
   sample workload instance against the pinned CRD, then run the matching Karta
   definition's path expressions against that instance and assert the expected
   pods/replicas/status are extracted.
3. Scheduled re-validation against the latest upstream CRD versions, so drift
   shows up as a failing nightly run, not a silent break, and triggers a bump.

Tier 3, controller e2e (lands with #34/#67-72). On kind, install the controller
plus a representative subset of real operators, create workloads, and assert the
controller reconciles them per the Karta contract. Keep a PR-blocking smoke
slice small; run the full operator matrix nightly.

Dependency management (the load-bearing decision). Use `go.mod` as the single
source of truth, exactly as Kueue does:

- Add each targeted operator module to `go.mod` (most are Go-importable;
  Knative/KServe/Milvus may need vendored CRD YAML at a pinned tag where a clean
  Go module is not available, an explicitly documented exception).
- Derive `*_VERSION` and `*_ROOT` in the Makefile with `go list -m`; copy CRDs
  into a generated `dep-crds/` (or `test/crds/`) directory for `envtest` and for
  schema generation. Never hand-maintain the version in two places.
- Let Dependabot bump `go.mod`; document the policy in a short
  `DEPENDENCY_LIFECYCLE.md`-style section.

Test selection and CI:

- Tag specs with structured Ginkgo labels per workload (`workload:jobset`,
  `workload:ray`, ...) so CI can run a single-integration slice and contributors
  can run just the one they touched.
- CI: a fast PR job (unit + integration + offline `kubeconform`) that gates
  merges, and a heavier scheduled job (kind, real operators, dry-run-server,
  upstream-latest re-validation) that catches rot and runs the full matrix.
  Adopt a Kubernetes-version matrix once Tier 3 exists.

Conformance suite (later, tracks the contract stabilizing toward v1beta1).
Package the Tier 2 definition checks as a reusable, versioned `go test` suite
runnable as a CLI and vendorable as a library, with Core/Extended feature tiers
and a reproducible report, following the Gateway API model. This is how external
Karta authors and Karta-aware tools self-certify a definition.

Why this and not the alternatives: Karta must understand foreign CRD schemas, so
the Volcano "ignore schemas" model does not fit; the Kueue `go.mod` model is the
only surveyed approach that makes version rot fail loudly. Ginkgo is already in
the tree, so the cert-manager/kubebuilder mechanics drop in with no framework
churn. The conformance-suite endpoint matches Karta's "one contract, many
consumers" identity better than any per-project e2e suite.

## What to install

Concrete answer to "which dependencies go into the test environment", grounded
in the actual `docs/samples/` set. The governing principle (see
[Three assets](#three-assets-import-vs-crd-vs-running-operator)): Karta needs CRD
schemas almost always and running operators almost never, because it reads and
mutates specs/status rather than requiring pods to actually run.

Always, every tier:

- Karta's own CRD (`charts/karta/crds`).
- `setup-envtest` binaries (apiserver + etcd) as a test dependency.

CRD schema vs running operator, per sample:

| Sample | Upstream | CRD schema (integration / conformance) | Run real operator (kind e2e)? |
|---|---|---|---|
| `batch-job` | core `batch/v1` Job | built-in (nothing to install) | n/a (built-in) |
| `jobset` | sigs.k8s.io/jobset | yes | yes (small, single controller) |
| `lws` | sigs.k8s.io/lws | yes | yes (small) |
| `pytorch` | Kubeflow training-operator | yes | yes (moderate) |
| `mpijob` | Kubeflow mpi-operator | yes | yes (moderate) |
| `raycluster` / `rayjob` | KubeRay | yes | yes (one operator covers both) |
| `knative-serving` | Knative Serving | yes | no, CRD only (needs networking layer, activator, autoscaler) |
| `kserve` | KServe | yes | no, CRD only (pulls Knative + cert-manager + Istio) |
| `nimservice` | NVIDIA NIM operator | yes | no, CRD only (needs NGC creds + GPUs) |
| `milvus` | Milvus operator | yes | no, CRD only (heavy stack) |
| `dynamo` | NVIDIA Dynamo | yes | no, CRD only (heavy; depends on Grove) |
| `grove-podcliqueset` | grove.io | yes | no, CRD only |

So the install footprint is about 12 CRD schemas (cheap, always-on, catches
rot) but only about 5 running operators (expensive, nightly, full fidelity), and
zero GPU/scheduling infrastructure to start.

Infrastructure dependencies install only when a feature needs them:

- cert-manager: only if/when the Karta controller has admission webhooks (also a
  transitive dependency of real KServe). Skip until then.
- Prometheus operator CRDs: only when Karta-exposed metrics are tested.
- fake-GPU operator / KWOK: only if a scenario needs pods to actually schedule.
  Most Karta checks do not. Default: skip.

## Action items

Ordered, each sized to be its own PR, with the files it touches and a
done-when. IDs are referenced by the [narrative plan](#phased-plan-narrative).

Phase 0, envtest harness (foundation):

- [ ] A1. Add envtest tooling to the Makefile: a `setup-envtest` install target
  (apiserver + etcd + kubectl into `bin/k8s/`), `ENVTEST_K8S_VERSION` derived
  from `k8s.io/api` in `go.mod`, and a `test-integration` target. Done when
  `make test-integration` runs against an empty suite.
- [ ] A2. Create a `test/integration/` Ginkgo suite that starts
  `envtest.Environment`, installs the Karta CRD from `charts/karta/crds`, and
  exposes a real client. Done when the suite boots an API server and applies the
  CRD in `BeforeSuite`.
- [ ] A3. Port the `test/blackbox` suspend/resume scenario onto envtest, running
  against a real object created through the API server. Keep the in-memory
  version. Done when suspend/resume passes against a live API server.
- [ ] A4. Add an integration job to `.github/workflows/ci.yaml` running
  `make test-integration`. Done when CI runs envtest on every PR.

Phase 1, pinned upstream CRDs (single source of truth):

- [ ] B1. Decide and document the pinning mechanism: go.mod-derived (Kueue
  style) vs vendored `test/crds/` at pinned tags. Done when the decision is
  recorded and the corresponding open question is closed.
- [ ] B2. Bring in the 12 upstream CRD schemas at pinned versions into a
  generated `dep-crds/` (or `test/crds/vendored/`), plus a Makefile `dep-crds`
  target to (re)populate it. Done when `make dep-crds` produces all CRD YAMLs.
- [ ] B3. Feed `dep-crds` into the integration suite `CRDDirectoryPaths`; enable
  Dependabot/Renovate on the module/version group. Done when envtest can create
  a real RayCluster object and a version bump is a one-line change.

Phase 2, non-rotting samples (the #79 win, highest value):

- [ ] C1. Generate JSON schemas from the pinned CRDs (`openapi2jsonschema` or
  `controller-gen` output) via a `samples-schemas` target. Done when schemas are
  produced.
- [ ] C2. Add a `validate-samples` offline gate (`kubeconform -strict` over
  `docs/samples/`) wired into PR CI. Done when a renamed/removed upstream field
  turns CI red.
- [ ] C3. Upgrade `examples_test.go` from a self-check to a real-CRD check:
  validate a representative workload instance against the upstream CRD via
  `--dry-run=server` in envtest, keeping the existing Karta-validity check. Done
  when each sample is proven valid against the real CRD it targets.
- [ ] C4. Add per-sample path-resolution specs (fixture workload + assertions on
  extracted pods/replicas/status), tagged `workload:<type>`. Done when every
  sample has an isolated, label-runnable path test.
- [ ] C5. Add a scheduled CI job that re-pulls latest upstream CRDs and re-runs
  C2/C3, flagging drift without gating PRs. Done when drift surfaces as a failing
  nightly.

Phase 3, kind e2e with real operators (later, nightly):

- [ ] D1. Add a kind config (pinned node image per k8s version) and
  `setup-test-e2e` / `cleanup-test-e2e` / `test-e2e` targets.
- [ ] D2. Install the lightweight operator subset (JobSet, LWS, Kubeflow
  training, Kubeflow MPI, KubeRay) keyed on the same pinned versions; pre-load
  images.
- [ ] D3. Add live-workload e2e specs (create workload, `Eventually` wait for
  real status, run Karta against the live object), skipping cleanly if an
  operator is absent while keeping C2/C3 as the always-on rot gate.
- [ ] D4. Dump logs/events/describe as CI artifacts on failure.

Phase 4, controller e2e (with #34 / #67-72):

- [ ] E1. Build and `kind load` the controller image; install via the Helm
  chart in e2e.
- [ ] E2. Add reconcile specs (create workload, controller observes, applies the
  contract, updates Karta status) and a controller-upgrade spec.
- [ ] E3. Add a Kubernetes-version matrix to the nightly job.

Phase 5, conformance suite (tracks v1beta1):

- [ ] F1. Extract C3/C4 into a reusable, versioned `go test` suite (CLI plus
  vendorable library).
- [ ] F2. Define Core (mandatory) and Extended (opt-in) feature tiers; emit a
  reproducible report.
- [ ] F3. Document how a third party validates a definition and submits a
  report.

Suggested first slice (biggest safety per unit of effort): A1 + A2 + A3
(envtest harness with one real test), then B2 + B3 (pinned CRDs in envtest),
then C2 + C3 (samples validated against real CRDs). About six small PRs that
move the project from zero real-API coverage and zero rot protection to every
sample provably matching its real CRD on every PR.

## Phased plan (narrative)

Each phase is independently shippable and ordered by dependency and value. The
checklist above is the actionable form; this section gives the intent behind
each phase.

### Phase 0: integration harness (envtest)

Goal: a real API server in tests, no third-party operators yet.

- Add `setup-envtest` to the Makefile (download/version-manage apiserver + etcd +
  kubectl into `bin/`); derive `ENVTEST_K8S_VERSION` from `k8s.io/api` in
  `go.mod`.
- Add a `test-integration` target: `KUBEBUILDER_ASSETS=... go test` over the
  integration packages.
- Add an integration suite that starts `envtest`, installs the Karta CRD from
  `charts/karta/crds`, and ports one existing blackbox scenario
  (suspend/resume) to run against the real API server with a real
  `unstructured` object.
- Wire it into a new CI job (or extend `ci.yaml`).

Exit: `make test-integration` passes locally and in CI; one scenario runs
against a real API server.

### Phase 1: pinned upstream CRDs from go.mod

Goal: the single-source-of-truth dependency mechanism.

- Add the targeted operator modules to `go.mod`; for any without a clean Go
  module, vendor the CRD YAML at a pinned tag under `test/crds/vendored/` with a
  comment recording the source tag.
- Add Makefile machinery: `*_VERSION = $(shell go list -m ...)`,
  `*_ROOT = $(shell go list -m -mod=readonly ...)`, and a `dep-crds` target that
  copies each operator's CRD YAML into a generated directory.
- Feed `dep-crds` into the integration suite's `CRDDirectoryPaths`.
- Add a short dependency-lifecycle note and enable Dependabot for the module
  group.

Exit: integration tests run with real upstream CRDs installed; bumping a version
is a one-line `go.mod` change.

### Phase 2: definition conformance and non-rotting examples (#79)

Goal: every `docs/samples/*.yaml` is continuously validated.

- Generate JSON schemas from the pinned CRDs (`openapi2jsonschema` or
  `controller-gen` output) and add a `validate-samples` target running
  `kubeconform -strict` over `docs/samples/`.
- Add per-sample fixture workloads and integration specs (labeled
  `workload:<type>`) that apply the workload with `--dry-run=server` and then run
  the matching Karta definition's path expressions, asserting extracted
  pods/replicas/status.
- Add a scheduled CI job that re-pulls the latest upstream CRDs and re-validates,
  flagging drift.

Exit: a renamed upstream field fails CI; samples are provably correct against
pinned CRDs.

### Phase 3: kind-based e2e with real operators

Goal: high-fidelity behavior against running operators.

- Add a kind config and `setup-test-e2e` / `cleanup-test-e2e` targets; pin the
  kind node image per k8s version.
- Install a representative subset of real operators (keyed on the same
  `*_VERSION` vars) via Helm/release manifests; pre-load images to avoid
  in-test pulls.
- Add e2e specs that create a real workload, wait for it to run
  (`Eventually`/typed `WaitFor` helpers), and run Karta against the live object.
- On failure, dump logs/events/describe as artifacts. Keep a small PR-blocking
  smoke slice; run the full operator matrix nightly.

Exit: `make test-e2e` brings up kind, installs operators, and runs Karta against
real workloads.

### Phase 4: controller e2e (with #34 / #67-72)

Goal: verify the controller reconcile loop end to end.

- Once the controller binary exists, build and `kind load` its image; install via
  the Helm chart in e2e.
- Add reconcile specs: create a workload, assert the controller observes it,
  applies the contract, and updates Karta status. Add a controller-upgrade spec.
- Add the Kubernetes-version matrix to the nightly job.

Exit: controller behavior is covered on a real cluster across supported k8s
versions.

### Phase 5: conformance suite (tracks v1beta1)

Goal: external self-certification of Karta definitions and Karta-aware tools.

- Extract the Phase 2/3 checks into a reusable, versioned `go test` suite
  runnable as a CLI and vendorable as a library.
- Define Core (mandatory) and Extended (opt-in) feature tiers; emit a
  reproducible conformance report.
- Document how a third party validates a new definition and submits a report.

Exit: anyone can run the Karta conformance suite against a definition and publish
a result.

## Open questions

- Which operators get real-install coverage (Phase 3) vs CRD-only validation
  (Phases 1-2)? Some (Knative, KServe, Dynamo/Grove) pull heavy dependency
  stacks; CRD-only validation may be the right cost/benefit for those.
- For operators without a clean importable Go module, is vendored pinned CRD
  YAML acceptable, or should those definitions be validated only against a
  remote pinned schema catalog?
- Where should the e2e/integration code live: a new `test/integration/` and
  `test/e2e/` split (Kueue layout), or grow the existing `test/blackbox/`?
- Should CI move to a PR-blocking smoke slice plus nightly full matrix now, or
  keep everything PR-blocking until runtimes become a problem?
- Is the conformance suite (Phase 5) in scope before v1beta1, or strictly after
  the API stabilizes?

## References

OSS projects surveyed:

- Kueue: `Makefile-deps.mk`, `Makefile-test.mk`, `test/integration/framework/framework.go`,
  `hack/testing/e2e-common.sh`, `hack/testing/shard-integration-tests.sh`,
  `DEPENDENCY_LIFECYCLE.md` (github.com/kubernetes-sigs/kueue).
- KAI Scheduler: `hack/setup-e2e-cluster.sh`,
  `hack/third_party_integrations/deploy_*.sh`, `go.mod`,
  `.github/workflows/on-pr.yaml` (github.com/NVIDIA/KAI-Scheduler).
- Volcano: `hack/run-e2e-kind.sh`, `.github/workflows/e2e.yaml`
  (github.com/volcano-sh/volcano).
- cert-manager: `make/e2e-setup.mk`, `make/cluster.sh`, `make/config/kind/cluster.yaml`
  (github.com/cert-manager/cert-manager); E2e docs at cert-manager.io/docs/contributing/e2e.
- kubebuilder scaffold + controller-runtime `envtest`: book.kubebuilder.io
  (envtest reference, writing-tests), `setup-envtest` tool.
- Cluster API: `test/framework`, `test/e2e` (github.com/kubernetes-sigs/cluster-api);
  cluster-api.sigs.k8s.io/developer/core/e2e.
- Gateway API conformance: `conformance/` and GEP-1709
  (github.com/kubernetes-sigs/gateway-api; gateway-api.sigs.k8s.io).

Tools referenced:

- `sigs.k8s.io/controller-runtime/tools/setup-envtest`
- `kind` (kubernetes-sigs/kind), `KWOK` (kubernetes-sigs/kwok)
- `kubeconform` (github.com/yannh/kubeconform), `openapi2jsonschema`
- `kubectl apply --dry-run=server`
- Ginkgo v2 + Gomega (already used in this repo)
