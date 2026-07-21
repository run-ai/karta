// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var dynamoCase = workloadCase{
	// Real operator-driven: the dynamo-operator runs the DynamoGraphDeployment
	// with the mocker backend (CPU, no GPU) and reports .status.state=successful
	// once the Frontend and worker pods are ready.
	name:      "DynamoGraphDeployment (real operator; mocker backend)",
	operator:  "dynamo",
	kartaFile: "../../docs/samples/dynamo.yaml",
	kartaName: "nvidia-com-dynamographdeployment-v1alpha1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/dynamo/happy.yaml",
			states:       []namedState{{"Initializing", phaseAny([]string{"pending", "initializing"}, "status", "state")}, {"Running", phaseEq("successful", "status", "state")}},
			want:         kartav1alpha1.RunningStatus,
		},
	},
	timeout: 5 * time.Minute,
}
