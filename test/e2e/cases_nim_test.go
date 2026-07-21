// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var nimCase = workloadCase{
	// Real operator-driven: the k8s-nim-operator runs a fictive CPU NIM image
	// (no GPU, no real NGC token) and drives the NIMService to state=Ready.
	name:      "NIMService (real operator; fictive CPU image)",
	operator:  "nim",
	kartaFile: "../../docs/samples/nimservice.yaml",
	kartaName: "apps-nvidia-com-nimservice-v1alpha1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/nim/happy.yaml",
			states:       []namedState{{"Initializing", phaseEq("NotReady", "status", "state")}, {"Running", phaseEq("Ready", "status", "state")}},
			want:         kartav1alpha1.RunningStatus,
		},
		{
			name:         "initializing",
			workloadFile: "testdata/nim/initializing.yaml",
			states:       []namedState{{"Initializing", phaseEq("NotReady", "status", "state")}},
			want:         kartav1alpha1.InitializingStatus,
		},
	},
	timeout: 5 * time.Minute,
}
