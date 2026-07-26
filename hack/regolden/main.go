// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Command regolden refreshes every recording's expected reading by replaying its frozen CRs through
// the current Karta library, and bumps each to the current SchemaVersion. It needs no cluster: the CRs
// are already recorded and Reading is a pure function of them, so this is the right way to accept an
// intended change in what Karta reads. It rewrites only the reading (first reading + merge-patches);
// the CRs and the recorded states are left untouched. The golden's correctness anchor still holds
// afterwards: if Karta no longer reads a step's recorded state, the refreshed reading will not contain
// it and TestGolden fails, so regolden cannot paper over Karta drifting from the ground truth. Run
// from the repo root: go run ./hack/regolden
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
	"github.com/run-ai/karta/test/conformance"
)

func main() {
	root := "test/conformance/fixtures"
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, file := range files {
		if err := regen(file); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", file, err)
			os.Exit(1)
		}
		fmt.Println("regoldened", file)
	}
}

func regen(file string) error {
	rec, err := conformance.LoadRecording(file)
	if err != nil {
		return err
	}
	kb, err := os.ReadFile(rec.KartaFile)
	if err != nil {
		return err
	}
	var karta v1alpha1.Karta
	if err := yaml.Unmarshal(kb, &karta); err != nil {
		return err
	}

	crs, err := rec.CRs()
	if err != nil {
		return err
	}
	var prev map[string]interface{}
	for i := range rec.Steps {
		reading, err := conformance.Reading(&karta, crs[i])
		if err != nil {
			return fmt.Errorf("step %d (%s): %w", i, rec.Steps[i].State, err)
		}
		if i == 0 {
			rec.Steps[i].Expected = reading
			rec.Steps[i].ExpectedPatch = nil
		} else {
			rec.Steps[i].Expected = nil
			rec.Steps[i].ExpectedPatch = conformance.MergePatch(prev, reading)
		}
		prev = reading
	}
	rec.SchemaVersion = conformance.SchemaVersion
	return conformance.WriteRecording(file, rec)
}
