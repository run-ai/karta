// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package flagtypes holds the flag value structures for the karta CLI: types
// that implement pflag.Value and define a flag's shape and behavior (parse,
// validate, print). It starts with the output-format enum.
package flagtypes

import "fmt"

// Output is the enum of supported CLI output formats. It implements pflag.Value
// so it can back the --output flag directly, validating at parse time.
type Output string

const (
	OutputTable Output = "table"
	OutputWide  Output = "wide"
	OutputJSON  Output = "json"
	OutputYAML  Output = "yaml"
)

// OutputValues lists every valid value, for usage text and shell completion.
var OutputValues = []string{
	string(OutputTable), string(OutputWide), string(OutputJSON), string(OutputYAML),
}

func (o *Output) String() string { return string(*o) }

func (o *Output) Set(v string) error {
	switch Output(v) {
	case OutputTable, OutputWide, OutputJSON, OutputYAML:
		*o = Output(v)
		return nil
	default:
		return fmt.Errorf("must be one of table, wide, json, yaml")
	}
}

func (o *Output) Type() string { return "output" }

// NewOutput returns an Output seeded with the default (table), for pflag's VarP.
func NewOutput() *Output { o := OutputTable; return &o }
