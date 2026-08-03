// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "fmt"

// ActionType names the mutations a flow can fire to drive a transition the operator will not make itself.
type ActionType string

const (
	ActionResume ActionType = "Resume"
	Scale        ActionType = "Scale"
)

// Action is a merge-patch of Type applied to the workload at a step. The recorder sends the patch and
// records the request and response.
type Action struct {
	Type  ActionType
	Patch []byte
}

// Resume clears spec.suspend so a suspended workload resumes.
func Resume() *Action {
	return &Action{Type: ActionResume, Patch: []byte(`{"spec":{"suspend":false}}`)}
}

// ScaleParallelism sets a batch Job's spec.parallelism.
func ScaleParallelism(n int) *Action {
	return &Action{Type: Scale, Patch: []byte(fmt.Sprintf(`{"spec":{"parallelism":%d}}`, n))}
}

// ScaleReplicas sets spec.replicas, for any workload with a scalar replica count (Deployment,
// StatefulSet, LeaderWorkerSet, Grove, ...).
func ScaleReplicas(n int) *Action {
	return &Action{Type: Scale, Patch: []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, n))}
}

// ResumeRunPolicy clears spec.runPolicy.suspend, where Kubeflow jobs (PyTorchJob, MPIJob) keep the suspend
// flag, unlike the top-level spec.suspend that Resume patches.
func ResumeRunPolicy() *Action {
	return &Action{Type: ActionResume, Patch: []byte(`{"spec":{"runPolicy":{"suspend":false}}}`)}
}
