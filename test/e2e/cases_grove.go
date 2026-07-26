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
		{name: "scaled", workloadFile: "testdata/grove/scaled.yaml", journey: []step{
			{state: running, settle: intEq(1, "status", "availableReplicas"), action: scaleReplicas(2)},
			{state: running, settle: intEq(2, "status", "availableReplicas"), action: scaleReplicas(1)},
			{state: running, settle: intEq(1, "status", "availableReplicas")},
		}},
	},
	timeout: 4 * time.Minute,
}
