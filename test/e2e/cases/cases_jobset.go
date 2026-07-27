// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

var jobsetCase = WorkloadCase{
	Name:      "JobSet",
	Operator:  "jobset",
	KartaFile: "../../docs/samples/jobset.yaml",
	KartaName: "jobset-x-k8s-io-v1alpha2-jobset",
	// States least to most advanced. Initializing and Running read the replicatedJobs counts
	// (active-not-ready vs ready); Completed, Failed and Suspended read conditions. Suspended is
	// first so a lingering Suspended condition never masks real progress after a resume.
	States: []NamedState{
		{suspended, CondTrue("Suspended")},
		{initializing, JobsetInitializing()},
		{running, JobsetRunning()},
		{completed, CondTrue("Completed")},
		{failed, CondTrue("Failed")},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/jobset/running.yaml", Journey: Steps(initializing, running)},
		// completed and resumed declare Initializing twice: a JobSet reports a replicatedJob
		// active-not-ready for a tick as its pod terminates, so Running dips back to Initializing
		// before the terminal. Declaring the revisit keeps the order check strict rather than waiving it.
		{Name: "completed", WorkloadFile: "testdata/jobset/completed.yaml", Journey: Steps(initializing, running, initializing, completed)},
		{Name: "failed", WorkloadFile: "testdata/jobset/failed.yaml", Journey: Steps(initializing, failed)},
		// Created already suspended, then resumed: the operator holds at Suspended until the action
		// clears spec.suspend, then a replicated Job runs to completion. Initializing is declared twice
		// for the terminating-pod dip (see above).
		{Name: "resumed", WorkloadFile: "testdata/jobset/resumed.yaml", Journey: []Step{
			{State: suspended, Action: Unsuspend},
			{State: initializing},
			{State: running},
			{State: initializing},
			{State: completed},
		}},
		{Name: "suspended", WorkloadFile: "testdata/jobset/suspended.yaml", Journey: Steps(suspended)},
	},
}
