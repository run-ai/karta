// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

// statefulSetCase drives a built-in StatefulSet through a scale up then down. Its definition reads
// Running only when readyReplicas == replicas at the current revision, so the scaled flow captures
// the CR at each settled count (1, 3, 1) - mid-scale it reads Degraded or Initializing and is
// skipped. The golden diffs each reading; the extracted scale changes.
var statefulSetCase = WorkloadCase{
	Name:      "StatefulSet",
	Operator:  "statefulset",
	KartaFile: "../../docs/catalog/apps-statefulset-v1.yaml",
	KartaName: "apps-statefulset-v1",
	States: []NamedState{
		{initializing, ReplicasInitializing()},
		{running, FullyAvailable()},
		{degraded, ReplicasDegraded()},
	},
	Flows: []Flow{
		{Name: "scaled", WorkloadFile: "testdata/statefulset/running.yaml", Journey: []Step{
			{State: running, Settle: ReplicasReady(1), Action: ScaleReplicas(3)},
			{State: running, Settle: ReplicasReady(3), Action: ScaleReplicas(1)},
			{State: running, Settle: ReplicasReady(1)},
		}},
		// 3 replicas + hostname antiAffinity on 2 nodes: one stays pending, so readyReplicas settles
		// below replicas with updatedReplicas == replicas, read as Degraded.
		{Name: "degraded", WorkloadFile: "testdata/statefulset/degraded.yaml", Journey: Steps(initializing, degraded)},
	},
	Timeout: 3 * time.Minute,
}
