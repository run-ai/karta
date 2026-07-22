<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# Built-in Karta Catalog - Detailed Design

Design for the built-in Karta defaults. Resolves issue #117; implements the catalog and
resolver API from epic #86 and the Go-as-source-of-truth breakdown in #118.

## Background

A key value Karta delivers is letting platforms and controllers support any workload type
without per-CRD adapters. Karta already ships 20 curated definitions as hand-written YAML
(PyTorchJob, RayCluster, RayJob, JobSet, LeaderWorkerSet, MPIJob, KServe, Knative Serving,
NIM Service, Milvus, Dynamo, Grove PodCliqueSet, batch Job). Today they are hand-written
YAML with two gaps:

- No compile-time checking. A wrong field path, a wrong GVK, or drift from the `Karta` API
  type (`pkg/api/runai/v1alpha1/types.go`, `pkg/api/runai/v1alpha1/structure.go`) is only
  caught at runtime, if at all.
- Not usable by Go callers at runtime. A platform integrating Karta cannot resolve a
  built-in definition for a workload GVK without shipping YAML and parsing it itself.

The goal is to make typed Go the single source of truth in a new `pkg/catalog/` package,
expose a resolver API keyed by workload GVK, and generate the YAML catalog from the Go
definitions with a CI drift check. This makes "Karta supports any workload" true on day
zero, with no cluster access.

## Settled decisions

- Generated YAML catalog lives in a new `docs/catalog/` directory. It replaces the
  hand-written `docs/samples/` set, which is removed: the generated catalog becomes the one
  place the YAML definitions live.
- Typed Go constructors live in a dedicated `pkg/catalog/kartas/` subpackage; the catalog
  code (`Catalog`, `New`, `Get`, `List`, the wiring list) lives in `pkg/catalog/catalog.go`.
  This keeps the many struct-literal definition files separate from the small amount of
  catalog logic.
- Filenames use the GVK slug `{group-slug}-{kind-lowercase}-{version}.yaml`
  (for example `ray-io-raycluster-v1.yaml`). The slug is derived from the root component's
  `kind`, so the generator needs no separate name field.
- The generator is a pure-Go program under `hack/gen-samples/`. No shell, cross-platform.
- The catalog is immutable. `catalog` builds a `Catalog` object holding the fixed list of
  Kartas from the Go definitions. There is no runtime registration: the only operations
  are `Get` and `List`. Callers that need their own internal workload types compose their
  own lookup alongside `catalog.List()` rather than mutating the catalog.
- The catalog carries no version of its own. It is coupled to the Karta API version
  (`run.ai/v1alpha1`): the definitions are typed against that API, so the catalog moves with
  it. A separate per-catalog version was considered and decided against for now.
- Code name for the concept is "catalog"; the public API verbs are `Get` and `List`.

## Design

### 1. The `pkg/catalog/` package

The typed definitions live in a `pkg/catalog/kartas/` subpackage, one file per workload,
each exposing a single exported constructor that returns a typed `*v1alpha1.Karta`. The
jq-expression notes currently living in the hand-written YAML comments move into Go comments
on the relevant fields as each definition is ported. The catalog logic lives one level up in
`pkg/catalog/catalog.go`, keeping the many struct-literal files separate from the small
catalog surface.

Wiring a workload into the catalog is a single explicit list in `catalog.go`, so it is one
reviewable diff rather than a line buried in each file. A completeness test (see section 5)
makes a forgotten entry fail CI, so this list cannot silently drift from the set of defined
constructors.

Representative definition, `pkg/catalog/kartas/jobset.go`:

```go
package kartas

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    v1alpha1 "github.com/.../pkg/api/runai/v1alpha1"
)

func Jobset() *v1alpha1.Karta {
    return &v1alpha1.Karta{
        TypeMeta:   metav1.TypeMeta{APIVersion: "run.ai/v1alpha1", Kind: "Karta"},
        ObjectMeta: metav1.ObjectMeta{Name: "jobset-x-k8s-io-v1alpha2-jobset"},
        Spec: v1alpha1.KartaSpec{StructureDefinition: v1alpha1.StructureDefinition{
            RootComponent: v1alpha1.ComponentDefinition{
                Name: "jobset",
                Kind: &v1alpha1.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
                // ... suspendDefinition, statusDefinition
            },
            // ... childComponents
        }},
    }
}
```

The catalog and resolver, `pkg/catalog/catalog.go`:

