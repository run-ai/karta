// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var lwsCase = workloadCase{
	name:      "LeaderWorkerSet",
	operator:  "lws",
	kartaFile: "../../docs/samples/lws.yaml",
	kartaName: "leaderworkerset-x-k8s-io-leaderworkerset-v1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/lws/happy.yaml",
			// lws leaves status.readyReplicas absent (not 0) while its pods start and its
			// Available/Progressing conditions churn, so an Initializing snapshot is not
			// cleanly capturable here; kept at the settled Running state.
			states: []namedState{{"Running", condTrue("Available")}},
			want:   kartav1alpha1.RunningStatus,
		},
	},
	extracts: []extractCheck{{component: "leader"}, {component: "worker"}},
}
