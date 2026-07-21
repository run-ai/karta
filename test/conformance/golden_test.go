// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// repoRoot is reached from this package's directory: go test runs with the working
// directory at test/conformance, so the repo root is two up. The recorded fixtures live
// alongside this package under fixtures/.
const repoRoot = "../.."

// TestGolden replays every recorded conformance fixture through the CURRENT Karta
// library and fails if the library's reading of any saved snapshot changed. No
// cluster, no operators - this is the per-version guarantee, run on every PR. New
// fixtures are picked up automatically; adding one needs no change here.
func TestGolden(t *testing.T) {
	root := "fixtures"

	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // conformance/ may not exist yet
		}
		if !d.IsDir() && d.Name() == "fixture.yaml" {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if len(dirs) == 0 {
		t.Skip("no conformance fixtures recorded yet (run: make record-e2e)")
	}

	for _, dir := range dirs {
		rel, _ := filepath.Rel(root, dir)
		t.Run(rel, func(t *testing.T) {
			fx, data, err := Load(dir)
			if err != nil {
				t.Fatalf("load fixture: %v", err)
			}
			if fx.SchemaVersion != SchemaVersion {
				t.Fatalf("fixture schema v%d != current v%d; refresh offline with `go run ./hack/regolden`, or re-record with make record-e2e", fx.SchemaVersion, SchemaVersion)
			}

			kartaBytes, err := os.ReadFile(filepath.Join(repoRoot, fx.KartaFile))
			if err != nil {
				t.Fatalf("read %s: %v", fx.KartaFile, err)
			}

			var karta v1alpha1.Karta
			if err := yaml.Unmarshal(kartaBytes, &karta); err != nil {
				t.Fatalf("parse %s: %v", fx.KartaFile, err)
			}

			// Transition sequence: snapshot dirs are NN-<State>, contiguous and in
			// order, and collapsing their states reproduces the recorded ObservedStates.
			var snapStates []string
			for i, s := range fx.Snapshots {
				if want := SnapshotDir(i, s.State); s.Dir != want {
					t.Errorf("snapshot %d: dir %q, want %q", i, s.Dir, want)
				}
				snapStates = append(snapStates, s.State)
			}
			if got := slices.Compact(snapStates); !slices.Equal(got, fx.ObservedStates) {
				t.Errorf("snapshot states collapse to %v, but ObservedStates=%v", got, fx.ObservedStates)
			}

			for _, s := range fx.Snapshots {
				sd, ok := data[s.Dir]
				if !ok {
					t.Errorf("missing snapshot dir %s", s.Dir)
					continue
				}
				got, err := Replay(&karta, sd.CR)
				if err != nil {
					t.Errorf("state=%s: replay: %v", s.State, err)
					continue
				}
				if diff := cmp.Diff(sd.Expected, got); diff != "" {
					t.Errorf("operator=%s version=%s flow=%s state=%s: library output changed (-recorded +current):\n%s",
						fx.Operator, fx.Version, fx.Flow, s.State, diff)
				}
			}

			// The terminal snapshot must read as the flow's declared want (offline echo
			// of the live gate). Only checked for fixtures recorded with the want field.
			if fx.Want != "" && len(fx.Snapshots) > 0 {
				last := fx.Snapshots[len(fx.Snapshots)-1]
				if ms := data[last.Dir].Expected.MatchedStatuses; !slices.Contains(ms, fx.Want) {
					t.Errorf("flow=%s terminal=%s: matchedStatuses %v does not contain want %s",
						fx.Flow, last.State, ms, fx.Want)
				}
			}
		})
	}
}
