// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

var batchJobCase = workloadCase{
	name:      "BatchJob (built-in)",
	operator:  "batch-job",
	kartaFile: "../../docs/catalog/batch-job-v1.yaml",
	kartaName: "batch-job-v1",
	// States least to most advanced. Initializing and Running read the Job's active/ready counts;
	// Completed, Failed and Suspended read conditions. Suspended is first so a lingering Suspended
	// condition never masks real progress after a resume.
	states: []namedState{
		{suspended, condTrue("Suspended")},
		{initializing, intAtLeast(1, "status", "active")},
		{running, intAtLeast(1, "status", "ready")},
		{completed, condTrue("Complete")},
		{failed, condTrue("Failed")},
		{degraded, jobDegraded()},
	},
	flows: []flow{
		{name: "running", workloadFile: "testdata/batch-job/running.yaml", journey: steps(initializing, running)},
		// completed and resumed declare Initializing twice: a Job reports active-not-ready for a tick
		// as its pod terminates, so Running dips back to Initializing before the terminal. Declaring
		// the revisit keeps the order check strict rather than waiving it.
		{name: "completed", workloadFile: "testdata/batch-job/completed.yaml", journey: steps(initializing, running, initializing, completed)},
		{name: "failed", workloadFile: "testdata/batch-job/failed.yaml", journey: steps(initializing, failed)},
		// Created already suspended, then resumed: the operator holds at Suspended until the action
		// clears spec.suspend, then a pod runs to completion. Initializing is declared twice for the
		// terminating-pod dip (see above).
		{name: "resumed", workloadFile: "testdata/batch-job/resumed.yaml", journey: []step{
			{state: suspended, action: unsuspend},
			{state: initializing},
			{state: running},
			{state: initializing},
			{state: completed},
		}},
		// Indexed Job: index 0 runs to completion while index 1 sleeps, so one pod succeeded and one
		// stays ready - a settled partial read as Degraded.
		{name: "degraded", workloadFile: "testdata/batch-job/degraded.yaml", journey: steps(initializing, running, degraded)},
		{name: "suspended", workloadFile: "testdata/batch-job/suspended.yaml", journey: steps(suspended)},
		// Parallelism scale: sleeping pods, no completions, so it stays Running while spec.parallelism
		// drives status.ready 1 -> 3 -> 1.
		{name: "scaled", workloadFile: "testdata/batch-job/scaled.yaml", journey: []step{
			{state: running, settle: intEq(1, "status", "ready"), action: scaleParallelism(3)},
			{state: running, settle: intEq(3, "status", "ready"), action: scaleParallelism(1)},
			{state: running, settle: intEq(1, "status", "ready")},
		}},
	},
}
