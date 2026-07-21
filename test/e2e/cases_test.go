// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

// workloadCases aggregates every per-type case, each defined in its own
// cases_<type>_test.go file. Adding an operator is a new file plus one line here.
// The slice is built at package-var-init time (before the Describe tree is built),
// so the per-type vars it names are initialised first.
var workloadCases = []workloadCase{
	lwsCase,
	jobsetCase,
	rayclusterCase,
	pytorchCase,
	mpijobCase,
	podCase,
	batchJobCase,
	deploymentCase,
	statefulsetCase,
	cronjobCase,
	knativeCase,
	kserveCase,
	milvusCase,
	groveCase,
	dynamoCase,
	nimCase,
	rayjobCase,
}
