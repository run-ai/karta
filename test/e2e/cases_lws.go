// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

var lwsCase = workloadCase{
	name:      "LeaderWorkerSet",
	operator:  "lws",
	kartaFile: "../../docs/samples/lws.yaml",
	kartaName: "leaderworkerset-x-k8s-io-leaderworkerset-v1",
	states: []namedState{{running, condTrue("Available")}},
	flows: []flow{
		{name: "running", workloadFile: "testdata/lws/running.yaml", journey: steps(running)},
		{name: "scaled", workloadFile: "testdata/lws/scaled.yaml", journey: []step{
			{state: running, settle: replicasReady(1), action: scaleReplicas(2)},
			{state: running, settle: replicasReady(2), action: scaleReplicas(1)},
			{state: running, settle: replicasReady(1)},
		}},
	},
}
