// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

var mpijobCase = workloadCase{
	name:      "MPIJob",
	operator:  "kubeflow",
	kartaFile: "../../docs/samples/mpijob.yaml",
	kartaName: "kubeflow-org-mpijob-v2beta1",
	states:    []namedState{{completed, condTrue("Succeeded")}},
	flows:     []flow{{name: "completed", workloadFile: "testdata/mpijob/completed.yaml", journey: steps(completed)}},
	extracts:  []extractCheck{{component: "launcher"}, {component: "worker"}},
}
