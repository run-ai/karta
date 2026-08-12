// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// defaultTimeout bounds one flow's Run when Config.Timeout is unset.
const defaultTimeout = 3 * time.Minute

// Cluster is how the recorder reaches Kubernetes.
type Cluster struct {
	Client    client.Client
	Dynamic   dynamic.Interface
	Namespace string
}

// Config is the suite-wide setup a recorder needs: cluster access, where recordings are written, and where
// progress lines go. The suite builds one and passes it to New.
type Config struct {
	Cluster   Cluster
	OutputDir string
	Log       io.Writer     // progress and warnings; nil discards
	Timeout   time.Duration // per-flow deadline; zero uses defaultTimeout
}

// Fixture is a recording's catalog identity: where it files under the fixtures tree, and which Karta
// definition the replay checks it against.
type Fixture struct {
	Operator  string // operator key, e.g. "deployment"
	Version   string // operator version, resolved by the suite
	KartaName string // Karta definition name
	KartaFile string // repo-relative path to the Karta definition, read by the replay golden
}

// Recorder records the flows of one workload type: build and Run a Flow per case, then Save each Recording.
type Recorder struct {
	config  Config
	states  []namedState
	timeout time.Duration
}

// New starts a recorder from cfg. Build flows on it and Run them; Save writes each Recording under cfg.OutputDir.
func New(cfg Config) *Recorder {
	if cfg.OutputDir == "" {
		panic("recorder: New needs a non-empty Config.OutputDir")
	}
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	timeout := defaultTimeout
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	return &Recorder{config: cfg, timeout: timeout}
}

// AddState registers a state predicate; declare states least- to most-advanced (classify keeps the furthest match).
func (r *Recorder) AddState(name kartav1alpha1.ResourceStatus, match StateCheck) *Recorder {
	if name == "" {
		panic("recorder: AddState needs a non-empty state name")
	}
	if match == nil {
		panic("recorder: AddState needs a non-nil predicate")
	}
	r.states = append(r.states, namedState{Name: name, Match: match})
	return r
}

// SetTimeout overrides the per-flow deadline (default 3m).
func (r *Recorder) SetTimeout(d time.Duration) *Recorder {
	if d <= 0 {
		panic("recorder: SetTimeout needs a positive duration")
	}
	r.timeout = d
	return r
}

// Run creates the workload, drives it through the journey, and returns the recording of what it observed;
// pass it to Save to persist. On a flow-level failure the recording still comes back (succeeded:false).
func (f *Flow) Run(ctx context.Context) (*Recording, error) {
	if len(f.journey) == 0 {
		return nil, fmt.Errorf("flow %s declares no stops", f.name)
	}
	workload, err := f.createWorkload(ctx)
	if err != nil {
		return nil, err
	}
	defer f.deleteWorkload(ctx, workload)

	flowCtx, cancel := context.WithTimeout(ctx, f.rec.timeout)
	defer cancel()

	obs := f.observe(flowCtx, workload)
	orderErr := observedOrderErr(f.journey, obs.states(), f.want())
	out := f.buildRecording(obs, obs.failure == "" && orderErr == nil)
	if obs.failure != "" {
		return out, errors.New(obs.failure)
	}
	return out, orderErr
}

// createWorkload reads the flow's workload manifest and creates it in the recorder's namespace.
func (f *Flow) createWorkload(ctx context.Context) (*unstructured.Unstructured, error) {
	raw, err := os.ReadFile(f.manifest)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", f.manifest, err)
	}
	workload := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(raw, workload); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", f.manifest, err)
	}
	workload.SetNamespace(f.rec.config.Cluster.Namespace)
	if err := f.client().Create(ctx, workload); err != nil {
		return nil, fmt.Errorf("create workload for %s: %w", f.name, err)
	}
	return workload, nil
}

// deleteWorkload removes the workload once the flow is done. It runs on a fresh, bounded context so cleanup
// still happens after the flow's own context was cancelled; a failed delete is logged, not fatal.
func (f *Flow) deleteWorkload(ctx context.Context, workload *unstructured.Unstructured) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := f.client().Delete(ctx, workload); err != nil && !apierrors.IsNotFound(err) {
		fmt.Fprintf(f.log(), "cleanup: delete %s/%s failed: %v\n", workload.GetNamespace(), workload.GetName(), err)
	}
}

// observe watches the workload and records every distinct CR until the flow finishes, acting on the
// journey's checkpoints as their states are reached. Its failure is set if the terminal state was not met.
func (f *Flow) observe(ctx context.Context, workload *unstructured.Unstructured) *observation {
	o := &observation{flow: f, workload: workload, pending: checkpoints(f.journey)}

	// A workload already at its terminal state when Create returns never produces a watch event (the watch
	// replays only newer resourceVersions), so record it straight from the create response.
	if isWorkloadObserved(workload) && o.hasReachedTerminal(classify(workload, f.rec.states)) {
		o.record(ctx, workload)
		return o
	}
	o.follow(ctx)
	return o
}

// buildRecording assembles the observed run into a Recording: a STATE event per distinct CR, and an ACTION
// event after a state where the flow performed one. Save fills in the catalog fields (Operator, Version, ...).
func (f *Flow) buildRecording(obs *observation, succeeded bool) *Recording {
	out := &Recording{
		SchemaVersion: schemaVersion,
		Flow:          f.name,
		Want:          string(f.want()),
		Succeeded:     succeeded,
	}
	for _, snap := range obs.snapshots {
		out.Events = append(out.Events, Event{Kind: EventState, State: string(snap.state), StaleObservedGeneration: snap.staleObservedGeneration, Object: significantFields(snap.cr)})
		if snap.action != nil {
			out.Events = append(out.Events, Event{Kind: EventAction, Action: snap.action})
		}
	}
	return out
}

// Save stamps fx onto rec and writes it under <OutputDir>/<operator>/<version>/<kartaName>/<flow>.yaml,
// returning the path. A failed run is saved too (succeeded:false); a nil recording is a no-op.
func (r *Recorder) Save(fx Fixture, rec *Recording) (string, error) {
	if rec == nil {
		return "", nil
	}
	if fx.Operator == "" || fx.Version == "" || fx.KartaName == "" || fx.KartaFile == "" {
		panic("recorder: Save needs a non-empty Fixture Operator, Version, KartaName, and KartaFile")
	}
	rec.Operator = fx.Operator
	rec.Version = fx.Version
	rec.KartaName = fx.KartaName
	rec.KartaFile = strings.TrimPrefix(fx.KartaFile, "../../")
	rec.Path = recordingPath(r.config.OutputDir, *rec)
	if err := writeRecording(rec.Path, *rec); err != nil {
		return "", fmt.Errorf("write recording %s: %w", rec.Path, err)
	}
	fmt.Fprintf(r.config.Log, "recorded %s/%s/%s/%s.yaml (%d events %v)\n",
		fx.Operator, fx.Version, fx.KartaName, rec.Flow, len(rec.Events), rec.states())
	return rec.Path, nil
}

// checkpoints are the journey stops the recorder must reach and act on in order: those carrying an action or
// an action predicate. Plain stops between them are recorded as they pass but are not checkpoints.
func checkpoints(journey []journeyStep) []journeyStep {
	var out []journeyStep
	for _, step := range journey {
		if step.Action != nil || step.ActionPredicate != nil {
			out = append(out, step)
		}
	}
	return out
}
