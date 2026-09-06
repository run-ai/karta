// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

package main

import (
	"syscall/js"
)

func main() {
	registerBindings()

	// Block forever
	select {}
}

func registerBindings() {
	karta := js.Global().Get("Object").New()
	karta.Set("buildTree", js.FuncOf(jsBuildTree))
	karta.Set("attributePods", js.FuncOf(jsAttributePods))
	karta.Set("evaluatePhases", js.FuncOf(jsEvaluatePhases))
	karta.Set("listCatalog", js.FuncOf(jsListCatalog))
	js.Global().Set("karta", karta)
}
