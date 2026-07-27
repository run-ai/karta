// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

var batchJobCase = WorkloadCase{
	Name:      "BatchJob (built-in)",
	Operator:  "batch-job",
	KartaFile: "../../docs/catalog/batch-job-v1.yaml",
	KartaName: "batch-job-v1",
	// States least to most advanced. Initializing and Running read the Job's active/ready counts;
	// Completed, Failed and Suspended read conditions. Suspended is first so a lingering Suspended
	// condition never masks real progress after a resume.
	States: []NamedState{
		{suspended, CondTrue("Suspended")},
		{initializing, IntAtLeast(1, "status", "active")},
		{running, IntAtLeast(1, "status", "ready")},
		{completed, CondTrue("Complete")},
		{failed, CondTrue("Failed")},
		{degraded, JobDegraded()},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/batch-job/running.yaml", Journey: Steps(initializing, running)},
		// completed and resumed declare Initializing twice: a Job reports active-not-ready for a tick
		// as its pod terminates, so Running dips back to Initializing before the terminal. Declaring
		// the revisit keeps the order check strict rather than waiving it.
		{Name: "completed", WorkloadFile: "testdata/batch-job/completed.yaml", Journey: Steps(initializing, running, initializing, completed)},
		{Name: "failed", WorkloadFile: "testdata/batch-job/failed.yaml", Journey: Steps(initializing, failed)},
		// Created already suspended, then resumed: the operator holds at Suspended until the action
		// clears spec.suspend, then a pod runs to completion. Initializing is declared twice for the
		// terminating-pod dip (see above).
		{Name: "resumed", WorkloadFile: "testdata/batch-job/resumed.yaml", Journey: []Step{
			{State: suspended, Action: Unsuspend},
			{State: initializing},
			{State: running},
			{State: initializing},
			{State: completed},
		}},
		// Indexed Job: index 0 runs to completion while index 1 sleeps, so one pod succeeded and one
		// stays ready - a settled partial read as Degraded.
		{Name: "degraded", WorkloadFile: "testdata/batch-job/degraded.yaml", Journey: Steps(initializing, running, degraded)},
		{Name: "suspended", WorkloadFile: "testdata/batch-job/suspended.yaml", Journey: Steps(suspended)},
		// Parallelism scale: sleeping pods, no completions, so it stays Running while spec.parallelism
		// drives status.ready 1 -> 3 -> 1.
		{Name: "scaled", WorkloadFile: "testdata/batch-job/scaled.yaml", Journey: []Step{
			{State: running, Settle: IntEq(1, "status", "ready"), Action: ScaleParallelism(3)},
			{State: running, Settle: IntEq(3, "status", "ready"), Action: ScaleParallelism(1)},
			{State: running, Settle: IntEq(1, "status", "ready")},
		}},
	},
}
