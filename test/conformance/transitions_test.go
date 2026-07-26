// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// repoRoot is reached from this package's directory: go test runs with the working directory at
// test/conformance, so the repo root is two up. Recordings live under fixtures/.
const repoRoot = "../.."

// terminalStates are sinks: once a workload reads as one, nothing legal may come after.
var terminalStates = []v1alpha1.ResourceStatus{v1alpha1.CompletedStatus, v1alpha1.FailedStatus}

// TestTransitions replays every recorded flow through the CURRENT Karta library with no cluster and
// no operators: it reconstructs each CR from the first CR plus the merge-patches, asserts Karta reads
// the recorded state at each step, and asserts the state sequence is a legal path (terminals only at
// the end, ending at want). This is the per-version guarantee run on every PR. New recordings are
// picked up automatically. There is no sanitize: a recording is a first CR + diffs + states.
func TestTransitions(t *testing.T) {
	root := "fixtures"

	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == recordingFile {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if len(dirs) == 0 {
		t.Skip("no recordings yet (run: make record-e2e)")
	}

	for _, dir := range dirs {
		rel, _ := filepath.Rel(root, dir)
		t.Run(rel, func(t *testing.T) {
			rec, err := LoadRecording(dir)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if rec.SchemaVersion != SchemaVersion {
				t.Fatalf("recording schema v%d != current v%d; re-record with make record-e2e", rec.SchemaVersion, SchemaVersion)
			}

			kb, err := os.ReadFile(filepath.Join(repoRoot, rec.KartaFile))
			if err != nil {
				t.Fatalf("read %s: %v", rec.KartaFile, err)
			}
			var karta v1alpha1.Karta
			if err := yaml.Unmarshal(kb, &karta); err != nil {
				t.Fatalf("parse %s: %v", rec.KartaFile, err)
			}

			crs, err := rec.CRs()
			if err != nil {
				t.Fatalf("reconstruct: %v", err)
			}
			if len(crs) != len(rec.Steps) {
				t.Fatalf("reconstructed %d CRs for %d steps", len(crs), len(rec.Steps))
			}

			// Karta reads each recorded state, not just the terminal.
			for i, step := range rec.Steps {
				matched, err := Read(&karta, crs[i])
				if err != nil {
					t.Errorf("step %d (%s): read: %v", i, step.State, err)
					continue
				}
				if !slices.Contains(matched, v1alpha1.ResourceStatus(step.State)) {
					t.Errorf("step %d: recorded state %s but Karta matched %v", i, step.State, matched)
				}
			}

			// Legal transition path: a terminal state may appear only as the last step, and the
			// last step is the flow's want.
			for i, s := range rec.Steps {
				if i < len(rec.Steps)-1 && slices.Contains(terminalStates, v1alpha1.ResourceStatus(s.State)) {
					t.Errorf("terminal state %s at step %d is not last; nothing may follow a terminal", s.State, i)
				}
			}
			if rec.Want != "" && len(rec.Steps) > 0 {
				if last := rec.Steps[len(rec.Steps)-1].State; last != string(rec.Want) {
					t.Errorf("last state %s != want %s", last, rec.Want)
				}
			}
		})
	}
}
