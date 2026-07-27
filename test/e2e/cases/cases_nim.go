// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

var nimCase = WorkloadCase{
	Name:      "NIMService",
	Operator:  "nim",
	KartaFile: "../../docs/samples/nimservice.yaml",
	KartaName: "apps-nvidia-com-nimservice-v1alpha1",
	States: []NamedState{
		{initializing, PhaseAny([]string{"NotReady", "Pending"}, "status", "state")},
		{running, PhaseEq("Ready", "status", "state")},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/nim/running.yaml", Journey: Steps(initializing, running)},
		{Name: "initializing", WorkloadFile: "testdata/nim/initializing.yaml", Journey: Steps(initializing)},
	},
	Timeout: 5 * time.Minute,
}
