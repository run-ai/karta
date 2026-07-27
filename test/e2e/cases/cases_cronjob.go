// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

// cronjobCase drives a built-in CronJob. Ported from the older e2e-karta suite, which recording-v2 had
// dropped: initializing (enabled but never scheduled, lastScheduleTime unset), running (fired, so
// lastScheduleTime is set and no Job is active), and suspended (spec.suspend true). The definition
// reads suspend last, so a suspended CronJob (which is also never-scheduled) classifies as Suspended.
var cronjobCase = WorkloadCase{
	Name:      "CronJob",
	Operator:  "cronjob",
	KartaFile: "../../docs/catalog/batch-cronjob-v1.yaml",
	KartaName: "batch-cronjob-v1",
	States: []NamedState{
		{initializing, Absent("status", "lastScheduleTime")},
		{running, CronjobFired()},
		{suspended, BoolTrue("spec", "suspend")},
	},
	Flows: []Flow{
		{Name: "initializing", WorkloadFile: "testdata/cronjob/initializing.yaml", Journey: Steps(initializing)},
		{Name: "running", WorkloadFile: "testdata/cronjob/running.yaml", Journey: Steps(running)},
		{Name: "suspended", WorkloadFile: "testdata/cronjob/suspended.yaml", Journey: Steps(suspended)},
	},
}
