<!--
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 NVIDIA Corporation
-->

# Karta Headlamp Plugin

A [Headlamp](https://headlamp.dev) plugin that visualizes Karta workload
trees: the components, instances, and normalized status of any workload
described by a Karta definition (`kartas.run.ai`).

## How the WASM engine is built and loaded

`engine/` is a separate Go module compiled to WebAssembly
(`GOOS=js GOARCH=wasm`) and shipped as a plugin asset via the
`headlamp.extraDist` entry in `package.json`. At runtime, `src/lib/engine.ts`
fetches `wasm_exec.js` and `karta.wasm` from wherever Headlamp served this
plugin from and instantiates the module in the browser — see `src/lib/engine.ts`
for the details.

## Development

Requires Node.js >= 22, npm, and Go (for the WebAssembly build).

```bash
make plugin-wasm   # from the repository root; builds engine/karta.wasm + engine/wasm_exec.js
npm install
npm start
```

`npm start` watches `src/` and rebuilds the plugin bundle on change, copying
it (plus whatever is currently in `engine/`) into Headlamp's plugin
directory. It does **not** watch Go source or rerun `make plugin-wasm` for
you — after editing `engine/main.go`, rerun `make plugin-wasm` from the
repository root, then save any file under `src/` (or restart `npm start`) to
pick up the new binary.

Headlamp only reads its plugin directory at startup, so after the very first
install, or whenever the set of files in Headlamp's plugin directory
(`~/Library/Application Support/Headlamp/plugins/karta` on macOS,
`~/.config/Headlamp/plugins/karta` on Linux) changes (not just their
contents), fully quit and reopen the Headlamp desktop app to pick it up. It
does not hot-reload plugins.

Lint, type-check, and test with:

```bash
npm run lint
npm run tsc
npm run test
```

## CI

`.github/workflows/plugin-ci.yaml` runs on push/PR to `main`/`v0.*`, gated to
changes under `headlamp-plugin/**` or the root `Makefile`. It runs
`make plugin-build` from the repository root, which is:

```bash
make plugin-wasm   # go build the WASM module
npm ci
npm run lint
npm run tsc
npm run test
npm run build      # production vite build; extraDist copies engine/karta.wasm + wasm_exec.js into dist/
```
