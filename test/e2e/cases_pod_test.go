// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var podCase = workloadCase{
	name:      "Pod (built-in)",
	operator:  "pod",
	kartaFile: "../../docs/samples/pod.yaml",
	kartaName: "core-pod-v1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/pod/happy.yaml",
			states:       []namedState{{"Initializing", phaseEq("Pending", "status", "phase")}, {"Running", phaseEq("Running", "status", "phase")}},
			want:         kartav1alpha1.RunningStatus,
		},
		{
			name:         "completed",
			workloadFile: "testdata/pod/completed.yaml",
			states:       []namedState{{"Initializing", phaseEq("Pending", "status", "phase")}, {"Running", phaseEq("Running", "status", "phase")}, {"Completed", phaseEq("Succeeded", "status", "phase")}},
			want:         kartav1alpha1.CompletedStatus,
		},
		{
			name:         "failed",
			workloadFile: "testdata/pod/failed.yaml",
			states:       []namedState{{"Initializing", phaseEq("Pending", "status", "phase")}, {"Running", phaseEq("Running", "status", "phase")}, {"Failed", phaseEq("Failed", "status", "phase")}},
			want:         kartav1alpha1.FailedStatus,
		},
		{
			name:         "initializing",
			workloadFile: "testdata/pod/initializing.yaml",
			states:       []namedState{{"Initializing", phaseEq("Pending", "status", "phase")}},
			want:         kartav1alpha1.InitializingStatus,
		},
	},
}
