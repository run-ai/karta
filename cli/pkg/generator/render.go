// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"sigs.k8s.io/yaml"
)

var ErrUnsupportedOutput = errors.New("unsupported output format")

// Render writes items in the machine formats and hands the human ones to table.
func Render[T any](out io.Writer, format Output, items []T, table func(io.Writer) error) error {
	switch format {
	case OutputTable, OutputWide:
		return table(out)

	case OutputJSON:
		if items == nil {
			// A nil slice marshals as null, where an empty result is an empty list.
			items = []T{}
		}
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(items); err != nil {
			return fmt.Errorf("encode as json: %w", err)
		}
		return nil

	case OutputYAML:
		docs := make([]string, 0, len(items))
		for _, item := range items {
			data, err := yaml.Marshal(item)
			if err != nil {
				return fmt.Errorf("encode as yaml: %w", err)
			}
			docs = append(docs, string(data))
		}
		// A document stream, not a List object kubectl would need told about.
		if _, err := io.WriteString(out, strings.Join(docs, "---\n")); err != nil {
			return fmt.Errorf("write yaml: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("%w %q", ErrUnsupportedOutput, format)
	}
}
