// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

var lwsCase = WorkloadCase{
	Name:      "LeaderWorkerSet",
	Operator:  "lws",
	KartaFile: "../../docs/samples/lws.yaml",
	KartaName: "leaderworkerset-x-k8s-io-leaderworkerset-v1",
	States:    []NamedState{{running, CondTrue("Available")}},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/lws/running.yaml", Journey: Steps(running)},
		{Name: "scaled", WorkloadFile: "testdata/lws/scaled.yaml", Journey: []Step{
			{State: running, Settle: ReplicasReady(1), Action: ScaleReplicas(2)},
			{State: running, Settle: ReplicasReady(2), Action: ScaleReplicas(1)},
			{State: running, Settle: ReplicasReady(1)},
		}},
	},
}
