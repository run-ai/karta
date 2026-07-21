// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package conformance

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"testing"
)

// TestFixturesRecordIntermediateCRs guards the recorder's granularity: it must keep every
// distinct CR a workload passes through, not one snapshot per state. When a workload sits
// in one state (say Running) across several distinct CRs, each is recorded and TestGolden
// replays each - so a later library change that would read a MIDDLE Running CR as Degraded
// is caught. A one-snapshot-per-state recording would keep only the first Running and hide
// that regression.
//
// The test fails if no recorded flow captured an intermediate CR (a state repeated across
// distinct CRs), which is what a regression back to one-per-state would look like. It also
// checks that repeated same-state snapshots are genuinely distinct CRs (dedup-by-content
// must have collapsed identical ones).
func TestFixturesRecordIntermediateCRs(t *testing.T) {
	root := "fixtures"

	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "fixture.yaml" {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if len(dirs) == 0 {
		t.Skip("no conformance fixtures recorded yet (run: make record-e2e)")
	}

	withIntermediate := 0
	for _, dir := range dirs {
		fx, data, err := Load(dir)
		if err != nil {
			t.Fatalf("load %s: %v", dir, err)
		}
		for i := 1; i < len(fx.Snapshots); i++ {
			prev, cur := fx.Snapshots[i-1], fx.Snapshots[i]
			if prev.State != cur.State {
				continue // a state boundary, not a repeated state
			}
			// Same state across two snapshots: dedup-by-content means they must be
			// distinct CRs, and golden replays both.
			a, _ := json.Marshal(data[prev.Dir].CR.Object)
			b, _ := json.Marshal(data[cur.Dir].CR.Object)
			if string(a) == string(b) {
				t.Errorf("%s/%s: %s and %s share a state and an identical CR; dedup should have collapsed them",
					fx.Operator, fx.Flow, prev.Dir, cur.Dir)
			}
			withIntermediate++
			break
		}
	}
	if withIntermediate == 0 {
		t.Error("no recorded flow captured an intermediate CR (a state repeated across distinct CRs); " +
			"the recorder may have been collapsed to one snapshot per state, which would hide a reading " +
			"regression on a middle state (for example a Running that a change would later read Degraded)")
	} else {
		t.Logf("%d recorded flow(s) capture intermediate CRs (a state repeated across distinct CRs)", withIntermediate)
	}
}
