// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

var podCase = WorkloadCase{
	Name:      "Pod (built-in)",
	Operator:  "pod",
	KartaFile: "../../docs/catalog/core-pod-v1.yaml",
	KartaName: "core-pod-v1",
	States: []NamedState{
		{initializing, PhaseEq("Pending", "status", "phase")},
		{running, PhaseEq("Running", "status", "phase")},
		{completed, PhaseEq("Succeeded", "status", "phase")},
		{failed, PhaseEq("Failed", "status", "phase")},
	},
	Flows: []Flow{
		{Name: "happy", WorkloadFile: "testdata/pod/happy.yaml", Journey: Steps(initializing, running)},
		{Name: "completed", WorkloadFile: "testdata/pod/completed.yaml", Journey: Steps(initializing, running, completed)},
		{Name: "failed", WorkloadFile: "testdata/pod/failed.yaml", Journey: Steps(initializing, running, failed)},
		{Name: "initializing", WorkloadFile: "testdata/pod/initializing.yaml", Journey: Steps(initializing)},
	},
}
