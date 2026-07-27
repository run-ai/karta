// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

var pytorchCase = WorkloadCase{
	Name:      "PyTorchJob",
	Operator:  "kubeflow",
	KartaFile: "../../docs/samples/pytorch.yaml",
	KartaName: "kubeflow-org-pytorchjob-v1",
	States: []NamedState{
		{running, CondTrue("Running")},
		{completed, CondTrue("Succeeded")},
		{failed, CondTrue("Failed")},
		{suspended, CondTrue("Suspended")},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/pytorch/running.yaml", Journey: Steps(running)},
		{Name: "completed", WorkloadFile: "testdata/pytorch/completed.yaml", Journey: Steps(running, completed)},
		{Name: "failed", WorkloadFile: "testdata/pytorch/failed.yaml", Journey: Steps(running, failed)},
		{Name: "suspended", WorkloadFile: "testdata/pytorch/suspended.yaml", Journey: Steps(suspended)},
		{Name: "resumed", WorkloadFile: "testdata/pytorch/resumed.yaml", Journey: []Step{
			{State: suspended, Action: UnsuspendRunPolicy},
			{State: running},
		}},
	},
	Timeout: 4 * time.Minute,
}
