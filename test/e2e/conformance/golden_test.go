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
	"github.com/run-ai/karta/test/e2e/cases"
)

// repoRoot is reached from this package's directory: go test runs with the working directory at
// test/e2e/conformance, so the repo root is three up. Recordings live under fixtures/.
const repoRoot = "../../.."

// TestGolden replays every recorded flow through the CURRENT Karta library with no cluster and no
// operators. For each flow it reconstructs each CR (first CR + merge-patches) and each stored reading
// (first reading + merge-patches), runs the CR through Karta, and checks three things at every step:
//
//   - Correctness anchor: Karta matches the recorded state, which was judged from the workload's own
//     fields at record time, never from Karta. This holds Karta to the ground truth, so a refreshed
//     golden can never quietly accept Karta drifting away from what the workload actually did.
//   - Golden: Karta's reading (status, conditions, per-component extraction) equals the recorded
//     reading. This catches any change in what Karta reads, not only a wrong state. Refresh it
//     deliberately after an intended change by re-recording (make record-e2e).
//   - Order: the recorded states are a legal subsequence of the case's declared journey (from
//     cases.All), ending at want - the same check the recorder runs live, now enforced offline too.
//
// This is the per-version guarantee run on every PR. New recordings are picked up automatically.
func TestGolden(t *testing.T) {
	root := "fixtures"

	var files []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".yaml") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		abs, _ := filepath.Abs(root)
		t.Fatalf("walk %s: %v", abs, err)
	}
	if len(files) == 0 {
		abs, _ := filepath.Abs(root)
		t.Fatalf("no recordings found under %s; a mis-pathed fixtures dir is a bug, not a pass (record with make record-e2e)", abs)
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
					t.Errorf("step %d (%s): Karta reading drifted from the recording (-recorded +now):\n%s\nif this change is intended, re-record with make record-e2e", i, step.State, diff)
				}
			}

			// Order: the recorded states must be a legal subsequence of the case's declared journey (the
			// same check the recorder runs live), ending at want. This catches a fixture whose order was
			// hand-edited or has drifted from its case.
			declared, ok := journeyFor(rec.Operator, rec.KartaName, rec.Flow)
			if !ok {
				t.Errorf("no case in cases.All for operator=%q kartaName=%q flow=%q", rec.Operator, rec.KartaName, rec.Flow)
				return
			}
			observed := make([]v1alpha1.ResourceStatus, len(rec.Steps))
			for i, s := range rec.Steps {
				observed[i] = v1alpha1.ResourceStatus(s.State)
			}
			if err := ObservedOrderErr(declared, observed, v1alpha1.ResourceStatus(rec.Want)); err != nil {
				t.Errorf("order: %v", err)
			}
		})
	}
}

// journeyFor is the declared journey of the case that produced this recording, looked up in cases.All
// by operator, definition, and flow. The journey is code, not stored in the fixture, so a recording
// whose order has drifted from its case is caught here.
func journeyFor(operator, kartaName, flow string) ([]JourneyStep, bool) {
	for _, tc := range cases.All {
		if tc.Operator != operator || tc.KartaName != kartaName {
			continue
		}
		for _, fl := range tc.Flows {
			if fl.Name == flow {
				out := make([]JourneyStep, len(fl.Journey))
				for i, st := range fl.Journey {
					out[i] = JourneyStep{State: st.State, Optional: st.Optional}
				}
				return out, true
			}
		}
	}
	return nil, false
}
