// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Command resanitize re-applies the current conformance.Sanitize to already-recorded
// fixtures under the given roots and rewrites them. It exists for a fixture that was
// recorded before a sanitize rule was added (a volatile field the denylist now strips)
// and cannot be re-recorded live. It changes only the sanitized CR; the Karta reading
// (expected.yaml) is preserved, and TestGolden proves the reading is unchanged.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/run-ai/karta/test/conformance"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: resanitize <fixture-root>...")
		os.Exit(2)
	}
	for _, root := range os.Args[1:] {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() != "fixture.yaml" {
				return nil
			}
			dir := filepath.Dir(path)
			fx, data, err := conformance.Load(dir)
			if err != nil {
				return fmt.Errorf("load %s: %w", dir, err)
			}
			for k, sd := range data {
				conformance.Sanitize(sd.CR)
				data[k] = sd
			}
			if err := conformance.Write(dir, fx, data); err != nil {
				return fmt.Errorf("write %s: %w", dir, err)
			}
			fmt.Println("re-sanitized", dir)
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
