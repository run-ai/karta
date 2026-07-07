// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package cliflag holds the flag value structures for the karta CLI: types
// that implement pflag.Value and define a flag's shape and behavior (parse,
// validate, print). It starts with the output-format enum.
package cliflag

// Output is the enum of supported CLI output formats.
type Output string

const (
	OutputTable Output = "table"
	OutputWide  Output = "wide"
	OutputJSON  Output = "json"
	OutputYAML  Output = "yaml"
)

// NewOutput returns an Enum backing the -o/--output flag, defaulting to table.
func NewOutput() *Enum[Output] {
	return NewEnum("output", OutputTable, OutputTable, OutputWide, OutputJSON, OutputYAML)
}
