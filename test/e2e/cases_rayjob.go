// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var rayJobCase = workloadCase{
	name:      "RayJob",
	operator:  "kuberay",
	kartaFile: "../../docs/samples/rayjob.yaml",
	kartaName: "ray-io-rayjob-v1",
	states: []namedState{
		{running, phaseEq("RUNNING", "status", "jobStatus")},
		{completed, phaseEq("SUCCEEDED", "status", "jobStatus")},
		{failed, phaseEq("FAILED", "status", "jobStatus")},
		{suspended, phaseEq("Suspended", "status", "jobDeploymentStatus")},
	},
	flows: []flow{
		// running uses a long-sleeping entrypoint; completed/failed pass through RUNNING first, so
		// their journeys declare it (their recorded fixtures still hold only the terminal).
		{name: "running", workloadFile: "testdata/rayjob/running.yaml", journey: steps(running)},
		{name: "completed", workloadFile: "testdata/rayjob/completed.yaml", journey: steps(running, completed)},
		{name: "failed", workloadFile: "testdata/rayjob/failed.yaml", journey: steps(running, failed)},
		{name: "suspended", workloadFile: "testdata/rayjob/suspended.yaml", journey: steps(suspended)},
		{name: "resumed", workloadFile: "testdata/rayjob/resumed.yaml", journey: []step{
			{state: suspended, action: unsuspend},
			{state: running},
		}},
	},
	timeout: 6 * time.Minute,
}
