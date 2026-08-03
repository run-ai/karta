// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

var batchJobCase = WorkloadCase{
	Name:      "BatchJob (built-in)",
	Operator:  "batch-job",
	KartaFile: "../../docs/catalog/batch-job-v1.yaml",
	KartaName: "batch-job-v1",
	States: []NamedState{
		{suspended, CondTrue("Suspended")},
		{initializing, IntAtLeast(1, "status", "active")},
		{running, IntAtLeast(1, "status", "ready")},
		{completed, CondTrue("Complete", "SuccessCriteriaMet")},
		{failed, CondTrue("Failed", "FailureTarget")},
		{degraded, JobDegraded()},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "cases/testdata/batch-job/running.yaml", Journey: Steps(initializing, running)},
		// the second Initializing is the active-not-ready dip as the pod terminates before Complete.
		{Name: "completed", WorkloadFile: "cases/testdata/batch-job/completed.yaml", Journey: Steps(initializing, running, initializing, completed)},
		{Name: "failed", WorkloadFile: "cases/testdata/batch-job/failed.yaml", Journey: Steps(initializing, failed)},
		{Name: "resumed", WorkloadFile: "cases/testdata/batch-job/resumed.yaml", Journey: []Step{
			{State: suspended, Action: Resume()},
			{State: initializing},
			{State: running},
			{State: initializing},
			{State: completed},
		}},
		{Name: "degraded", WorkloadFile: "cases/testdata/batch-job/degraded.yaml", Journey: Steps(initializing, running, degraded)},
		{Name: "suspended", WorkloadFile: "cases/testdata/batch-job/suspended.yaml", Journey: Steps(suspended)},
		{Name: "scaled", WorkloadFile: "cases/testdata/batch-job/scaled.yaml", Journey: []Step{
			{State: initializing, Optional: true},
			{State: running, ActionPredicate: IntEq(1, "status", "ready"), Action: ScaleParallelism(3)},
			{State: running, ActionPredicate: IntEq(3, "status", "ready"), Action: ScaleParallelism(1)},
			{State: running, ActionPredicate: IntEq(1, "status", "ready")},
		}},
	},
}
