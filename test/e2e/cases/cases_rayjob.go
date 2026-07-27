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
		// running uses a long-sleeping entrypoint; completed/failed pass through RUNNING first, so
		// their journeys declare it (their recorded fixtures still hold only the terminal).
		{Name: "running", WorkloadFile: "testdata/rayjob/running.yaml", Journey: Steps(initializing, running)},
		{Name: "completed", WorkloadFile: "testdata/rayjob/completed.yaml", Journey: Steps(initializing, running, completed)},
		{Name: "failed", WorkloadFile: "testdata/rayjob/failed.yaml", Journey: Steps(initializing, running, failed)},
		{Name: "suspended", WorkloadFile: "testdata/rayjob/suspended.yaml", Journey: Steps(suspended)},
		{Name: "resumed", WorkloadFile: "testdata/rayjob/resumed.yaml", Journey: []Step{
			{State: suspended, Action: Unsuspend},
			{State: initializing},
			{State: running},
		}},
	},
	Timeout: 6 * time.Minute,
}
