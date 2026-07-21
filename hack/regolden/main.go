// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Command regolden regenerates every fixture's expected.yaml by replaying its frozen
// cr.yaml through the current Karta library, and bumps each fixture to the current
// SchemaVersion. It needs no cluster: the CRs are already recorded, and Replay is a
// pure function of them, so this is the right way to refresh the golden after changing
// what Replay extracts. It reads only fixture.yaml and each cr.yaml (never the old
// expected.yaml), so it works across an incompatible expected-format change. Run from
// the repo root: go run ./hack/regolden
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/conformance"
)

func main() {
	root := "test/conformance/fixtures"
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "fixture.yaml" {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, dir := range dirs {
		if err := regen(dir); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", dir, err)
			os.Exit(1)
		}
		fmt.Println("regenerated", dir)
	}
}

func regen(dir string) error {
	var fx conformance.Fixture
	if err := readYAML(filepath.Join(dir, "fixture.yaml"), &fx); err != nil {
		return err
	}
	kartaBytes, err := os.ReadFile(fx.KartaFile)
	if err != nil {
		return err
	}
	var karta v1alpha1.Karta
	if err := yaml.Unmarshal(kartaBytes, &karta); err != nil {
		return err
	}

	data := map[string]conformance.SnapshotData{}
	for _, s := range fx.Snapshots {
		crMap := map[string]interface{}{}
		if err := readYAML(filepath.Join(dir, s.Dir, "cr.yaml"), &crMap); err != nil {
			return err
		}
		cr := &unstructured.Unstructured{Object: crMap}
		conformance.Sanitize(cr) // re-apply the current denylist (it may have grown since record)
		reading, err := conformance.Replay(&karta, cr)
		if err != nil {
			return err
		}
		data[s.Dir] = conformance.SnapshotData{CR: cr, Expected: reading}
	}
	fx.SchemaVersion = conformance.SchemaVersion
	return conformance.Write(dir, fx, data)
}

func readYAML(path string, v interface{}) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
