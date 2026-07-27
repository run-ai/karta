// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

var lwsCase = WorkloadCase{
	Name:      "LeaderWorkerSet",
	Operator:  "lws",
	KartaFile: "../../docs/samples/lws.yaml",
	KartaName: "leaderworkerset-x-k8s-io-leaderworkerset-v1",
	// Karta reads a LeaderWorkerSet as running when every current replica is ready and updated (it
	// compares to status.replicas, which the controller sets to the desired count at once). Initializing
	// mirrors the definition: Progressing while not yet Available.
	States: []NamedState{
		{initializing, AllOf(CondTrue("Progressing"), CondFalse("Available"))},
		{running, ReplicasSettled()},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/lws/running.yaml", Journey: Steps(initializing, running)},
		{Name: "scaled", WorkloadFile: "testdata/lws/scaled.yaml", Journey: []Step{
			{State: running, Settle: ReplicasReady(1), Action: ScaleReplicas(2)},
			{State: running, Settle: ReplicasReady(2), Action: ScaleReplicas(1)},
			{State: running, Settle: ReplicasReady(1)},
		}},
	},
}
