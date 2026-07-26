// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var dynamoCase = workloadCase{
	name:      "DynamoGraphDeployment",
	operator:  "dynamo",
	kartaFile: "../../docs/samples/dynamo.yaml",
	kartaName: "nvidia-com-dynamographdeployment-v1alpha1",
	states:    []namedState{{running, phaseEq("successful", "status", "state")}},
	flows:     []flow{{name: "running", workloadFile: "testdata/dynamo/running.yaml", journey: steps(running)}},
	timeout:   5 * time.Minute,
}
