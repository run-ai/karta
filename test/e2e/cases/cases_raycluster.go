// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

var rayClusterCase = WorkloadCase{
	Name:      "RayCluster",
	Operator:  "kuberay",
	KartaFile: "../../docs/samples/raycluster.yaml",
	KartaName: "ray-io-raycluster-v1",
	States: []NamedState{
		{running, PhaseEq("ready", "status", "state")},
		{suspended, RaySuspended()},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/raycluster/running.yaml", Journey: Steps(running)},
		{Name: "suspended", WorkloadFile: "testdata/raycluster/suspended.yaml", Journey: Steps(suspended)},
		{Name: "resumed", WorkloadFile: "testdata/raycluster/resumed.yaml", Journey: []Step{
			{State: suspended, Action: Unsuspend},
			{State: running},
		}},
	},
	Timeout: 8 * time.Minute,
}
