// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cases

// All is every workload case the suite runs: online to record, offline to replay. Add a case
// here after defining it in cases_<type>.go.
var All = []WorkloadCase{
	// built-in kinds (no operator install)
	podCase,
	batchJobCase,
	deploymentCase,
	statefulSetCase,
	cronjobCase,
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
