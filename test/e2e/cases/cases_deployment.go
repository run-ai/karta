// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

// deploymentCase drives a built-in Deployment through a scale up then down. The scaled flow lists
// Running three times, each gated by a settle on the ready count, so the recorder captures the CR at
// 1, 3, then 1 replicas. Karta reads Running at every step; the golden diffs the extraction, which
// changes with the replica count.
var deploymentCase = WorkloadCase{
	Name:      "Deployment",
	Operator:  "deployment",
	KartaFile: "../../docs/catalog/apps-deployment-v1.yaml",
	KartaName: "apps-deployment-v1",
	States: []NamedState{
		{initializing, AllOf(CondTrue("Progressing"), CondFalse("Available"))},
		{running, CondReason("Progressing", "NewReplicaSetAvailable")},
		{failed, CondFalse("Progressing")},
	},
	Flows: []Flow{
		{Name: "scaled", WorkloadFile: "testdata/deployment/running.yaml", Journey: []Step{
			{State: running, Settle: ReplicasReady(1), Action: ScaleReplicas(3)},
			{State: running, Settle: ReplicasReady(3), Action: ScaleReplicas(1)},
			{State: running, Settle: ReplicasReady(1)},
		}},
		// Bad image, no progress deadline: Progressing stays True with Available False, read as Initializing.
		{Name: "initializing", WorkloadFile: "testdata/deployment/initializing.yaml", Journey: Steps(initializing)},
		// Pinned to a nonexistent node with a 10s progress deadline: the controller sets
		// Progressing=False/ProgressDeadlineExceeded, read as Failed. It passes through Initializing first.
		{Name: "failed", WorkloadFile: "testdata/deployment/failed.yaml", Journey: Steps(initializing, failed)},
	},
	Timeout: 3 * time.Minute,
}
