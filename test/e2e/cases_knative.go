// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var knativeCase = workloadCase{
	name:      "KnativeService",
	operator:  "knative",
	kartaFile: "../../docs/samples/knative-serving.yaml",
	kartaName: "serving-knative-dev-service-v1",
	states:    []namedState{{running, condTrue("Ready")}},
	flows:     []flow{{name: "running", workloadFile: "testdata/knative/running.yaml", journey: steps(running)}},
	extracts:  []extractCheck{{component: "revision"}},
	timeout:   5 * time.Minute,
}
