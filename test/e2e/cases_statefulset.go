// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

// statefulSetCase drives a built-in StatefulSet through a scale up then down. Its definition reads
// Running only when readyReplicas == replicas at the current revision, so the scaled flow captures
// the CR at each settled count (1, 3, 1) - mid-scale it reads Degraded or Initializing and is
// skipped. The golden diffs each reading; the extracted scale changes.
var statefulSetCase = workloadCase{
	name:      "StatefulSet",
	operator:  "statefulset",
	kartaFile: "../../docs/catalog/apps-statefulset-v1.yaml",
	kartaName: "apps-statefulset-v1",
	states: []namedState{
		{running, fullyAvailable()},
		{degraded, replicasDegraded()},
	},
	flows: []flow{
		{name: "scaled", workloadFile: "testdata/statefulset/running.yaml", journey: []step{
			{state: running, settle: replicasReady(1), action: scaleReplicas(3)},
			{state: running, settle: replicasReady(3), action: scaleReplicas(1)},
			{state: running, settle: replicasReady(1)},
		}},
		// 3 replicas + hostname antiAffinity on 2 nodes: one stays pending, so readyReplicas settles
		// below replicas with updatedReplicas == replicas, read as Degraded.
		{name: "degraded", workloadFile: "testdata/statefulset/degraded.yaml", journey: steps(degraded)},
	},
	timeout: 3 * time.Minute,
}
