// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var nimCase = workloadCase{
	name:      "NIMService",
	operator:  "nim",
	kartaFile: "../../docs/samples/nimservice.yaml",
	kartaName: "apps-nvidia-com-nimservice-v1alpha1",
	states: []namedState{
		{initializing, phaseAny([]string{"NotReady", "Pending"}, "status", "state")},
		{running, phaseEq("Ready", "status", "state")},
	},
	flows: []flow{
		{name: "running", workloadFile: "testdata/nim/running.yaml", journey: steps(initializing, running)},
		{name: "initializing", workloadFile: "testdata/nim/initializing.yaml", journey: steps(initializing)},
	},
	timeout: 5 * time.Minute,
}
