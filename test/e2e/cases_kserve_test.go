// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var kserveCase = workloadCase{
	// Real operator-driven: the KServe controller runs the InferenceService in
	// Serverless mode on Knative + Kourier and marks it Ready once the
	// predictor is serving (PredictorReady, RoutesReady, LatestDeploymentReady
	// all True, which is what the sample maps to running).
	name:      "KServe InferenceService (real operator)",
	operator:  "kserve",
	kartaFile: "../../docs/samples/kserve.yaml",
	kartaName: "serving-kserve-io-inferenceservice-v1beta1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/kserve/happy.yaml",
			states:       []namedState{{"Running", condTrue("Ready")}},
			want:         kartav1alpha1.RunningStatus,
		},
		{
			name:         "failed",
			workloadFile: "testdata/kserve/failed.yaml",
			states:       []namedState{{"Failed", condsFalse("PredictorReady", "PredictorConfigurationReady", "RoutesReady")}},
			want:         kartav1alpha1.FailedStatus,
		},
	},
	extracts: []extractCheck{{component: "predictor"}},
	timeout:  6 * time.Minute,
}
