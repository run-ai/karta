// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var milvusCase = workloadCase{
	name:      "Milvus",
	operator:  "milvus",
	kartaFile: "../../docs/samples/milvus.yaml",
	kartaName: "milvus",
	states:    []namedState{{running, condTrue("MilvusReady")}},
	flows:     []flow{{name: "running", workloadFile: "testdata/milvus/running.yaml", journey: steps(running)}},
	timeout:   8 * time.Minute,
}
