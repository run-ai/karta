// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var rayJobCase = workloadCase{
	name:      "RayJob",
	operator:  "kuberay",
	kartaFile: "../../docs/samples/rayjob.yaml",
	kartaName: "ray-io-rayjob-v1",
	states:    []namedState{{completed, phaseEq("SUCCEEDED", "status", "jobStatus")}},
	flows:     []flow{{name: "completed", workloadFile: "testdata/rayjob/completed.yaml", journey: steps(completed)}},
	timeout:   6 * time.Minute,
}
