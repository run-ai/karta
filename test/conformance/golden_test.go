// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// repoRoot is reached from this package's directory: go test runs with the working directory at
// test/conformance, so the repo root is two up. Recordings live under fixtures/.
const repoRoot = "../.."

// terminalStates are sinks: once a workload reads as one, nothing legal may come after.
var terminalStates = []string{string(v1alpha1.CompletedStatus), string(v1alpha1.FailedStatus)}

// TestGolden replays every recorded flow through the CURRENT Karta library with no cluster and no
// operators. For each flow it reconstructs each CR (first CR + merge-patches) and each stored reading
// (first reading + merge-patches), runs the CR through Karta, and checks three things at every step:
//
//   - Correctness anchor: Karta matches the recorded state, which was judged from the workload's own
//     fields at record time, never from Karta. This holds Karta to the ground truth, so a refreshed
//     golden can never quietly accept Karta drifting away from what the workload actually did.
//   - Golden: Karta's reading (status, conditions, per-component extraction) equals the recorded
//     reading. This catches any change in what Karta reads, not only a wrong state. Refresh it
//     deliberately after an intended change with go run ./hack/regolden.
//   - Legal transition: a terminal state appears only as the last step, and the last step is want.
//
// This is the per-version guarantee run on every PR. New recordings are picked up automatically.
func TestGolden(t *testing.T) {
	root := "fixtures"

	var files []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	if len(files) == 0 {
		t.Skip("no recordings yet (run: make record-e2e)")
	}

	for _, file := range files {
		rel, _ := filepath.Rel(root, file)
		t.Run(strings.TrimSuffix(rel, ".yaml"), func(t *testing.T) {
			rec, err := LoadRecording(file)
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
				t.Fatalf("reconstruct CRs: %v", err)
			}
			want, err := rec.Readings()
			if err != nil {
				t.Fatalf("reconstruct readings: %v", err)
			}
			if len(crs) != len(rec.Steps) || len(want) != len(rec.Steps) {
				t.Fatalf("reconstructed %d CRs and %d readings for %d steps", len(crs), len(want), len(rec.Steps))
			}

			for i, step := range rec.Steps {
				got, err := Reading(&karta, crs[i])
				if err != nil {
					t.Errorf("step %d (%s): read: %v", i, step.State, err)
					continue
				}
				if matched := matchedStatuses(got); !slices.Contains(matched, step.State) {
					t.Errorf("step %d: recorded state %s but Karta matched %v", i, step.State, matched)
				}
				if diff := cmp.Diff(want[i], got); diff != "" {
					t.Errorf("step %d (%s): Karta reading drifted from the recording (-recorded +now):\n%s\nif this change is intended, refresh with go run ./hack/regolden", i, step.State, diff)
				}
			}

			for i, s := range rec.Steps {
				if i < len(rec.Steps)-1 && slices.Contains(terminalStates, s.State) {
					t.Errorf("terminal state %s at step %d is not last; nothing may follow a terminal", s.State, i)
				}
			}
			if rec.Want != "" && len(rec.Steps) > 0 {
				if last := rec.Steps[len(rec.Steps)-1].State; last != rec.Want {
					t.Errorf("last state %s != want %s", last, rec.Want)
				}
			}
		})
	}
}
