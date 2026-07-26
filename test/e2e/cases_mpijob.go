// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

var mpijobCase = workloadCase{
	name:      "MPIJob",
	operator:  "kubeflow",
	kartaFile: "../../docs/samples/mpijob.yaml",
	kartaName: "kubeflow-org-mpijob-v2beta1",
	states: []namedState{
		{completed, condTrue("Succeeded")},
		{failed, condTrue("Failed")},
		{suspended, condTrue("Suspended")},
	},
	flows: []flow{
		{name: "completed", workloadFile: "testdata/mpijob/completed.yaml", journey: steps(completed)},
		{name: "failed", workloadFile: "testdata/mpijob/failed.yaml", journey: steps(failed)},
		{name: "suspended", workloadFile: "testdata/mpijob/suspended.yaml", journey: steps(suspended)},
	},
}
