// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

var dynamoCase = WorkloadCase{
	Name:      "DynamoGraphDeployment",
	Operator:  "dynamo",
	KartaFile: "../../docs/samples/dynamo.yaml",
	KartaName: "nvidia-com-dynamographdeployment-v1alpha1",
	States: []NamedState{
		{initializing, PhaseAny([]string{"initializing", "pending"}, "status", "state")},
		{running, PhaseEq("successful", "status", "state")},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/dynamo/running.yaml", Journey: Steps(initializing, running)},
		{Name: "initializing", WorkloadFile: "testdata/dynamo/initializing.yaml", Journey: Steps(initializing)},
	},
	Timeout: 5 * time.Minute,
}
