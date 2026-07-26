// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var kserveCase = workloadCase{
	name:      "KServe InferenceService",
	operator:  "kserve",
	kartaFile: "../../docs/samples/kserve.yaml",
	kartaName: "serving-kserve-io-inferenceservice-v1beta1",
	states:    []namedState{{running, condTrue("Ready")}},
	flows:     []flow{{name: "running", workloadFile: "testdata/kserve/running.yaml", journey: steps(running)}},
	extracts:  []extractCheck{{component: "predictor"}},
	timeout:   6 * time.Minute,
}
