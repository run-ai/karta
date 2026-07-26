// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

// deploymentCase drives a built-in Deployment through a scale up then down. The scaled flow lists
// Running three times, each gated by a settle on the ready count, so the recorder captures the CR at
// 1, 3, then 1 replicas. Karta reads Running at every step; the golden diffs the extraction, which
// changes with the replica count.
var deploymentCase = workloadCase{
	name:      "Deployment",
	operator:  "deployment",
	kartaFile: "../../docs/catalog/apps-deployment-v1.yaml",
	kartaName: "apps-deployment-v1",
	states: []namedState{
		{running, fullyAvailable()},
		{failed, condFalse("Progressing")},
	},
	flows: []flow{
		{name: "scaled", workloadFile: "testdata/deployment/running.yaml", journey: []step{
			{state: running, settle: replicasReady(1), action: scaleReplicas(3)},
			{state: running, settle: replicasReady(3), action: scaleReplicas(1)},
			{state: running, settle: replicasReady(1)},
		}},
		// Pinned to a nonexistent node with a 10s progress deadline: the controller sets
		// Progressing=False/ProgressDeadlineExceeded, read as Failed.
		{name: "failed", workloadFile: "testdata/deployment/failed.yaml", journey: steps(failed)},
	},
	timeout: 3 * time.Minute,
}
