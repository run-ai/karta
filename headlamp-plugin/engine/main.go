// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

// Command wasm is a placeholder WebAssembly module proving the
// make plugin-wasm build pipeline end-to-end. The real Karta engine
// bindings (tree building, pod attribution, status evaluation) are added
// once RUN-42193 wires pkg/tree, pkg/resource, and pkg/status into this
// module.
package main

import "syscall/js"

func main() {
	js.Global().Set("kartaVersion", js.FuncOf(func(js.Value, []js.Value) any {
		return js.ValueOf("dev")
	}))
	select {}
}
