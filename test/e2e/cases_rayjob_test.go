// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var rayjobCase = workloadCase{
	// Real operator-driven: KubeRay runs the RayJob to completion and reports
	// .status.jobStatus=SUCCEEDED, which the sample maps to completed.
	name:      "RayJob (real operator)",
	operator:  "kuberay",
	kartaFile: "../../docs/samples/rayjob.yaml",
	kartaName: "ray-io-rayjob-v1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/rayjob/happy.yaml",
			states:       []namedState{{"Running", phaseEq("RUNNING", "status", "jobStatus")}, {"Completed", phaseEq("SUCCEEDED", "status", "jobStatus")}},
			want:         kartav1alpha1.CompletedStatus,
		},
		{
			name:         "failed",
			workloadFile: "testdata/rayjob/failed.yaml",
			states:       []namedState{{"Running", phaseEq("RUNNING", "status", "jobStatus")}, {"Failed", phaseEq("FAILED", "status", "jobStatus")}},
			want:         kartav1alpha1.FailedStatus,
		},
		{
			name:         "suspended",
			workloadFile: "testdata/rayjob/suspended.yaml",
			states:       []namedState{{"Suspended", phaseEq("Suspended", "status", "jobDeploymentStatus")}},
			want:         kartav1alpha1.SuspendedStatus,
		},
	},
	extracts: []extractCheck{{component: "head"}},
	timeout:  6 * time.Minute,
}
