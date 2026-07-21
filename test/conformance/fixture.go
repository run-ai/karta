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
// v3: fixtures are YAML (not JSON) and Components carries per-instance scale.
const SchemaVersion = 3

const (
	fixtureFile  = "fixture.yaml"
	crFile       = "cr.yaml"
	expectedFile = "expected.yaml"
)

// Scale mirrors the replica information Karta extracts for an instance. It is a
// stable, spec-derived projection, unlike the pod template it accompanies, so it is
// safe to keep in a golden fixture.
type Scale struct {
	Replicas    *int32 `json:"replicas,omitempty"`
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
}

// ComponentReading is what Karta extracts for one component of a CR: the sorted
// instance keys and, per instance, its scale. It deliberately omits the pod specs
// and mutated metadata inside ExtractedInstance, which churn run to run and would
// make golden diffs noisy.
type ComponentReading struct {
	Instances []string         `json:"instances"`
	Scale     map[string]Scale `json:"scale,omitempty"`
}

// Expected is the projection of what the Karta library reads for one CR snapshot.
// It keeps only the stable signal: the matched statuses, the phase, and the sorted
// instance keys plus scale per component.
type Expected struct {
	MatchedStatuses []v1alpha1.ResourceStatus   `json:"matchedStatuses"`
	Phase           *string                     `json:"phase,omitempty"`
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
	KartaFile      string                  `json:"kartaFile"`      // repo-relative path to the docs/samples definition
	ObservedStates []string                `json:"observedStates"`
	Snapshots      []Snapshot              `json:"snapshots"`
}

// CollapseConsecutive returns the sequence of distinct states in order, dropping
// equal consecutive entries. The recorder's ObservedStates is exactly this collapse
// of the per-snapshot states, so golden replay can tie the snapshot dirs to it.
func CollapseConsecutive(states []string) []string {
	var out []string
	for _, s := range states {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
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
		d := data[s.Dir]
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
