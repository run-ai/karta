<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# Karta Headlamp Plugin

A [Headlamp](https://headlamp.dev) plugin that visualizes Karta workload trees:
the components, instances, and normalized status of any workload described by a
Karta definition.

## Features

- Karta sidebar section with a workload list per Karta definition and a
  definitions browser. The section is hidden when the Karta CRD is not
  installed in the cluster.
- Workload tree view: component hierarchy, instance keys, scale, container
  image, and normalized status phases.
- Map integration: Karta workloads appear as a source in Headlamp's Map view.
  Selecting a node shows the workload tree in the details panel.

## How it works

The plugin fetches the Karta definition and the workload object with your own
Kubernetes credentials, then evaluates the definition against the workload in
the browser with a WebAssembly build of the karta Go library (`tree.Build`
from `pkg/tree`, compiled from `wasm/`). No operator deployment or extra RBAC
is needed for tree building. See `docs/headlamp-plugin.md` in the repository
root for details.

## Requirements

- Karta CRs installed in the cluster (`kartas.run.ai`).
- Permission to list `kartas.run.ai`, CustomResourceDefinitions, and the
  workload kinds to inspect.

## Development

Requires Node.js >= 22, npm >= 11, and Go (for the WebAssembly engine).

```bash
make plugin-wasm   # from the repository root; builds wasm/karta.wasm + wasm_exec.js
npm install
npm run start
```

Open the Headlamp desktop app; it picks up plugins in development mode
automatically. Lint, type-check, and test with:

```bash
npm run lint
npm run tsc
npm run test
```

## Install into Headlamp

Desktop (macOS/Linux):

```bash
make plugin-wasm   # from the repository root
npm install
npm run build
npm run package
mkdir -p ~/.config/Headlamp/plugins
tar xvf karta-0.1.0.tar.gz -C ~/.config/Headlamp/plugins/
```

In-cluster Headlamp: copy the built plugin into the directory served by
Headlamp's `-plugins-dir` flag, for example with an init container. See the
Headlamp docs on building and shipping plugins.
