// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

const (
	initializing = kartav1alpha1.InitializingStatus
	running      = kartav1alpha1.RunningStatus
	completed    = kartav1alpha1.CompletedStatus
	failed       = kartav1alpha1.FailedStatus
)

var podCase = workloadCase{
	name:      "Pod (built-in)",
	operator:  "pod",
	kartaFile: "../../docs/catalog/core-pod-v1.yaml",
	kartaName: "core-pod-v1",
	states: []namedState{
		{initializing, phaseEq("Pending", "status", "phase")},
		{running, phaseEq("Running", "status", "phase")},
		{completed, phaseEq("Succeeded", "status", "phase")},
		{failed, phaseEq("Failed", "status", "phase")},
	},
	flows: []flow{
		{name: "happy", workloadFile: "testdata/pod/happy.yaml", journey: steps(initializing, running)},
		{name: "completed", workloadFile: "testdata/pod/completed.yaml", journey: steps(initializing, running, completed)},
		{name: "failed", workloadFile: "testdata/pod/failed.yaml", journey: steps(initializing, running, failed)},
		{name: "initializing", workloadFile: "testdata/pod/initializing.yaml", journey: steps(initializing)},
	},
}

var allCases = []workloadCase{podCase}
