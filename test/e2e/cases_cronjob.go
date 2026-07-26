// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

// cronjobCase drives a built-in CronJob. Ported from the older e2e-karta suite, which recording-v2 had
// dropped: initializing (enabled but never scheduled, lastScheduleTime unset), running (fired, so
// lastScheduleTime is set and no Job is active), and suspended (spec.suspend true). The definition
// reads suspend last, so a suspended CronJob (which is also never-scheduled) classifies as Suspended.
var cronjobCase = workloadCase{
	name:      "CronJob",
	operator:  "cronjob",
	kartaFile: "../../docs/catalog/batch-cronjob-v1.yaml",
	kartaName: "batch-cronjob-v1",
	states: []namedState{
		{initializing, absent("status", "lastScheduleTime")},
		{running, cronjobFired()},
		{suspended, boolTrue("spec", "suspend")},
	},
	flows: []flow{
		{name: "initializing", workloadFile: "testdata/cronjob/initializing.yaml", journey: steps(initializing)},
		{name: "running", workloadFile: "testdata/cronjob/running.yaml", journey: steps(running)},
		{name: "suspended", workloadFile: "testdata/cronjob/suspended.yaml", journey: steps(suspended)},
	},
}
