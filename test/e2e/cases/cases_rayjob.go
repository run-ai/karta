// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

var rayJobCase = WorkloadCase{
	Name:      "RayJob",
	Operator:  "kuberay",
	KartaFile: "../../docs/samples/rayjob.yaml",
	KartaName: "ray-io-rayjob-v1",
	States: []NamedState{
		{initializing, PhaseEq("PENDING", "status", "jobStatus")},
		{running, PhaseEq("RUNNING", "status", "jobStatus")},
		{completed, PhaseEq("SUCCEEDED", "status", "jobStatus")},
		{failed, PhaseEq("FAILED", "status", "jobStatus")},
		{suspended, PhaseEq("Suspended", "status", "jobDeploymentStatus")},
	},
	Flows: []Flow{
		// jobStatus jumps between PENDING/RUNNING/SUCCEEDED/FAILED, and a fast job can skip the
		// intermediate phases, so Initializing and (for the terminal flows) Running are Optional - they
		// are declared so they are accepted when observed, but the flow does not require them.
		{Name: "running", WorkloadFile: "testdata/rayjob/running.yaml", Journey: []Step{
			{State: initializing, Optional: true},
			{State: running},
		}},
		{Name: "completed", WorkloadFile: "testdata/rayjob/completed.yaml", Journey: []Step{
			{State: initializing, Optional: true},
			{State: running, Optional: true},
			{State: completed},
		}},
		{Name: "failed", WorkloadFile: "testdata/rayjob/failed.yaml", Journey: []Step{
			{State: initializing, Optional: true},
			{State: running, Optional: true},
			{State: failed},
		}},
		{Name: "suspended", WorkloadFile: "testdata/rayjob/suspended.yaml", Journey: Steps(suspended)},
		{Name: "resumed", WorkloadFile: "testdata/rayjob/resumed.yaml", Journey: []Step{
			{State: suspended, Action: Unsuspend},
			{State: initializing, Optional: true},
			{State: running},
		}},
	},
	Timeout: 6 * time.Minute,
}
