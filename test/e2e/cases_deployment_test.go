// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var deploymentCase = workloadCase{
	name:      "Deployment (built-in)",
	operator:  "deployment",
	kartaFile: "../../docs/samples/deployment.yaml",
	kartaName: "apps-deployment-v1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/deployment/happy.yaml",
			states:       []namedState{{"Running", condTrue("Available")}},
			want:         kartav1alpha1.RunningStatus,
		},
		{
			name:         "failed",
			workloadFile: "testdata/deployment/failed.yaml",
			states:       []namedState{{"Failed", condFalse("Progressing")}},
			want:         kartav1alpha1.FailedStatus,
		},
	},
}
