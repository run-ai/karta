// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package e2e

import (
	"time"

	kartav1alpha1 "github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

// workloadCases is the operator matrix the e2e suite exercises. Each entry pairs
// a bundled Karta definition (docs/samples) with a real CPU-only workload driven
// to a stable state, and asserts Karta's reading of it. Adding an operator is a
// single entry plus a workload fixture under testdata/.
var workloadCases = []workloadCase{
	{
		name:         "LeaderWorkerSet",
		kartaFile:    "../../docs/samples/lws.yaml",
		kartaName:    "leaderworkerset-x-k8s-io-leaderworkerset-v1",
		workloadFile: "testdata/lws-workload.yaml",
		ready:        condTrue("Available"),
		want:         kartav1alpha1.RunningStatus,
		extracts:     []extractCheck{{component: "leader"}, {component: "worker"}},
	},
	{
		name:         "JobSet",
		kartaFile:    "../../docs/samples/jobset.yaml",
		kartaName:    "jobset-x-k8s-io-v1alpha2-jobset",
		workloadFile: "testdata/jobset-workload.yaml",
		ready:        condTrue("Completed"),
		want:         kartav1alpha1.CompletedStatus,
		extracts:     []extractCheck{{component: "replicatedjob", keys: []string{"workers"}}},
	},
	{
		name:         "RayCluster",
		kartaFile:    "../../docs/samples/raycluster.yaml",
		kartaName:    "ray-io-raycluster-v1",
		workloadFile: "testdata/raycluster-workload.yaml",
		ready:        phaseEq("ready", "status", "state"),
		want:         kartav1alpha1.RunningStatus,
		extracts:     []extractCheck{{component: "head"}, {component: "worker", keys: []string{"small"}}},
		timeout:      8 * time.Minute,
	},
	{
		name:         "PyTorchJob",
		kartaFile:    "../../docs/samples/pytorch.yaml",
		kartaName:    "kubeflow-org-pytorchjob-v1",
		workloadFile: "testdata/pytorch-workload.yaml",
		ready:        condTrue("Running"),
		want:         kartav1alpha1.RunningStatus,
		extracts:     []extractCheck{{component: "master"}},
		timeout:      4 * time.Minute,
	},
	{
		name:         "MPIJob",
		kartaFile:    "../../docs/samples/mpijob.yaml",
		kartaName:    "kubeflow-org-mpijob-v1",
		workloadFile: "testdata/mpijob-workload.yaml",
		ready:        condTrue("Succeeded"),
		want:         kartav1alpha1.CompletedStatus,
		extracts:     []extractCheck{{component: "launcher"}, {component: "worker"}},
		timeout:      3 * time.Minute,
	},
	{
		name:         "BatchJob (built-in)",
		kartaFile:    "../../docs/samples/batch-job.yaml",
		kartaName:    "batch-v1-job",
		workloadFile: "testdata/batch-job-workload.yaml",
		builtin:      true,
		ready:        condTrue("Complete"),
		want:         kartav1alpha1.CompletedStatus,
		extracts:     []extractCheck{{component: "job"}},
	},
	{
		// Real operator-driven: the Knative Serving controller (with Kourier)
		// drives the Service to Ready once its Revision pod runs and the route
		// is admitted.
		name:         "KnativeService (real operator)",
		kartaFile:    "../../docs/samples/knative-serving.yaml",
		kartaName:    "serving-knative-dev-service-v1",
		workloadFile: "testdata/knative-workload.yaml",
		ready:        condTrue("Ready"),
		want:         kartav1alpha1.RunningStatus,
		extracts:     []extractCheck{{component: "revision"}},
		timeout:      5 * time.Minute,
	},
	{
		// Real operator-driven: the KServe controller runs the InferenceService in
		// Serverless mode on Knative + Kourier and marks it Ready once the
		// predictor is serving (PredictorReady, RoutesReady, LatestDeploymentReady
		// all True, which is what the sample maps to running).
		name:         "KServe InferenceService (real operator)",
		kartaFile:    "../../docs/samples/kserve.yaml",
		kartaName:    "serving-kserve-io-inferenceservice-v1beta1",
		workloadFile: "testdata/kserve-workload.yaml",
		ready:        condTrue("Ready"),
		want:         kartav1alpha1.RunningStatus,
		extracts:     []extractCheck{{component: "predictor"}},
		timeout:      6 * time.Minute,
	},
	{
		// Real operator-driven: the milvus-operator brings up a standalone Milvus
		// (etcd + MinIO + the standalone pod) and sets MilvusReady once all four
		// readiness conditions are True, which is what the sample maps to running.
		name:         "Milvus (real operator)",
		kartaFile:    "../../docs/samples/milvus.yaml",
		kartaName:    "milvus-io-milvus-v1beta1",
		workloadFile: "testdata/milvus-workload.yaml",
		ready:        condTrue("MilvusReady"),
		want:         kartav1alpha1.RunningStatus,
		timeout:      8 * time.Minute,
	},
	{
		// Real operator-driven: the Grove operator (with kai-scheduler installed)
		// brings the PodCliqueSet's pods up; the sample maps running when
		// availableReplicas >= spec.replicas.
		name:         "Grove PodCliqueSet (real operator)",
		kartaFile:    "../../docs/samples/grove-podcliqueset.yaml",
		kartaName:    "grove-io-podcliqueset-v1alpha1",
		workloadFile: "testdata/grove-workload.yaml",
		ready:        intAtLeast(1, "status", "availableReplicas"),
		want:         kartav1alpha1.RunningStatus,
		timeout:      4 * time.Minute,
	},
	{
		// Real operator-driven: the dynamo-operator runs the DynamoGraphDeployment
		// with the mocker backend (CPU, no GPU) and reports .status.state=successful
		// once the Frontend and worker pods are ready.
		name:         "DynamoGraphDeployment (real operator; mocker backend)",
		kartaFile:    "../../docs/samples/dynamo.yaml",
		kartaName:    "nvidia-com-dynamographdeployment-v1alpha1",
		workloadFile: "testdata/dynamo-workload.yaml",
		ready:        phaseEq("successful", "status", "state"),
		want:         kartav1alpha1.RunningStatus,
		timeout:      5 * time.Minute,
	},
	{
		// Real operator-driven: the k8s-nim-operator runs a fictive CPU NIM image
		// (no GPU, no real NGC token) and drives the NIMService to state=Ready.
		name:         "NIMService (real operator; fictive CPU image)",
		kartaFile:    "../../docs/samples/nimservice.yaml",
		kartaName:    "apps-nvidia-com-nimservice-v1alpha1",
		workloadFile: "testdata/nim-workload.yaml",
		ready:        phaseEq("Ready", "status", "state"),
		want:         kartav1alpha1.RunningStatus,
		timeout:      5 * time.Minute,
	},
	{
		// Real operator-driven: KubeRay runs the RayJob to completion and reports
		// .status.jobStatus=SUCCEEDED, which the sample maps to completed.
		name:         "RayJob (real operator)",
		kartaFile:    "../../docs/samples/rayjob.yaml",
		kartaName:    "ray-io-rayjob-v1",
		workloadFile: "testdata/rayjob-workload.yaml",
		ready:        phaseEq("SUCCEEDED", "status", "jobStatus"),
		want:         kartav1alpha1.CompletedStatus,
		extracts:     []extractCheck{{component: "head"}},
		timeout:      6 * time.Minute,
	},
}
