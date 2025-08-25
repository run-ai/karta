package rid_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/runai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/runai/kai-bolt/pkg/utils/rid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ComponentProvider", func() {
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

	Context("PyTorch Job ComponentProvider", func() {
		var pytorchProvider rid.ComponentProvider

		BeforeEach(func() {
			pytorchProvider = rid.NewRidComponentProvider(pytorchRID, pytorchJob)
			Expect(pytorchProvider).ToNot(BeNil())
		})

		It("should retrieve PyTorch components", func() {
			// Test root component
			pytorchJobComponent, err := pytorchProvider.GetComponent("pytorchjob")
			Expect(err).ToNot(HaveOccurred())
			Expect(pytorchJobComponent).ToNot(BeNil())
			Expect(pytorchJobComponent.Name()).To(Equal("pytorchjob"))
			Expect(pytorchJobComponent.Definition().Name).To(Equal("pytorchjob"))

			// Test master component
			masterComponent, err := pytorchProvider.GetComponent("master")
			Expect(err).ToNot(HaveOccurred())
			Expect(masterComponent).ToNot(BeNil())
			Expect(masterComponent.Name()).To(Equal("master"))
			Expect(masterComponent.Definition().Name).To(Equal("master"))

			// Test worker component
			workerComponent, err := pytorchProvider.GetComponent("worker")
			Expect(err).ToNot(HaveOccurred())
			Expect(workerComponent).ToNot(BeNil())
			Expect(workerComponent.Name()).To(Equal("worker"))
			Expect(workerComponent.Definition().Name).To(Equal("worker"))

			// Test non-existent component should fail
			_, err = pytorchProvider.GetComponent("nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("component nonexistent not found"))
		})

		It("should extract pod template specs from PyTorch components", func() {
			masterComponent, err := pytorchProvider.GetComponent("master")
			Expect(err).ToNot(HaveOccurred())

			// Test pod template spec extraction from master
			podTemplateSpecs, err := masterComponent.GetPodTemplateSpec()
			Expect(err).ToNot(HaveOccurred())
			Expect(podTemplateSpecs).ToNot(BeNil())
			Expect(podTemplateSpecs).To(HaveLen(1), "Master should return exactly 1 pod template spec")

			// Validate the extracted pod template spec content
			masterSpec := podTemplateSpecs[0]
			Expect(masterSpec.Spec.Containers).ToNot(BeEmpty(), "Pod template should have containers")
			Expect(masterSpec.Spec.Containers[0].Name).To(Equal("pytorch"), "Container name should be 'pytorch'")
			Expect(masterSpec.Spec.Containers[0].Image).To(Equal("pytorch/pytorch:latest"), "Container image should be correct")
			Expect(string(masterSpec.Spec.RestartPolicy)).To(Equal("OnFailure"), "Restart policy should be OnFailure")

			// Test GPU resource requests
			resources := masterSpec.Spec.Containers[0].Resources.Requests
			Expect(resources).To(HaveKey(corev1.ResourceName("nvidia.com/gpu")), "Should request GPU resources")

			// Test worker component
			workerComponent, err := pytorchProvider.GetComponent("worker")
			Expect(err).ToNot(HaveOccurred())

			workerSpecs, err := workerComponent.GetPodTemplateSpec()
			Expect(err).ToNot(HaveOccurred())
			Expect(workerSpecs).ToNot(BeNil())
			Expect(workerSpecs).To(HaveLen(1), "Worker should return exactly 1 pod template spec")

			// Validate worker spec content
			workerSpec := workerSpecs[0]
			Expect(workerSpec.Spec.Containers).ToNot(BeEmpty(), "Worker pod template should have containers")
			Expect(workerSpec.Spec.Containers[0].Name).To(Equal("pytorch"), "Worker container name should be 'pytorch'")
		})

		It("should cache component results", func() {
			masterComponent, err := pytorchProvider.GetComponent("master")
			Expect(err).ToNot(HaveOccurred())

			// First call - should execute extraction
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

	Context("JobSet ComponentProvider", func() {
		var jobSetProvider rid.ComponentProvider

		BeforeEach(func() {
			jobSetProvider = rid.NewRidComponentProvider(jobSetRID, jobSet)
			Expect(jobSetProvider).ToNot(BeNil())
		})

		It("should retrieve JobSet components", func() {
			// Test root component
			jobSetComponent, err := jobSetProvider.GetComponent("jobset")
			Expect(err).ToNot(HaveOccurred())
			Expect(jobSetComponent).ToNot(BeNil())
			Expect(jobSetComponent.Name()).To(Equal("jobset"))
			Expect(jobSetComponent.Definition().Name).To(Equal("jobset"))

			// Test replicatedjob component
			replicatedJobComponent, err := jobSetProvider.GetComponent("replicatedjob")
			Expect(err).ToNot(HaveOccurred())
			Expect(replicatedJobComponent).ToNot(BeNil())
			Expect(replicatedJobComponent.Name()).To(Equal("replicatedjob"))
			Expect(replicatedJobComponent.Definition().Name).To(Equal("replicatedjob"))

			// Test non-existent component should fail
			_, err = jobSetProvider.GetComponent("nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("component nonexistent not found"))
		})

		XIt("should extract pod template specs from JobSet components", func() {
			replicatedJobComponent, err := jobSetProvider.GetComponent("replicatedjob")
			Expect(err).ToNot(HaveOccurred())

			// Test pod template spec extraction from replicatedjob
			podTemplateSpecs, err := replicatedJobComponent.GetPodTemplateSpec()
			Expect(err).ToNot(HaveOccurred())
			Expect(podTemplateSpecs).ToNot(BeNil())
			// JobSet replicatedJobs array should return multiple specs (leader + worker in our test data)
			Expect(podTemplateSpecs).To(HaveLen(2), "ReplicatedJob should return 2 pod template specs (leader + worker)")

			// Validate leader job spec (first in array)
			leaderSpec := podTemplateSpecs[0]
			Expect(leaderSpec.Spec.Containers).ToNot(BeEmpty(), "Leader pod template should have containers")
			Expect(leaderSpec.Spec.Containers[0].Name).To(Equal("leader"), "Leader container name should be 'leader'")
			Expect(leaderSpec.Spec.Containers[0].Image).To(Equal("busybox:latest"), "Leader container image should be busybox")
			Expect(leaderSpec.Spec.RestartPolicy).To(Equal("Never"), "Leader restart policy should be Never")

			// Validate worker job spec (second in array)
			workerSpec := podTemplateSpecs[1]
			Expect(workerSpec.Spec.Containers).ToNot(BeEmpty(), "Worker pod template should have containers")
			Expect(workerSpec.Spec.Containers[0].Name).To(Equal("worker"), "Worker container name should be 'worker'")
			Expect(workerSpec.Spec.Containers[0].Image).To(Equal("busybox:latest"), "Worker container image should be busybox")
		})
	})

	Context("Dynamo ComponentProvider", func() {
		var dynamoProvider rid.ComponentProvider

		BeforeEach(func() {
			dynamoProvider = rid.NewRidComponentProvider(dynamoRID, dynamo)
			Expect(dynamoProvider).ToNot(BeNil())
		})

		It("should retrieve Dynamo components", func() {
			// Test root component
			dynamoComponent, err := dynamoProvider.GetComponent("dynamographdeployment")
			Expect(err).ToNot(HaveOccurred())
			Expect(dynamoComponent).ToNot(BeNil())
			Expect(dynamoComponent.Name()).To(Equal("dynamographdeployment"))
			Expect(dynamoComponent.Definition().Name).To(Equal("dynamographdeployment"))

			// Test service component
			serviceComponent, err := dynamoProvider.GetComponent("service")
			Expect(err).ToNot(HaveOccurred())
			Expect(serviceComponent).ToNot(BeNil())
			Expect(serviceComponent.Name()).To(Equal("service"))
			Expect(serviceComponent.Definition().Name).To(Equal("service"))

			// Test non-existent component should fail
			_, err = dynamoProvider.GetComponent("nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("component nonexistent not found"))
		})

		It("should extract pod template specs from Dynamo components", func() {
			serviceComponent, err := dynamoProvider.GetComponent("service")
			Expect(err).ToNot(HaveOccurred())

			// Test pod template spec extraction from service
			fragmentedSpecs, err := serviceComponent.GetFragmentedPodSpec()
			Expect(err).ToNot(HaveOccurred())
			Expect(fragmentedSpecs).ToNot(BeNil())
		})
	})
})

// Helper functions - exact copies from original tests

func getProjectRoot() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Look for go.mod file to identify project root
	for {
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			return currentDir, nil
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}

	return "", fmt.Errorf("could not find project root")
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

	// Debug check to ensure RID loaded correctly
	if rid.Spec.StructureDefinition.RootComponent.Name == "" {
		fmt.Printf("WARNING: Empty RID loaded from %s\n", filePath)
	}

	return &rid, nil
}

func createPyTorchJobObject() client.Object {
	pytorchJob := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubeflow.org/v1",
			"kind":       "PyTorchJob",
			"metadata": map[string]interface{}{
				"name":      "pytorch-simple",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"pytorchReplicaSpecs": map[string]interface{}{
					"Master": map[string]interface{}{
						"replicas": 1,
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"app": "pytorch-master",
								},
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "pytorch",
										"image": "pytorch/pytorch:latest",
										"command": []interface{}{
											"python",
											"train.py",
										},
										"resources": map[string]interface{}{
											"requests": map[string]interface{}{
												"nvidia.com/gpu": 1,
											},
										},
									},
								},
								"restartPolicy": "OnFailure",
							},
						},
					},
					"Worker": map[string]interface{}{
						"replicas": 3,
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"app": "pytorch-worker",
								},
							},
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "pytorch",
										"image": "pytorch/pytorch:latest",
										"command": []interface{}{
											"python",
											"train.py",
										},
										"resources": map[string]interface{}{
											"requests": map[string]interface{}{
												"nvidia.com/gpu": 1,
											},
										},
									},
								},
								"restartPolicy": "OnFailure",
							},
						},
					},
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Running",
						"status": "True",
					},
				},
			},
		},
	}

	return pytorchJob
}

