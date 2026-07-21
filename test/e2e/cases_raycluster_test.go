// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var rayclusterCase = workloadCase{
	name:      "RayCluster",
	operator:  "kuberay",
	kartaFile: "../../docs/samples/raycluster.yaml",
	kartaName: "ray-io-raycluster-v1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/raycluster/happy.yaml",
			states:       []namedState{{"Running", phaseEq("ready", "status", "state")}},
			want:         kartav1alpha1.RunningStatus,
		},
		{
			name:         "suspended",
			workloadFile: "testdata/raycluster/suspended.yaml",
			states:       []namedState{{"Suspended", raySuspended()}},
			want:         kartav1alpha1.SuspendedStatus,
		},
	},
	extracts: []extractCheck{{component: "head"}, {component: "worker", keys: []string{"small"}}},
	timeout:  8 * time.Minute,
}
