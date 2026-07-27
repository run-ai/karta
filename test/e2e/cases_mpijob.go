// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

var mpijobCase = workloadCase{
	name:      "MPIJob",
	operator:  "kubeflow",
	kartaFile: "../../docs/samples/mpijob.yaml",
	kartaName: "kubeflow-org-mpijob-v2beta1",
	states: []namedState{
		{running, condTrue("Running")},
		{completed, condTrue("Succeeded")},
		{failed, condTrue("Failed")},
		{suspended, condTrue("Suspended")},
	},
	flows: []flow{
		{name: "running", workloadFile: "testdata/mpijob/running.yaml", journey: steps(running)},
		// completed/failed use instant launchers that may skip Running; the subsequence check tolerates
		// a skipped step, so declaring running then the terminal is enough (no backwards here).
		{name: "completed", workloadFile: "testdata/mpijob/completed.yaml", journey: steps(running, completed)},
		{name: "failed", workloadFile: "testdata/mpijob/failed.yaml", journey: steps(running, failed)},
		{name: "suspended", workloadFile: "testdata/mpijob/suspended.yaml", journey: steps(suspended)},
		{name: "resumed", workloadFile: "testdata/mpijob/resumed.yaml", journey: []step{
			{state: suspended, action: unsuspendRunPolicy},
			{state: running},
		}},
	},
}
