// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

import "time"

var kserveCase = WorkloadCase{
	Name:      "KServe InferenceService",
	Operator:  "kserve",
	KartaFile: "../../docs/samples/kserve.yaml",
	KartaName: "serving-kserve-io-inferenceservice-v1beta1",
	States: []NamedState{
		{running, CondTrue("Ready")},
		{failed, CondsFalse("PredictorReady", "PredictorConfigurationReady", "RoutesReady")},
	},
	Flows: []Flow{
		{Name: "running", WorkloadFile: "testdata/kserve/running.yaml", Journey: Steps(running)},
		// Custom predictor container with a nonexistent-registry image: the Revision fails, driving
		// PredictorReady/PredictorConfigurationReady/RoutesReady all False -> Failed.
		{Name: "failed", WorkloadFile: "testdata/kserve/failed.yaml", Journey: Steps(failed)},
	},
	Timeout: 6 * time.Minute,
}
