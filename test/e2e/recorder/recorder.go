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
	Log       io.Writer // progress and warnings; nil discards
}

// Recorder records the flows of one workload type: build and Run a Flow per case.
type Recorder struct {
	cluster   Cluster
	outputDir string
	log       io.Writer
	operator  string
	version   string
	kartaName string
	kartaFile string
	states    []namedState
	timeout   time.Duration
}

// New starts a recorder from cfg; version is stamped on the recording and kartaFile is recorded as metadata
// for the replay golden (neither path is read here).
func New(cfg Config, operator, version, kartaName, kartaFile string) *Recorder {
	if operator == "" || version == "" || kartaName == "" || kartaFile == "" {
		panic("recorder: New needs a non-empty operator, version, kartaName, and kartaFile")
	}
	if cfg.OutputDir == "" {
		panic("recorder: New needs a non-empty Config.OutputDir")
	}
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	return &Recorder{
		cluster:   cfg.Cluster,
		outputDir: cfg.OutputDir,
		log:       cfg.Log,
		operator:  operator,
		version:   version,
		kartaName: kartaName,
		kartaFile: kartaFile,
		timeout:   3 * time.Minute,
	}
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

// Run creates the workload, drives it through the journey, and writes the recording. On a flow-level failure
// the recording is still written (succeeded:false) for triage.
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
	out, err := f.write(obs, obs.failure == "" && orderErr == nil)
	if err != nil {
		return nil, err
	}
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
	workload.SetNamespace(f.rec.cluster.Namespace)
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

// observe watches the workload and records every distinct settled CR until the flow finishes, firing the
// journey's checkpoints as their states are reached. Its failure is set if the terminal state was not met.
func (f *Flow) observe(ctx context.Context, workload *unstructured.Unstructured) *observation {
	o := &observation{flow: f, workload: workload, pending: checkpoints(f.journey)}

	// A workload already at its terminal state when Create returns never fires a watch event (the watch
	// replays only newer resourceVersions), so record it straight from the create response.
	if isStatusSettled(workload) && o.hasReachedTerminal(classify(workload, f.rec.states)) {
		o.record(ctx, workload)
		return o
	}
	o.follow(ctx)
	return o
}

// write persists the run under <outputDir>/<operator>/<version>/<kartaName>/<flow>.yaml and returns it: a
// STATE event per distinct CR, and an ACTION event after a state where the flow fired one.
func (f *Flow) write(obs *observation, succeeded bool) (*Recording, error) {
	out := Recording{
		SchemaVersion: schemaVersion,
		Operator:      f.rec.operator,
		Version:       f.rec.version,
		KartaName:     f.rec.kartaName,
		Flow:          f.name,
		Want:          string(f.want()),
		Succeeded:     succeeded,
		KartaFile:     strings.TrimPrefix(f.rec.kartaFile, "../../"),
	}
	for _, snap := range obs.snapshots {
		out.Events = append(out.Events, Event{Kind: EventState, State: string(snap.state), Object: significantFields(snap.cr)})
		if snap.action != nil {
			out.Events = append(out.Events, Event{Kind: EventAction, Action: snap.action})
		}
	}

	out.Path = recordingPath(f.rec.outputDir, out)
	if err := writeRecording(out.Path, out); err != nil {
		return nil, fmt.Errorf("write recording %s: %w", out.Path, err)
	}
	fmt.Fprintf(f.log(), "recorded %s/%s/%s/%s.yaml (%d events %v)\n",
		f.rec.operator, f.rec.version, f.rec.kartaName, f.name, len(out.Events), obs.states())
	return &out, nil
}

// checkpoints are the journey stops the recorder must reach and fire in order: those carrying an action or
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
