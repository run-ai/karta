// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

var milvusCase = WorkloadCase{
	Name:      "Milvus",
	Operator:  "milvus",
	KartaFile: "../../docs/samples/milvus.yaml",
	KartaName: "milvus",
	States: []NamedState{
		{initializing, PhaseEq("Pending", "status", "status")},
		{running, CondTrue("MilvusReady")},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/milvus/running.yaml", Journey: Steps(initializing, running)},
		{Name: "initializing", WorkloadFile: "testdata/milvus/initializing.yaml", Journey: Steps(initializing)},
	},
	Timeout: 8 * time.Minute,
}
