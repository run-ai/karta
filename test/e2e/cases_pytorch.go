// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var pytorchCase = workloadCase{
	name:      "PyTorchJob",
	operator:  "kubeflow",
	kartaFile: "../../docs/samples/pytorch.yaml",
	kartaName: "kubeflow-org-pytorchjob-v1",
	states: []namedState{
		{running, condTrue("Running")},
		{completed, condTrue("Succeeded")},
		{failed, condTrue("Failed")},
		{suspended, condTrue("Suspended")},
	},
	flows: []flow{
		{name: "running", workloadFile: "testdata/pytorch/running.yaml", journey: steps(running)},
		{name: "completed", workloadFile: "testdata/pytorch/completed.yaml", journey: steps(running, completed)},
		{name: "failed", workloadFile: "testdata/pytorch/failed.yaml", journey: steps(running, failed)},
		{name: "suspended", workloadFile: "testdata/pytorch/suspended.yaml", journey: steps(suspended)},
	},
	timeout: 4 * time.Minute,
}
