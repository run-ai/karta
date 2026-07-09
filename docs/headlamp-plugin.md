<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# Headlamp Plugin

The `headlamp-plugin/` directory contains a Headlamp plugin that visualizes
workload trees built by `pkg/tree`. This page documents how the plugin gets
its data and what cluster-side pieces exist.

## Architecture

1. The plugin lists Karta definitions and, for each definition's root kind,
   the workload objects. Both requests use the Headlamp user's credentials.
2. To render a tree, the plugin evaluates the Karta definition against the
   workload object in the browser, using a WebAssembly build of the karta Go
   library (`headlamp-plugin/wasm`). Tree building is pure computation on the
   two objects, so nothing leaves the browser and no operator deployment is
   required.

The WebAssembly module (`karta.wasm`, about 4 MiB compressed) and Go's
`wasm_exec.js` runtime glue ship inside the plugin package and are fetched
once per session. Build them with:

```bash
make plugin-wasm    # or make plugin-build, which includes it
```

The module exports one function:

```text
kartaBuildTree(kartaJSON, workloadJSON) -> {tree: <WorkloadTree JSON>} | {error: <message>}
```

Because the engine is compiled from this repository, tree semantics follow
the plugin version, not the operator version deployed in the cluster.

## RBAC for plugin users

Headlamp users need:

- `list` on `kartas.run.ai` and `customresourcedefinitions.apiextensions.k8s.io`.
- `list`/`get` on the workload kinds they want to inspect.

Tree building itself runs in the browser and needs no additional permissions.

## Limitations

- Workload kinds must be CRD-backed. Native kinds such as `Deployment` are
  not resolvable because the plugin discovers the resource plural and scope
  from the CustomResourceDefinition.
- The tree reflects the workload spec only. Live pod status and per-replica
  breakdowns are out of scope for the tree engine.
