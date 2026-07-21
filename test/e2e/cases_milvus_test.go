// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

var milvusCase = workloadCase{
	// Real operator-driven: the milvus-operator brings up a standalone Milvus
	// (etcd + MinIO + the standalone pod) and sets MilvusReady once its
	// readiness conditions are True; .status.status is then Healthy, which the
	// sample maps to running.
	name:      "Milvus (real operator)",
	operator:  "milvus",
	kartaFile: "../../docs/samples/milvus.yaml",
	kartaName: "milvus",
	flows: []flow{
		{
			name:         "happy",
			workloadFile: "testdata/milvus/happy.yaml",
			// Milvus passes through .status.status=Pending (initializing) before Healthy
			// (running), but its etcd + standalone are slow and flaky to become healthy on
			// a loaded kind cluster (repeated 12-minute timeouts stuck at Pending, even
			// with fresh PVCs), so only the settled Running state is recorded here. On a
			// fresh cluster the Initializing snapshot is reachable; add it back then.
			states: []namedState{{"Running", condTrue("MilvusReady")}},
			want:   kartav1alpha1.RunningStatus,
		},
	},
	timeout: 12 * time.Minute,
}
