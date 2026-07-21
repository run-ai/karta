// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var jobsetCase = workloadCase{
	name:      "JobSet",
	operator:  "jobset",
	kartaFile: "../../docs/samples/jobset.yaml",
	kartaName: "jobset-x-k8s-io-v1alpha2-jobset",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/jobset/happy.yaml",
			states:       []namedState{{"Initializing", jobsetInitializing()}, {"Running", jobsetRunning()}, {"Completed", condTrue("Completed")}},
			want:         kartav1alpha1.CompletedStatus,
		},
		{
			name:         "failed",
			workloadFile: "testdata/jobset/failed.yaml",
			states:       []namedState{{"Running", jobsetRunning()}, {"Failed", condTrue("Failed")}},
			want:         kartav1alpha1.FailedStatus,
		},
		{
			name:         "suspended",
			workloadFile: "testdata/jobset/suspended.yaml",
			states:       []namedState{{"Suspended", condTrue("Suspended")}},
			want:         kartav1alpha1.SuspendedStatus,
		},
		{
			name:         "running",
			workloadFile: "testdata/jobset/running.yaml",
			states:       []namedState{{"Running", jobsetRunning()}},
			want:         kartav1alpha1.RunningStatus,
		},
	},
	extracts: []extractCheck{{component: "replicatedjob", keys: []string{"workers"}}},
}
