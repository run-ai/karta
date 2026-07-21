// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var pytorchCase = workloadCase{
	name:      "PyTorchJob",
	operator:  "kubeflow",
	kartaFile: "../../docs/samples/pytorch.yaml",
	kartaName: "kubeflow-org-pytorchjob-v1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/pytorch/happy.yaml",
			states:       []namedState{{"Initializing", condTrue("Created")}, {"Running", condTrue("Running")}},
			want:         kartav1alpha1.RunningStatus,
		},
		{
			name:         "failed",
			workloadFile: "testdata/pytorch/failed.yaml",
			states:       []namedState{{"Running", condTrue("Running")}, {"Failed", condTrue("Failed")}},
			want:         kartav1alpha1.FailedStatus,
		},
		{
			name:         "completed",
			workloadFile: "testdata/pytorch/completed.yaml",
			states:       []namedState{{"Running", condTrue("Running")}, {"Completed", condTrue("Succeeded")}},
			want:         kartav1alpha1.CompletedStatus,
		},
		{
			name:         "suspended",
			workloadFile: "testdata/pytorch/suspended.yaml",
			states:       []namedState{{"Suspended", condTrue("Suspended")}},
			want:         kartav1alpha1.SuspendedStatus,
		},
	},
	extracts: []extractCheck{{component: "master"}},
	timeout:  4 * time.Minute,
}
