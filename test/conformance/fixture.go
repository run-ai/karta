// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Package conformance is the shared seam between the live e2e recorder (test/e2e)
// and the offline golden replay (golden_test.go). Both call Replay so the way a
// recorded workload is read can never drift between the two, and both use the same
// on-disk schema under test/conformance/fixtures/<operator>/<version>/<kartaName>/<flow>/.
package conformance

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// SchemaVersion is bumped when the on-disk fixture format changes incompatibly.
// v4: expected.yaml carries the full extracted instances and status, stripped, rather
// than a hand-picked projection.
const SchemaVersion = 4

const (
	fixtureFile  = "fixture.yaml"
	crFile       = "cr.yaml"
	expectedFile = "expected.yaml"
)

// ComponentReading is everything Karta extracts for one component: each instance id
// mapped to its full extraction (pod template, scale, metadata), volatile fields removed.
type ComponentReading struct {
	Instances map[string]interface{} `json:"instances,omitempty"`
}

// Expected is what Karta reads for one CR snapshot: matched statuses, phase, conditions,
// and the full per-component extraction, all with volatile fields stripped.
type Expected struct {
	// MatchedStatuses has no omitempty on purpose: it is the primary thing the golden
	// guards, so every snapshot shows the matched set even when it is empty.
	MatchedStatuses []v1alpha1.ResourceStatus   `json:"matchedStatuses"`
	Phase           *string                     `json:"phase,omitempty"`
	Conditions      []interface{}               `json:"conditions,omitempty"`
	Components      map[string]ComponentReading `json:"components,omitempty"`
}

// Snapshot indexes one recorded state. Its data lives in its own directory Dir,
// which holds cr.yaml (the sanitized CR) and expected.yaml (Karta's reading), so a
// reviewer reads the input and the output side by side per state.
type Snapshot struct {
	State string `json:"state"`
	Dir   string `json:"dir"`
}

// SnapshotData is the on-disk content of one snapshot directory.
type SnapshotData struct {
	CR       *unstructured.Unstructured
	Expected Expected
}

// Fixture indexes one flow of one operator/version/kartaName. A flow is a way the
// workload was driven (for example happy or failed). The set of Fixtures present
// under test/conformance/fixtures/ is the tested operator matrix for a Karta release.
type Fixture struct {
	SchemaVersion  int                     `json:"schemaVersion"`
	Operator       string                  `json:"operator"`
	Version        string                  `json:"version"`
	KartaName      string                  `json:"kartaName"`
	Flow           string                  `json:"flow"`
	Want           v1alpha1.ResourceStatus `json:"want,omitempty"` // the flow's terminal ResourceStatus
	KartaFile      string                  `json:"kartaFile"`      // repo-relative path to the Karta definition
	ObservedStates []string                `json:"observedStates"`
	Snapshots      []Snapshot              `json:"snapshots"`
}

// SnapshotDir is the deterministic directory name for the idx-th observed state,
// relative to the flow directory (for example "00-Running").
func SnapshotDir(idx int, state string) string {
	return fmt.Sprintf("%02d-%s", idx, state)
}

// Write persists the flow's fixture index plus one directory per snapshot holding
// cr.yaml and expected.yaml. sigs.k8s.io/yaml marshals through JSON with sorted
// keys, so re-recording an unchanged workload yields a byte-identical tree. data is
// keyed by Snapshot.Dir.
func Write(root string, f Fixture, data map[string]SnapshotData) error {
	// Clear any snapshot dirs from a previous record first, so a flow that now yields
	// fewer or differently-named snapshots leaves no stale orphan dirs behind.
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := writeYAML(filepath.Join(root, fixtureFile), f); err != nil {
		return err
	}
	for _, s := range f.Snapshots {
		d, ok := data[s.Dir]
		if !ok || d.CR == nil {
			return fmt.Errorf("no snapshot data for %s", s.Dir)
		}
		dir := filepath.Join(root, s.Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := writeYAML(filepath.Join(dir, crFile), d.CR.Object); err != nil {
			return err
		}
		if err := writeYAML(filepath.Join(dir, expectedFile), d.Expected); err != nil {
			return err
		}
	}
	return nil
}

// Load reads a flow fixture at root: the index plus each snapshot's cr.yaml and
// expected.yaml, keyed by Snapshot.Dir.
func Load(root string) (Fixture, map[string]SnapshotData, error) {
	var f Fixture
	if err := readYAML(filepath.Join(root, fixtureFile), &f); err != nil {
		return f, nil, err
	}
	data := make(map[string]SnapshotData, len(f.Snapshots))
	for _, s := range f.Snapshots {
		dir := filepath.Join(root, s.Dir)
		cr := map[string]interface{}{}
		if err := readYAML(filepath.Join(dir, crFile), &cr); err != nil {
			return f, nil, err
		}
		var exp Expected
		if err := readYAML(filepath.Join(dir, expectedFile), &exp); err != nil {
			return f, nil, err
		}
		data[s.Dir] = SnapshotData{CR: &unstructured.Unstructured{Object: cr}, Expected: exp}
	}
	return f, data, nil
}

func writeYAML(path string, v interface{}) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
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
