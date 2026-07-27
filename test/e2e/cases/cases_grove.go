// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

var groveCase = WorkloadCase{
	Name:      "Grove PodCliqueSet",
	Operator:  "grove",
	KartaFile: "../../docs/samples/grove-podcliqueset.yaml",
	KartaName: "grove-io-podcliqueset-v1alpha1",
	States: []NamedState{
		{initializing, ReplicasComingUp()},
		{running, AllReplicasAvailable()},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/grove/running.yaml", Journey: Steps(initializing, running)},
		{Name: "initializing", WorkloadFile: "testdata/grove/initializing.yaml", Journey: Steps(initializing)},
		{Name: "scaled", WorkloadFile: "testdata/grove/scaled.yaml", Journey: []Step{
			{State: initializing, Optional: true},
			{State: running, Settle: IntEq(1, "status", "availableReplicas"), Action: ScaleReplicas(2)},
			{State: initializing, Optional: true},
			{State: running, Settle: IntEq(2, "status", "availableReplicas"), Action: ScaleReplicas(1)},
			{State: initializing, Optional: true},
			{State: running, Settle: IntEq(1, "status", "availableReplicas")},
		}},
	},
	Timeout: 4 * time.Minute,
}
