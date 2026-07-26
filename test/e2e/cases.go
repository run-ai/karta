// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"

// State names, from the Karta ResourceStatus enum. A case's registry maps a workload's own fields
// to these; a journey step names one.
const (
	initializing = kartav1alpha1.InitializingStatus
	running      = kartav1alpha1.RunningStatus
	completed    = kartav1alpha1.CompletedStatus
	failed       = kartav1alpha1.FailedStatus
	suspended    = kartav1alpha1.SuspendedStatus
	degraded     = kartav1alpha1.DegradedStatus
)

// allCases is every workload case the suite runs: online to record, offline to replay. Add a case
// here after defining it in cases_<type>.go.
var allCases = []workloadCase{
	// built-in kinds (no operator install)
	podCase,
	batchJobCase,
	deploymentCase,
	statefulSetCase,
	// operator-driven
	jobsetCase,
	lwsCase,
	rayClusterCase,
	rayJobCase,
	pytorchCase,
	mpijobCase,
	knativeCase,
	kserveCase,
	milvusCase,
	groveCase,
	dynamoCase,
	nimCase,
}