```go
package catalog

import (
    "fmt"

    "k8s.io/apimachinery/pkg/runtime/schema"
    v1alpha1 "github.com/.../pkg/api/runai/v1alpha1"
    "github.com/.../pkg/catalog/kartas"
)

// Catalog is an immutable set of built-in Kartas indexed by their root component GVK.
// It is built once from the Go definitions and never mutated, so it needs no locking.
type Catalog struct {
    byGVK map[schema.GroupVersionKind]*v1alpha1.Karta
}

var ErrNotFound = fmt.Errorf("builtin: no Karta registered for GVK")

// definitions is the single wiring point for the catalog. Adding a workload means adding
// its file plus one line here. The completeness test asserts this list matches the set of
// constructors defined in the package, so a forgotten entry fails CI.
var definitions = []func() *v1alpha1.Karta{
    kartas.Jobset, kartas.Pytorch, kartas.Raycluster, kartas.Rayjob, kartas.Mpijob,
    kartas.LWS, kartas.KServe, kartas.KnativeServing, kartas.NIMService, kartas.Milvus,
    kartas.Dynamo, kartas.GrovePodCliqueSet, kartas.BatchJob,
}

// New builds the catalog from the definitions list. It panics on a definition with no root
// GVK or on a duplicate GVK, so two definitions cannot silently shadow each other. Because
// definitions is a compile-time constant, any such panic surfaces immediately in tests.
func New() *Catalog { /* call each def, key by root GVK, detect duplicates, sort */ }

// RootKey derives the workload identity from the root component's kind.
func RootKey(k *v1alpha1.Karta) schema.GroupVersionKind { /* convert root Kind (group may be empty) */ }

func (c *Catalog) Get(gvk schema.GroupVersionKind) (*v1alpha1.Karta, error) { /* lookup, ErrNotFound */ }
func (c *Catalog) List() []*v1alpha1.Karta { /* sort byGVK keys, return deep copies */ }

// defaultCatalog is the package-global instance callers use directly.
var defaultCatalog = New()

func Get(gvk schema.GroupVersionKind) (*v1alpha1.Karta, error) { return defaultCatalog.Get(gvk) }
func List() []*v1alpha1.Karta                                  { return defaultCatalog.List() }
```

Design notes:

- The key type is the standard `schema.GroupVersionKind`, not the local
  `v1alpha1.GroupVersionKind`. Callers already have `unstructured.GroupVersionKind()`, so
  cluster-first fallback composes cleanly. `RootKey()` converts the local root `Kind` to the
  schema type internally.
- `Get` and `List` return deep copies of the stored definitions, so a caller cannot mutate
  the immutable package-global catalog and affect later resolutions. The `Karta` type already
  has generated DeepCopy.
- Duplicate GVKs are detected at construction time in `New()`, not at runtime, since the
  definition set is fixed at compile time.
- Validation via the existing `NewKartaValidator(k).Validate()`
  (`pkg/api/runai/v1alpha1/validation.go`) runs in tests over `List()`, keeping the catalog
  build itself cheap.
- There is no `Add`. The catalog is a read-only building block; callers needing their own
  internal workload types keep a separate lookup and fall through to `catalog.Get`.

Cluster-first composition stays on the caller side; the package is the building block:

```go
karta, err := clusterClient.GetKarta(ctx, name)
if apierrors.IsNotFound(err) {
    karta, err = catalog.Get(obj.GroupVersionKind())
}
```

### 2. Generator `hack/gen-samples/`

A small `main.go` imports `pkg/catalog`, iterates `catalog.List()`, marshals each definition
with `sigs.k8s.io/yaml` (already a direct dependency in `pkg/api/runai/v1alpha1`), and writes
`docs/catalog/{gvk-slug}.yaml`. It clears `docs/catalog/*.yaml` first so deletions
propagate. The slug:

```
slug = strings.ReplaceAll(group, ".", "-") + "-" + strings.ToLower(kind) + "-" + version
```

Output is deterministic: `List()` sorts the `byGVK` keys on each call, and
`sigs.k8s.io/yaml` marshals through JSON with stable key order from struct field order. Each file gets the SPDX and copyright YAML
header comment prepended.

### 3. Make and CI wiring

Add to the `Makefile`:

```make
.PHONY: generate-samples
generate-samples: ## Regenerate docs/catalog/ from pkg/catalog
	go run ./hack/gen-samples

# add generate-samples to the validate chain so git diff --exit-code covers it
validate: generate manifests generate-mocks generate-licenses generate-samples
	@git diff --exit-code
```

