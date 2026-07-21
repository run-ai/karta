// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var groveCase = workloadCase{
	// Real operator-driven: the Grove operator (with kai-scheduler installed)
	// brings the PodCliqueSet's pods up; the sample maps running when
	// availableReplicas >= spec.replicas.
	name:      "Grove PodCliqueSet (real operator)",
	operator:  "grove",
	kartaFile: "../../docs/samples/grove-podcliqueset.yaml",
	kartaName: "grove-io-podcliqueset-v1alpha1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/grove/happy.yaml",
			states:       []namedState{{"Initializing", intBelow(1, "status", "availableReplicas")}, {"Running", intAtLeast(1, "status", "availableReplicas")}},
			want:         kartav1alpha1.RunningStatus,
		},
	},
	timeout: 4 * time.Minute,
}
