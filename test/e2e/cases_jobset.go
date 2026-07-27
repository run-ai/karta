// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

var jobsetCase = workloadCase{
	name:      "JobSet",
	operator:  "jobset",
	kartaFile: "../../docs/samples/jobset.yaml",
	kartaName: "jobset-x-k8s-io-v1alpha2-jobset",
	// States least to most advanced. Initializing and Running read the replicatedJobs counts
	// (active-not-ready vs ready); Completed, Failed and Suspended read conditions. Suspended is
	// first so a lingering Suspended condition never masks real progress after a resume.
	states: []namedState{
		{suspended, condTrue("Suspended")},
		{initializing, jobsetInitializing()},
		{running, jobsetRunning()},
		{completed, condTrue("Completed")},
		{failed, condTrue("Failed")},
	},
	flows: []flow{
		{name: "running", workloadFile: "testdata/jobset/running.yaml", journey: steps(initializing, running)},
		// completed and resumed declare Initializing twice: a JobSet reports a replicatedJob
		// active-not-ready for a tick as its pod terminates, so Running dips back to Initializing
		// before the terminal. Declaring the revisit keeps the order check strict rather than waiving it.
		{name: "completed", workloadFile: "testdata/jobset/completed.yaml", journey: steps(initializing, running, initializing, completed)},
		{name: "failed", workloadFile: "testdata/jobset/failed.yaml", journey: steps(initializing, failed)},
		// Created already suspended, then resumed: the operator holds at Suspended until the action
		// clears spec.suspend, then a replicated Job runs to completion. Initializing is declared twice
		// for the terminating-pod dip (see above).
		{name: "resumed", workloadFile: "testdata/jobset/resumed.yaml", journey: []step{
			{state: suspended, action: unsuspend},
			{state: initializing},
			{state: running},
			{state: initializing},
			{state: completed},
		}},
		{name: "suspended", workloadFile: "testdata/jobset/suspended.yaml", journey: steps(suspended)},
	},
}
