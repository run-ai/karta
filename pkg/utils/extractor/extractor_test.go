package extractor_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/runai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/runai/kai-bolt/pkg/utils/extractor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Extractor", func() {
	var (
		pytorchRID *v1alpha1.ResourceInterpretationDefinition
		jobSetRID  *v1alpha1.ResourceInterpretationDefinition
		dynamoRID  *v1alpha1.ResourceInterpretationDefinition
		pytorchJob client.Object
		jobSet     client.Object
		dynamo     client.Object
	)

	BeforeEach(func() {
		// Get project root directory
		projectRoot, err := getProjectRoot()
		Expect(err).ToNot(HaveOccurred())

		// Load PyTorch RID
		pytorchRIDPath := filepath.Join(projectRoot, "docs", "examples", "pytorch.yaml")
		pytorchRID, err = loadRIDFromFile(pytorchRIDPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(pytorchRID).ToNot(BeNil())

		// Load JobSet RID
		jobSetRIDPath := filepath.Join(projectRoot, "docs", "examples", "jobset.yaml")
		jobSetRID, err = loadRIDFromFile(jobSetRIDPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(jobSetRID).ToNot(BeNil())

		// Load Dynamo RID
		dynamoRIDPath := filepath.Join(projectRoot, "docs", "examples", "dynamo.yaml")
		dynamoRID, err = loadRIDFromFile(dynamoRIDPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(dynamoRID).ToNot(BeNil())

		// Create PyTorchJob object
		pytorchJob = createPyTorchJobObject()

		// Create JobSet object (array of specs)
		jobSet = createJobSetObject()

		// Create Dynamo object (map of specs)
		dynamo = createDynamoObject()
	})

	Context("PyTorch Job Extractor", func() {
		var pytorchExtractor extractor.Extractor

		BeforeEach(func() {
			pytorchExtractor = extractor.NewComponentExtractor(pytorchRID, pytorchJob)
			Expect(pytorchExtractor).ToNot(BeNil())
		})

		It("should retrieve PyTorch components", func() {
			// Test master component
			masterComponent, err := pytorchExtractor.GetComponent("master")
			Expect(err).ToNot(HaveOccurred())
			Expect(masterComponent).ToNot(BeNil())

			// Test worker component
			workerComponent, err := pytorchExtractor.GetComponent("worker")
			Expect(err).ToNot(HaveOccurred())
			Expect(workerComponent).ToNot(BeNil())
		})

		It("should extract pod template specs from PyTorch components", func() {
			masterComponent, err := pytorchExtractor.GetComponent("master")
			Expect(err).ToNot(HaveOccurred())

			// Test pod template spec extraction
			podTemplateSpecs, err := masterComponent.GetPodTemplateSpec()
			Expect(err).ToNot(HaveOccurred())
			Expect(podTemplateSpecs).ToNot(BeNil())
			Expect(podTemplateSpecs).To(HaveLen(1))
		})

		It("should cache component results", func() {
			masterComponent, err := pytorchExtractor.GetComponent("master")
			Expect(err).ToNot(HaveOccurred())

			// First call - should execute JQ
			podTemplateSpecs1, err1 := masterComponent.GetPodTemplateSpec()

			// Second call - should return cached result
			podTemplateSpecs2, err2 := masterComponent.GetPodTemplateSpec()

			// Both calls should succeed
			Expect(err1).ToNot(HaveOccurred())
			Expect(err2).ToNot(HaveOccurred())

			// Results should be identical (cached)
			Expect(podTemplateSpecs1).To(Equal(podTemplateSpecs2))
		})
	})

	XContext("JobSet Extractor", func() {
		var jobSetExtractor extractor.Extractor

		BeforeEach(func() {
			jobSetExtractor = extractor.NewComponentExtractor(jobSetRID, jobSet)
			Expect(jobSetExtractor).ToNot(BeNil())
		})

		It("should retrieve JobSet components", func() {
			// Test leader component
			leaderComponent, err := jobSetExtractor.GetComponent("leader")
			Expect(err).ToNot(HaveOccurred())
			Expect(leaderComponent).ToNot(BeNil())

			// Test worker component
			workerComponent, err := jobSetExtractor.GetComponent("worker")
			Expect(err).ToNot(HaveOccurred())
			Expect(workerComponent).ToNot(BeNil())
		})

		It("should extract pod template specs from JobSet components", func() {
			leaderComponent, err := jobSetExtractor.GetComponent("leader")
			Expect(err).ToNot(HaveOccurred())

			// Test pod template spec extraction from array of specs
			podTemplateSpecs, err := leaderComponent.GetPodTemplateSpec()
			// The actual expectations will depend on the implementation
			_ = podTemplateSpecs
			_ = err
		})
	})

	XContext("Dynamo Extractor", func() {
		var dynamoExtractor extractor.Extractor

		BeforeEach(func() {
			dynamoExtractor = extractor.NewComponentExtractor(dynamoRID, dynamo)
			Expect(dynamoExtractor).ToNot(BeNil())
		})

		It("should retrieve Dynamo components", func() {
			// Test dynamographdeployment component (root)
			dynamoComponent, err := dynamoExtractor.GetComponent("dynamographdeployment")
			Expect(err).ToNot(HaveOccurred())
			Expect(dynamoComponent).ToNot(BeNil())

			// Test service component (child)
			serviceComponent, err := dynamoExtractor.GetComponent("service")
			Expect(err).ToNot(HaveOccurred())
			Expect(serviceComponent).ToNot(BeNil())
		})

		It("should extract pod template specs from Dynamo components", func() {
			serviceComponent, err := dynamoExtractor.GetComponent("service")
			Expect(err).ToNot(HaveOccurred())

			// Test pod template spec extraction from map of services
			podTemplateSpecs, err := serviceComponent.GetPodTemplateSpec()
			// The actual expectations will depend on the implementation
			_ = podTemplateSpecs
			_ = err
		})
	})
})

// Helper functions

func getProjectRoot() (string, error) {
	// Start from current directory and walk up to find go.mod
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break // reached root directory
		}
		currentDir = parentDir
	}

	return "", os.ErrNotExist
}

