// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var groveCase = workloadCase{
	name:      "Grove PodCliqueSet",
	operator:  "grove",
	kartaFile: "../../docs/samples/grove-podcliqueset.yaml",
	kartaName: "grove-io-podcliqueset-v1alpha1",
	states: []namedState{
		{initializing, intBelow(1, "status", "availableReplicas")},
		{running, intAtLeast(1, "status", "availableReplicas")},
	},
	flows: []flow{
		{name: "running", workloadFile: "testdata/grove/running.yaml", journey: steps(initializing, running)},
		{name: "initializing", workloadFile: "testdata/grove/initializing.yaml", journey: steps(initializing)},
	},
	timeout: 4 * time.Minute,
}
