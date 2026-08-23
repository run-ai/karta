// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

package main

import "syscall/js"

func main() {
	registerKartaVersion()

	// Block forever
	select {}
}

func registerKartaVersion() {
	js.Global().Set("kartaVersion", js.FuncOf(kartaVersion))
}

func kartaVersion(js.Value, []js.Value) any {
	return js.ValueOf("dev")
}
