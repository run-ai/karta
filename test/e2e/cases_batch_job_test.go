// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var batchJobCase = workloadCase{
	name:      "BatchJob (built-in)",
	operator:  "batch-job",
	kartaFile: "../../docs/samples/batch-job.yaml",
	kartaName: "batch-v1-job",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/batch-job/happy.yaml",
			states: []namedState{
				{"Initializing", intAtLeast(1, "status", "active")},
				{"Running", intAtLeast(1, "status", "ready")},
				{"Completed", condTrue("Complete")},
			},
			want: kartav1alpha1.CompletedStatus,
		},
		{
			name:         "failed",
			workloadFile: "testdata/batch-job/failed.yaml",
			states: []namedState{
				{"Running", intAtLeast(1, "status", "active")},
				{"Failed", condTrue("Failed")},
			},
			want: kartav1alpha1.FailedStatus,
		},
		{
			name:         "suspended",
			workloadFile: "testdata/batch-job/suspended.yaml",
			states:       []namedState{{"Suspended", condTrue("Suspended")}},
			want:         kartav1alpha1.SuspendedStatus,
		},
		{
			name:         "resumed",
			workloadFile: "testdata/batch-job/resumed.yaml",
			states: []namedState{
				{"Suspended", condTrue("Suspended")},
				{"Running", intAtLeast(1, "status", "active")},
				{"Completed", condTrue("Complete")},
			},
			actions: map[string]stateAction{"Suspended": unsuspend},
			want:    kartav1alpha1.CompletedStatus,
		},
		{
			name:         "degraded",
			workloadFile: "testdata/batch-job/degraded.yaml",
			states:       []namedState{{"Running", intAtLeast(1, "status", "active")}, {"Degraded", jobDegraded()}},
			want:         kartav1alpha1.DegradedStatus,
		},
	},
	extracts: []extractCheck{{component: "job"}},
}
