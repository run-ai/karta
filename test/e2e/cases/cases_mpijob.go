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
		// completed/failed: the launcher can finish before Running is ever observed, so Running is
		// Optional; and Kubeflow keeps Created (init) set, so the CR can read Initializing again for a
		// tick before the terminal - a repeat the order check tolerates whether or not a run catches it.
		{Name: "completed", WorkloadFile: "testdata/mpijob/completed.yaml", Journey: []Step{{State: initializing}, {State: running, Optional: true}, {State: initializing}, {State: completed}}},
		{Name: "failed", WorkloadFile: "testdata/mpijob/failed.yaml", Journey: []Step{{State: initializing}, {State: running, Optional: true}, {State: initializing}, {State: failed}}},
		{Name: "suspended", WorkloadFile: "testdata/mpijob/suspended.yaml", Journey: Steps(suspended)},
		{Name: "resumed", WorkloadFile: "testdata/mpijob/resumed.yaml", Journey: []Step{
			{State: suspended, Action: UnsuspendRunPolicy},
			{State: initializing},
			{State: running},
		}},
	},
}
