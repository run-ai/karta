// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import "time"

var pytorchCase = workloadCase{
	name:      "PyTorchJob",
	operator:  "kubeflow",
	kartaFile: "../../docs/samples/pytorch.yaml",
	kartaName: "kubeflow-org-pytorchjob-v1",
	states:    []namedState{{running, condTrue("Running")}},
	flows:     []flow{{name: "running", workloadFile: "testdata/pytorch/running.yaml", journey: steps(running)}},
	extracts:  []extractCheck{{component: "master"}},
	timeout:   4 * time.Minute,
}
