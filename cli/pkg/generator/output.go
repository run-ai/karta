// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package generator holds types and helpers for rendering karta output.
package generator

// Output is the enum of supported CLI output formats.
type Output string

const (
	OutputTable Output = "table"
	OutputWide  Output = "wide"
	OutputJSON  Output = "json"
	OutputYAML  Output = "yaml"
)
