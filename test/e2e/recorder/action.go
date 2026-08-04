// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package recorder

// ActionType names the mutations a flow can fire to drive a transition the operator will not make itself.
type ActionType string

const (
	ActionResume ActionType = "Resume"
	ActionScale  ActionType = "Scale"
)

// Action is a merge-patch of Type applied to the workload at a step. The recorder sends the patch and
// records the request and response. Flows build these with the action helpers in the flows package.
type Action struct {
	Type  ActionType
	Patch []byte
}