func loadRIDFromFile(filePath string) (*v1alpha1.ResourceInterpretationDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var rid v1alpha1.ResourceInterpretationDefinition
	if err := yaml.Unmarshal(data, &rid); err != nil {
		return nil, err
	}

	// Debug: Check if RID is empty
	if rid.Spec.StructureDefinition.RootComponent.Name == "" {
		return nil, fmt.Errorf("RID loaded but appears empty - root component name is blank")
	}

	return &rid, nil
}

func createPyTorchJobObject() client.Object {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubeflow.org/v1",
			"kind":       "PyTorchJob",
			"metadata": map[string]interface{}{
				"name":      "pytorch-training-job",
				"namespace": "default",
				"labels": map[string]interface{}{
					"training.kubeflow.org/job-name": "pytorch-training-job",
				},
			},
			"spec": map[string]interface{}{
				"pytorchReplicaSpecs": map[string]interface{}{
					"Master": map[string]interface{}{
						"replicas": 1,
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"training.kubeflow.org/replica-type": "master",
								},
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "pytorch",
										"image": "pytorch/pytorch:latest",
										"resources": map[string]interface{}{
											"limits": map[string]interface{}{
												"nvidia.com/gpu": "1",
											},
										},
									},
								},
							},
						},
					},
					"Worker": map[string]interface{}{
						"replicas": 2,
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"training.kubeflow.org/replica-type": "worker",
								},
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "pytorch",
										"image": "pytorch/pytorch:latest",
										"resources": map[string]interface{}{
											"limits": map[string]interface{}{
												"nvidia.com/gpu": "1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func createJobSetObject() client.Object {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "jobset.x-k8s.io/v1alpha2",
			"kind":       "JobSet",
			"metadata": map[string]interface{}{
				"name":      "distributed-training",
				"namespace": "default",
				"labels": map[string]interface{}{
					"jobset.sigs.k8s.io/jobset-name": "distributed-training",
				},
			},
			"spec": map[string]interface{}{
				"replicatedJobs": []interface{}{
					map[string]interface{}{
						"name":     "leader",
						"replicas": 1,
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"parallelism":  1,
								"completions":  1,
								"backoffLimit": 0,
								"template": map[string]interface{}{
									"metadata": map[string]interface{}{
										"labels": map[string]interface{}{
											"jobset.sigs.k8s.io/job-name": "leader",
											"app":                         "distributed-training",
										},
									},
									"spec": map[string]interface{}{
										"restartPolicy": "OnFailure",
										"containers": []interface{}{
											map[string]interface{}{
												"name":    "trainer",
												"image":   "pytorch/pytorch:2.0.1-cuda11.7-cudnn8-devel",
												"command": []interface{}{"python", "train.py"},
												"args":    []interface{}{"--role=leader"},
												"resources": map[string]interface{}{
													"limits": map[string]interface{}{
														"nvidia.com/gpu": "2",
														"memory":         "8Gi",
													},
													"requests": map[string]interface{}{
														"nvidia.com/gpu": "2",
														"memory":         "8Gi",
													},
												},
												"env": []interface{}{
													map[string]interface{}{
														"name":  "RANK",
														"value": "0",
													},
													map[string]interface{}{
														"name":  "WORLD_SIZE",
														"value": "4",
													},
												},
											},
										},
									},
								},
							},
						},
					},
					map[string]interface{}{
						"name":     "worker",
						"replicas": 3,
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"parallelism":  1,
								"completions":  1,
								"backoffLimit": 0,
								"template": map[string]interface{}{
									"metadata": map[string]interface{}{
										"labels": map[string]interface{}{
											"jobset.sigs.k8s.io/job-name": "worker",
											"app":                         "distributed-training",
										},
									},
									"spec": map[string]interface{}{
										"restartPolicy": "OnFailure",
										"containers": []interface{}{
											map[string]interface{}{
												"name":    "trainer",
												"image":   "pytorch/pytorch:2.0.1-cuda11.7-cudnn8-devel",
												"command": []interface{}{"python", "train.py"},
												"args":    []interface{}{"--role=worker"},
												"resources": map[string]interface{}{
													"limits": map[string]interface{}{
														"nvidia.com/gpu": "1",
														"memory":         "4Gi",
													},
													"requests": map[string]interface{}{
														"nvidia.com/gpu": "1",
														"memory":         "4Gi",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Completed",
						"status": "False",
					},
					map[string]interface{}{
						"type":   "Failed",
						"status": "False",
					},
				},
				"replicatedJobsStatus": []interface{}{
					map[string]interface{}{
						"name":      "leader",
						"ready":     0,
						"succeeded": 0,
						"failed":    0,
						"active":    1,
					},
					map[string]interface{}{
						"name":      "worker",
						"ready":     0,
						"succeeded": 0,
						"failed":    0,
						"active":    3,
					},
				},
			},
		},
	}
}

func createDynamoObject() client.Object {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "nvidia.com/v1alpha1",
			"kind":       "DynamoGraphDeployment",
			"metadata": map[string]interface{}{
				"name":      "graph-deployment",
				"namespace": "default",
				"labels": map[string]interface{}{
					"nvidia.com/dynamo-graph": "graph-deployment",
				},
			},
			"spec": map[string]interface{}{
				"services": map[string]interface{}{
					"frontend": map[string]interface{}{
						"replicas": 2,
						"labels": map[string]interface{}{
							"nvidia.com/dynamo-service": "frontend",
							"app":                       "frontend",
						},
						"annotations": map[string]interface{}{
							"nvidia.com/gpu-count": "1",
						},
						"resources": map[string]interface{}{
							"limits": map[string]interface{}{
								"nvidia.com/gpu": "1",
								"memory":         "4Gi",
							},
						},
						"autoscaling": map[string]interface{}{
							"minReplicas": 1,
							"maxReplicas": 5,
						},
						"extraPodSpec": map[string]interface{}{
							"schedulerName":     "gpu-scheduler",
							"priorityClassName": "high-priority",
							"mainContainer": map[string]interface{}{
								"image": "frontend:latest",
							},
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "sidecar",
									"image": "sidecar:latest",
								},
							},
							"affinity": map[string]interface{}{
								"nodeAffinity": map[string]interface{}{
									"requiredDuringSchedulingIgnoredDuringExecution": map[string]interface{}{
										"nodeSelectorTerms": []interface{}{
											map[string]interface{}{
												"matchExpressions": []interface{}{
													map[string]interface{}{
														"key":      "gpu-type",
														"operator": "In",
														"values":   []interface{}{"A100", "V100"},
													},
												},
											},
										},
									},
								},
								"podAffinity": map[string]interface{}{
									"preferredDuringSchedulingIgnoredDuringExecution": []interface{}{
										map[string]interface{}{
											"weight": 100,
											"podAffinityTerm": map[string]interface{}{
												"labelSelector": map[string]interface{}{
													"matchExpressions": []interface{}{
														map[string]interface{}{
															"key":      "app",
															"operator": "In",
															"values":   []interface{}{"backend"},
														},
													},
												},
												"topologyKey": "kubernetes.io/hostname",
											},
										},
									},
								},
							},
						},
					},
					"backend": map[string]interface{}{
						"replicas": 3,
						"labels": map[string]interface{}{
							"nvidia.com/dynamo-service": "backend",
							"app":                       "backend",
						},
						"resources": map[string]interface{}{
							"limits": map[string]interface{}{
								"nvidia.com/gpu": "2",
								"memory":         "8Gi",
							},
						},
						"autoscaling": map[string]interface{}{
							"minReplicas": 2,
							"maxReplicas": 10,
						},
						"extraPodSpec": map[string]interface{}{
							"schedulerName":     "gpu-scheduler",
							"priorityClassName": "medium-priority",
							"mainContainer": map[string]interface{}{
								"image": "backend:latest",
							},
						},
					},
				},
			},
			"status": map[string]interface{}{
				"state": "successful",
			},
		},
	}
}