func createJobSetObject() client.Object {
	jobSet := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "jobset.x-k8s.io/v1alpha2",
			"kind":       "JobSet",
			"metadata": map[string]interface{}{
				"name":      "simple-jobset",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicatedJobs": []interface{}{
					map[string]interface{}{
						"name":     "leader",
						"replicas": 1,
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"parallelism":   1,
								"completions":   1,
								"backoffLimit":  6,
								"restartPolicy": "OnFailure",
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"containers": []interface{}{
											map[string]interface{}{
												"name":    "leader",
												"image":   "busybox:latest",
												"command": []interface{}{"sh", "-c", "echo 'Leader job' && sleep 30"},
												"resources": map[string]interface{}{
													"requests": map[string]interface{}{
														"cpu":    "100m",
														"memory": "128Mi",
													},
												},
												"env": []interface{}{
													map[string]interface{}{
														"name":  "JOB_ROLE",
														"value": "leader",
													},
												},
											},
										},
										"restartPolicy": "Never",
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
								"parallelism":   1,
								"completions":   1,
								"backoffLimit":  6,
								"restartPolicy": "OnFailure",
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"containers": []interface{}{
											map[string]interface{}{
												"name":    "worker",
												"image":   "busybox:latest",
												"command": []interface{}{"sh", "-c", "echo 'Worker job' && sleep 60"},
												"resources": map[string]interface{}{
													"requests": map[string]interface{}{
														"cpu":    "200m",
														"memory": "256Mi",
													},
												},
												"env": []interface{}{
													map[string]interface{}{
														"name":  "JOB_ROLE",
														"value": "worker",
													},
												},
											},
										},
										"restartPolicy": "Never",
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
						"status": "True",
					},
				},
			},
		},
	}

	return jobSet
}