`make check` already runs `validate` in CI (`.github/workflows/ci.yaml`, `make check`), so
drift between `pkg/catalog/` and `docs/catalog/` fails the build via the existing
`git diff --exit-code`. No new CI step is needed. This mirrors how the CRD, DeepCopy, mock,
and license generators are already validated.

### 4. Porting the 20 existing samples

- Port each existing `docs/samples/*.yaml` definition to a typed Go file under
  `pkg/catalog/kartas/`, moving inline jq comments to Go comments. The generator then
  produces the GVK-slug-named YAML under `docs/catalog/`, and the round-trip test guarantees
  the Go structs reproduce it.
- Once every definition is ported, `docs/samples/` is deleted: the generated `docs/catalog/`
  becomes the single home for the YAML definitions.
- The existing sample-validation test (`pkg/api/runai/v1alpha1/examples_test.go`) is replaced
  by the new `pkg/catalog` tests, which validate `docs/catalog/` instead of `docs/samples/`.
- Update references that point at the samples set (README, `docs/examples/`, and
  `docs/ri-studio/wasm/examples/` per issue #116) to `docs/catalog/`.

### 5. Tests

Under `pkg/catalog/`:

- Structural validity: for every `catalog.List()` entry, run
  `NewKartaValidator(k).Validate()` and assert `APIVersion` and `Kind`.
- Round-trip equality: marshal each definition and assert byte-equality against the
  committed `docs/catalog/{slug}.yaml`, matching the issue #118 acceptance and giving the
  `git diff --exit-code` guarantee at unit-test level.
- Catalog semantics: `Get` hit and miss (`ErrNotFound`), `List` determinism, and that
  `New()` panics on a duplicate GVK.
- Completeness: this is what enforces that every workload is actually in the catalog, not
  reviewer diligence. The test parses `pkg/catalog/kartas/*.go` (excluding `_test.go`) with
  `go/ast`, collects every exported top-level constructor with signature
  `func() *v1alpha1.Karta`, and asserts that set of names equals the set wired into the
  `definitions` list in `catalog.go`. Writing a constructor but forgetting to list it, or
  listing a name with no constructor, makes the two sets differ and fails `make check`.
  Combined with the duplicate-GVK check in `New()` and the round-trip YAML test, this
  guarantees every definition is in the catalog exactly once and matches its committed YAML.

Deeper contract tests (pod-spec extraction, scale, status correctness against upstream
CRDs) are tracked by issue #79; this design leaves the seam for them.

## Files to create or modify

- New: `pkg/catalog/catalog.go`, `pkg/catalog/catalog_test.go`,
  `pkg/catalog/catalog_completeness_test.go` (the `go/ast` scan
  that enforces every constructor is wired into `definitions`).
- New: one `pkg/catalog/kartas/<workload>.go` per definition (jobset, pytorch, raycluster,
  rayjob, mpijob, lws, kserve, knative-serving, nimservice, milvus, dynamo,
  grove-podcliqueset, batch-job).
- New: `hack/gen-samples/main.go`.
- New directory: `docs/catalog/` (generated YAML).
- Remove: `docs/samples/` and the `pkg/api/runai/v1alpha1/examples_test.go` that validates
  it, replaced by the `pkg/catalog` tests over `docs/catalog/`.
- Modify: `Makefile` (add `generate-samples`, wire into `validate`).
- Modify: docs that reference the samples set (README, `docs/examples/`) to point at
  `docs/catalog/`.

## Verification

1. `make generate-samples`, then `git status` shows `docs/catalog/` regenerated with no
   unexpected diff.
2. `go test ./pkg/catalog/...` passes (structural, round-trip, catalog, completeness tests).
3. `make check` is green. Confirm the drift guard by editing one `pkg/catalog/kartas/*.go`
   value without regenerating and verifying `make validate` fails.
4. Smoke a Go caller:
   `catalog.Get(schema.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayCluster"})`
   returns the RayCluster Karta; an unknown GVK returns `ErrNotFound`.

## CLI integration (deferred to the CLI design)

The `pkg/catalog` catalog is the building block the CLI consumes. Two CLI behaviors depend
on it and are designed in `docs/design/cli/`, not here:

- An explicit flag selects the built-in catalog Kartas, so the CLI resolves a workload
  against `catalog.Get`/`catalog.List` only when the user opts in rather than implicitly.
- An option outputs a resolved Karta as JSON or YAML, so users can inspect or export a
  built-in definition directly.
