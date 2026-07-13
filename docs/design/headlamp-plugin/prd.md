<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# Karta Headlamp Plugin - Product Requirements Document

## Background

Karta is a CRD-based Go library that provides a universal abstraction for any
Kubernetes workload type. Using JQ-based Karta definitions, it extracts
structure (components, hierarchy), status (phases, conditions), scaling
(replicas, min/max), and pod specs from any CRD: PyTorchJob, RayCluster,
JobSet, KServe, Dynamo, and others.

The Karta CLI design (`docs/design/cli/high-level-design.md`) identifies the
gap this project fills: no existing tool provides workload-aware visibility
across CRD types. kubectl works per resource type, kubectl-tree walks
ownership without semantics, and general cluster browsers have no concept of
a workload as a composed unit.

The Headlamp plugin brings this visibility to [Headlamp](https://headlamp.dev),
the CNCF Kubernetes UI. It answers, in a browser:

- What workloads run in my cluster, across all types, and what is their
  normalized status?
- What is the structure of a workload: which components, instances, and pods
  play which role?
- How many resources (CPU, memory, GPU) does a workload request and use?

## Goals

- Visualize the workload tree (components, instances, live pods) for any
  workload type described by a Karta definition.
- Work with zero server-side dependencies: no operator endpoint, no extra
  RBAC, no plugin settings. Tree semantics come from the Karta Go library
  itself, compiled to WebAssembly and evaluated in the browser.
- Respect the user's own RBAC: every cluster read uses the Headlamp user's
  credentials.
- Integrate natively with Headlamp: sidebar, routes, resource links, the Map
  view, light and dark themes.

## Non-goals (v1)

- Write actions (scale, suspend, delete). The plugin is read-only.
- Native (non-CRD) workload kinds such as Deployment. Resource plural and
  scope are discovered from the CustomResourceDefinition, so only CRD-backed
  kinds are listed.
- Ready counts and the full `WorkloadView` display layer from the CLI design.
  See Future work.
- Publishing to ArtifactHub / the Headlamp Plugin Catalog.

## Architecture

### WebAssembly tree engine

The plugin ships a WebAssembly build of the Karta library
(`headlamp-plugin/wasm`, built by `make plugin-wasm`). Requirements:

- Exports `kartaBuildTree(kartaJSON, workloadJSON)` returning the
  `WorkloadTree` JSON produced by `pkg/tree` (which carries stable camelCase
  json tags), or a structured error.
- Exports `kartaMatchPods(kartaJSON, podListJSON)` classifying pods to
  components and instances via the definition's `podSelector`:
  - `componentTypeSelector` establishes component membership.
  - `componentInstanceSelector` extracts the instance key. A definition with
    only an instance selector (for example JobSet's replicated-job label)
    also establishes membership when the key is present on the pod.
  - A pod lacking the selector keys is not a match; it is never an error.
- Tree building is pure computation on the two JSON documents. It never
  contacts the API server, so JQ evaluation semantics are identical to the Go
  library by construction.
- The module (`karta.wasm`, about 18 MiB raw / 4 MiB compressed) and Go's
  `wasm_exec.js` runtime glue are built from the same Go toolchain, shipped
  in the plugin package via `headlamp.extraDist`, fetched once per session,
  and instantiated lazily. A failed load is retried; the plugin base URL is
  discovered from Headlamp's `/plugins` metadata with a conventional
  fallback.
- Because the engine is compiled from this repository, tree semantics follow
  the plugin version, not any operator deployed in the cluster.

An operator-hosted HTTP endpoint (stateless `POST /v1/tree`) was implemented
first and intentionally removed in favor of the WASM engine: it required a
Service, chart wiring, and `services/proxy` RBAC for every user, and existed
only to serve this plugin.

### Data access

- Karta definitions are read via a `KubeObject` subclass
  (`kartas.run.ai`, cluster-scoped). The root workload GVK is resolved from
  the operator-stamped `karta.run.ai/group|version|kind` labels, falling back
  to the definition spec.
- Workload classes are constructed dynamically per GVK by locating the CRD
  and reading its plural name and scope.
- Sub-resource discovery lists the definition's child component kinds plus
  `additionalChildKinds` plus Pods, in the workload's namespace, and keeps
  objects whose `ownerReferences` chain reaches the workload transitively.
- Pods without an owner chain (some operators create pods without
  ownerReferences) are kept only when the podSelector matches AND the pod
  references the workload by name (a label or annotation value equal to the
  workload name, or a `<workload>-` name prefix). Selectors identify
  component membership, not workload identity, so this guard prevents
  claiming pods of a sibling workload of the same kind.

## Functional requirements

### Sidebar and navigation

- A top-level `Karta` sidebar section with `Workloads` and `Definitions`
  entries. The section is hidden on clusters where the `kartas.run.ai` CRD is
  not installed (checked per cluster with a short TTL).
- All links are cluster-prefixed through Headlamp's router (`createRouteURL`
  by route name); raw route paths must never be used for navigation targets.

### Workloads tab

- One section per Karta definition whose workload CRD is applied in the
  cluster. The section title is the workload kind only (for example
  `JobSet`), in the small `label` header style; the page title uses the
  `subsection` style.
- Definitions whose workload CRD is not applied render nothing.
- Definitions whose kind has no workload instances are aggregated below the
  populated sections, inside a collapsed `Kinds without workloads (N)`
  accordion.
- Table columns: Name (link to the workload tree page), Namespace, Kind,
  Phase, Components, Age.
- The Phase column evaluates the workload's tree locally per row via the
  WASM engine and renders the normalized phases as colored chips.
- The Components column lists the definition's child component names.

### Workload page

- Back button returning to the Workloads list.
- Details table with Name (linking to the workload's built-in custom
  resource page), Namespace, Kind, owning Karta, and two aggregate rows:
  - Requested Resources: the workload's desired totals from the spec tree.
    Per instance: container requests multiplied by replicas, recursing into
    nested components. CPU and memory prefer requests over limits; GPU counts
    come from limits (`nvidia.com/gpu`). Rendered as `cpu / mem / gpu`.
  - Actual Usage: live CPU and memory summed from the `metrics.k8s.io` API
    for the workload's pods, plus GPUs allocated by currently Running pods
    (GPU utilization is not reported by metrics-server). When the metrics
    API is unavailable the row says so while still reporting GPU allocation.

#### Workload Tree section

- An interactive flow graph (bundled `@xyflow/react`, MIT) with pan, zoom,
  draggable nodes, fit-view controls, and Headlamp theme awareness (light
  and dark color modes).
- Top-down layered layout computed by a tested pure module: one row per
  depth, siblings never overlap, parents centered over their children's
  span.
- Node types and accent colors, with a legend:
  - workload root: blue; shows kind, name, and normalized phase chips.
  - component: purple; shows name and kind (or `logical group`).
  - instance: green; shows the instance key, a replicas badge (`x8`), a
    `scale min..max` line when defined, and per-container resource rows
    (`cpu / mem / gpu`, GPU highlighted when non-zero).
  - pod: accent colored by phase (Running green, Pending amber, Failed red,
    Succeeded blue); shows name, phase, and node placement.
- Live pods attach under their matching instance (by extracted instance key,
  or to the single instance of a component). At most 8 pods render per
  instance, with a `+N more pods` overflow node.
- A `Pods (N)` toggle above the graph collapses or expands the pod level;
  the view re-fits on toggle.
- The root node links to the workload's resource page; pod nodes link to
  their Pod pages.

#### Resources section

- A table of the live Kubernetes objects belonging to the workload, ordered
  as a hierarchy: every object renders below its parent, indented per depth.
- Columns: Kind, Name (link to the built-in Headlamp resource page; built-in
  kinds use their native pages, CRD-backed kinds use the generic custom
  resource page), Component (component and instance for selector-matched
  pods), Status, Age.

### Definitions tab

- Table of Karta definitions: Name (link to the built-in custom resource
  page for `kartas.run.ai`), Workload Kind (group/version kind), Components,
  Ready, Age.
- The Ready column surfaces the operator-managed Ready condition: a green
  `Ready` chip, a red chip with the failure reason and message tooltip when
  not ready, and a grey `Unknown` chip (with a hint that the operator may
  not be running) when no status is reported.

### Map view integration

- A `Karta` source registered in Headlamp's Map view: one node per Karta
  workload, using the workload's uid, with the owning Karta as subtitle.
- Selecting a node shows the workload tree in the details side panel using a
  compact list rendering (the flow graph needs more width than the panel
  offers). The tree is evaluated only on selection, never fanned out for all
  nodes upfront.

### Status phases

- One distinct color per normalized phase: Running (green), Completed
  (blue), Failed (red), Degraded (amber), Initializing (purple), Suspended
  and Undefined (grey). An empty match list renders `Undefined`.
- Chip colors derive from the active theme palette with alpha-tinted
  backgrounds. Text contrast is computed with lighten/darken from the main
  color, not the palette's light/dark variants, because some Headlamp themes
  define those too close to the background.

## Non-functional requirements

- No cluster-side deployment is required for any plugin feature. The
  operator only manages Karta CRs (validation, conditions, GVK labels).
- Required user RBAC: list `kartas.run.ai`, list CustomResourceDefinitions,
  and list/get the workload kinds (and their children and pods) the user
  wants to inspect. Nothing else.
- The plugin follows the official Headlamp plugin conventions: scaffolded
  with `@kinvolk/headlamp-plugin`, shared dependencies (React, MUI, router)
  are provided by the host and never bundled; the only bundled runtime
  dependency is `@xyflow/react`.
- All new source files carry SPDX + copyright headers per repository policy.

## Build, test, distribution

- `make plugin-wasm` builds the engine and copies the matching
  `wasm_exec.js` from the active Go toolchain. `make plugin-build` runs it,
  then lint, type-check, and the production build. Node >= 22, npm >= 11,
  and a Go toolchain are required to build; artifacts are gitignored.
- Unit tests (vitest) cover the layout module (graph building, pod
  attachment, overflow, links, sibling overlap, parent centering), the
  resources hierarchy ordering, quantity parsing/formatting/aggregation, and
  the app URL derivation. The Go engine is smoke-tested in Node against real
  definitions (tree output, pod matching including instance extraction, and
  error paths).
- Distribution: `npm run package` tarball extracted into the Headlamp
  desktop plugins directory, or copied into `-plugins-dir` via initContainer
  for in-cluster Headlamp. Dev loop: `npm run start` with the desktop app.

## Known limitations

- Workload kinds must be CRD-backed (see Non-goals).
- The tree reflects the workload spec; readiness ratios ("3/4 ready") and
  per-replica breakdowns are not yet computed.
- Phase evaluation runs per listed workload row; very large clusters will
  want pagination or lazy evaluation for visible rows.
- Sub-resource discovery depends on ownerReferences plus the podSelector
  fallback; children created without owner refs that are not pods are not
  discovered.
- GPU "actual usage" is allocation of running pods, not utilization.

## Future work

Aligned with the CLI high-level design, in suggested order:

1. Adopt a shared `WorkloadView` layer in `pkg/` (ready counts, pod details,
   resource aggregation) and expose it through the WASM engine so the CLI,
   web, and MCP consume identical semantics.
2. Ready counts and per-replica breakdowns in the tree and list (PodMatcher).
3. GPU column in the Workloads table, and a per-component resources table
   with totals (mirroring `karta workload resources`).
4. Embed community definitions (docs/samples) in the engine so the plugin
   works on clusters with no Karta CRs, with an ORIGIN column and
   cluster-over-community precedence.
5. List UX: phase/kind/label filters, namespace scoping, pagination.
6. Definition describe (structure, status mappings, gang scheduling) and
   validate/preview: paste a Karta or workload manifest and render its tree
   pre-submission.
7. Write actions (scale, suspend) once the read-only experience is settled.
