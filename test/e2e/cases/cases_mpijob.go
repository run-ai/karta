// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

var mpijobCase = WorkloadCase{
	Name:      "MPIJob",
	Operator:  "kubeflow",
	KartaFile: "../../docs/samples/mpijob.yaml",
	KartaName: "kubeflow-org-mpijob-v2beta1",
	States: []NamedState{
		{initializing, CondTrue("Created")},
		{running, CondTrue("Running")},
		{completed, CondTrue("Succeeded")},
		{failed, CondTrue("Failed")},
		{suspended, CondTrue("Suspended")},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/mpijob/running.yaml", Journey: Steps(initializing, running)},
		// completed/failed use instant launchers that may skip Running; the subsequence check tolerates
		// a skipped step, so declaring running then the terminal is enough (no backwards here).
		{Name: "completed", WorkloadFile: "testdata/mpijob/completed.yaml", Journey: Steps(initializing, running, completed)},
		{Name: "failed", WorkloadFile: "testdata/mpijob/failed.yaml", Journey: Steps(initializing, running, failed)},
		{Name: "suspended", WorkloadFile: "testdata/mpijob/suspended.yaml", Journey: Steps(suspended)},
		{Name: "resumed", WorkloadFile: "testdata/mpijob/resumed.yaml", Journey: []Step{
			{State: suspended, Action: UnsuspendRunPolicy},
			{State: initializing},
			{State: running},
		}},
	},
}
