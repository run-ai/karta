// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var statefulsetCase = workloadCase{
	name:      "StatefulSet (built-in)",
	operator:  "statefulset",
	kartaFile: "../../docs/samples/statefulset.yaml",
	kartaName: "apps-statefulset-v1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/statefulset/happy.yaml",
			states:       []namedState{{"Running", intAtLeast(1, "status", "readyReplicas")}},
			want:         kartav1alpha1.RunningStatus,
		},
		{
			name:         "degraded",
			workloadFile: "testdata/statefulset/degraded.yaml",
			states:       []namedState{{"Degraded", replicasDegraded()}},
			want:         kartav1alpha1.DegradedStatus,
		},
	},
}
