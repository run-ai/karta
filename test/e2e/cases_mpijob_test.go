// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var mpijobCase = workloadCase{
	name:      "MPIJob",
	operator:  "kubeflow",
	kartaFile: "../../docs/samples/mpijob.yaml",
	kartaName: "kubeflow-org-mpijob-v2beta1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/mpijob/happy.yaml",
			states:       []namedState{{"Initializing", condTrue("Created")}, {"Running", condTrue("Running")}, {"Completed", condTrue("Succeeded")}},
			want:         kartav1alpha1.CompletedStatus,
		},
		{
			name:         "failed",
			workloadFile: "testdata/mpijob/failed.yaml",
			states:       []namedState{{"Running", condTrue("Running")}, {"Failed", condTrue("Failed")}},
			want:         kartav1alpha1.FailedStatus,
		},
		{
			name:         "suspended",
			workloadFile: "testdata/mpijob/suspended.yaml",
			states:       []namedState{{"Suspended", condTrue("Suspended")}},
			want:         kartav1alpha1.SuspendedStatus,
		},
		{
			name:         "running",
			workloadFile: "testdata/mpijob/running.yaml",
			states:       []namedState{{"Running", condTrue("Running")}},
			want:         kartav1alpha1.RunningStatus,
		},
	},
	extracts: []extractCheck{{component: "launcher"}, {component: "worker"}},
	timeout:  3 * time.Minute,
}
