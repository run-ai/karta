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
		// completed and resumed set mayGoBackwards: a JobSet reports a replicatedJob active-not-ready
		// for a tick as its pod terminates, which reads as Initializing again after Running, so the
		// observed order is not strictly forward. The check then only requires each observed state is
		// in the journey and the terminal is last.
		{name: "completed", workloadFile: "testdata/jobset/completed.yaml", mayGoBackwards: true, journey: steps(initializing, running, completed)},
		{name: "failed", workloadFile: "testdata/jobset/failed.yaml", journey: steps(initializing, failed)},
		// Created already suspended, then resumed: the operator holds at Suspended until the action
		// clears spec.suspend, then a replicated Job runs to completion.
		{name: "resumed", workloadFile: "testdata/jobset/resumed.yaml", mayGoBackwards: true, journey: []step{
			{state: suspended, action: unsuspend},
			{state: initializing},
			{state: running},
			{state: completed},
		}},
		{name: "suspended", workloadFile: "testdata/jobset/suspended.yaml", journey: steps(suspended)},
	},
}
