// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cmd

import (
	"fmt"
	"io"

	"github.com/run-ai/karta/cli/pkg/definitions"
)

// printWarnings reports non-fatal diagnostics on stderr, where they stay clear
// of machine-readable output.
func printWarnings(out io.Writer, messages []string) error {
	for _, message := range messages {
		if _, err := fmt.Fprintf(out, "warning: %s\n", message); err != nil {
			return fmt.Errorf("write warning: %w", err)
		}
	}
	return nil
}

// printLoadWarnings reports what definition loading could not do. Every command
// that reads definitions degrades the same way - catalog-only, with a note - so
// they report it the same way too.
func printLoadWarnings(out io.Writer, warnings []definitions.Warning) error {
	messages := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		messages = append(messages, warning.Message)
	}
	return printWarnings(out, messages)
}