func createDynamoObject() client.Object {
	dynamo := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "dynamo.nvidia.com/v1alpha1",
			"kind":       "DynamoGraphDeployment",
			"metadata": map[string]interface{}{
				"name":      "example-dynamo",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"services": map[string]interface{}{
					"frontend": map[string]interface{}{
						"image": "nginx:latest",
						"extraPodSpec": map[string]interface{}{
							"schedulerName": "volcano",
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "frontend",
									"image": "nginx:latest",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu":    "100m",
											"memory": "128Mi",
										},
									},
								},
							},
						},
						"resources": map[string]interface{}{
							"requests": map[string]interface{}{
								"cpu":    "100m",
								"memory": "128Mi",
							},
						},
						"autoscaling": map[string]interface{}{
							"minReplicas": 1,
							"maxReplicas": 5,
						},
						"labels": map[string]interface{}{
							"app":  "dynamo-frontend",
							"tier": "frontend",
						},
						"annotations": map[string]interface{}{
							"prometheus.io/scrape": "true",
						},
					},
					"backend": map[string]interface{}{
						"image": "python:3.9",
						"extraPodSpec": map[string]interface{}{
							"schedulerName": "volcano",
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "backend",
									"image": "python:3.9",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu":            "200m",
											"memory":         "256Mi",
											"nvidia.com/gpu": 1,
										},
									},
								},
							},
						},
						"resources": map[string]interface{}{
							"requests": map[string]interface{}{
								"cpu":            "200m",
								"memory":         "256Mi",
								"nvidia.com/gpu": 1,
							},
						},
						"autoscaling": map[string]interface{}{
							"minReplicas": 2,
							"maxReplicas": 10,
						},
						"labels": map[string]interface{}{
							"app":  "dynamo-backend",
							"tier": "backend",
						},
						"annotations": map[string]interface{}{
							"prometheus.io/scrape": "true",
						},
					},
				},
			},
			"status": map[string]interface{}{
				"phase": "Running",
			},
		},
	}

	return dynamo
}
