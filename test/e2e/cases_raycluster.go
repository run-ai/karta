// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var rayClusterCase = workloadCase{
	name:      "RayCluster",
	operator:  "kuberay",
	kartaFile: "../../docs/samples/raycluster.yaml",
	kartaName: "ray-io-raycluster-v1",
	states: []namedState{
		{running, phaseEq("ready", "status", "state")},
		{suspended, raySuspended()},
	},
	flows: []flow{
		{name: "running", workloadFile: "testdata/raycluster/running.yaml", journey: steps(running)},
		{name: "suspended", workloadFile: "testdata/raycluster/suspended.yaml", journey: steps(suspended)},
	},
	timeout: 8 * time.Minute,
}
