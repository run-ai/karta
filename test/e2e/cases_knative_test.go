// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var knativeCase = workloadCase{
	// Real operator-driven: the Knative Serving controller (with Kourier)
	// drives the Service to Ready once its Revision pod runs and the route
	// is admitted.
	name:      "KnativeService (real operator)",
	operator:  "knative",
	kartaFile: "../../docs/samples/knative-serving.yaml",
	kartaName: "serving-knative-dev-service-v1",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/knative/happy.yaml",
			states:       []namedState{{"Running", condTrue("Ready")}},
			want:         kartav1alpha1.RunningStatus,
		},
	},
	extracts: []extractCheck{{component: "revision"}},
	timeout:  5 * time.Minute,
}
