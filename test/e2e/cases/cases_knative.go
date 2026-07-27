// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

var knativeCase = WorkloadCase{
	Name:      "KnativeService",
	Operator:  "knative",
	KartaFile: "../../docs/samples/knative-serving.yaml",
	KartaName: "serving-knative-dev-service-v1",
	States:    []NamedState{{running, CondTrue("Ready")}},
	Flows:     []Flow{{Name: "running", WorkloadFile: "testdata/knative/running.yaml", Journey: Steps(running)}},
	Timeout:   5 * time.Minute,
}
