// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var cronjobCase = workloadCase{
	name:      "CronJob (built-in)",
	operator:  "cronjob",
	kartaFile: "../../docs/samples/cronjob.yaml",
	kartaName: "batch-cronjob-v1",
	flows: []flow{
		{
			name:         "initializing",
			workloadFile: "testdata/cronjob/initializing.yaml",
			states:       []namedState{{"Initializing", absent("status", "lastScheduleTime")}},
			want:         kartav1alpha1.InitializingStatus,
		},
		{
			name:         "suspended",
			workloadFile: "testdata/cronjob/suspended.yaml",
			states:       []namedState{{"Suspended", boolTrue("spec", "suspend")}},
			want:         kartav1alpha1.SuspendedStatus,
		},
		{
			name:         "running",
			workloadFile: "testdata/cronjob/running.yaml",
			states:       []namedState{{"Running", cronjobFired()}},
			want:         kartav1alpha1.RunningStatus,
		},
	},
}
