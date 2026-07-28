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
		// completed/failed declare Initializing twice: Kubeflow keeps the Created (init) condition set for
		// the job's whole life, so if Running flips off a tick before Succeeded/Failed flips on, the CR
		// reads Initializing again before the terminal. Declaring the revisit keeps the order check strict.
		{Name: "completed", WorkloadFile: "testdata/mpijob/completed.yaml", Journey: Steps(initializing, running, initializing, completed)},
		{Name: "failed", WorkloadFile: "testdata/mpijob/failed.yaml", Journey: Steps(initializing, running, initializing, failed)},
		{Name: "suspended", WorkloadFile: "testdata/mpijob/suspended.yaml", Journey: Steps(suspended)},
		{Name: "resumed", WorkloadFile: "testdata/mpijob/resumed.yaml", Journey: []Step{
			{State: suspended, Action: UnsuspendRunPolicy},
			{State: initializing},
			{State: running},
		}},
	},
}
