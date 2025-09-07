package resource

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/query"
	"github.com/run-ai/kai-bolt/test/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

const (
	// Component names
	masterComponentName  = "master"
	workerComponentName  = "worker"
	jobComponentName     = "job"
	serviceComponentName = "service"

	// Container names
	trainerContainerName   = "trainer"
	indexerContainerName   = "indexer"
	processorContainerName = "processor"

	// Service names
	apiServiceName    = "api"
	workerServiceName = "worker"
	cacheServiceName  = "cache"

	// Label keys
	roleLabel        = "role"
	serviceNameLabel = "service-name"
	appLabel         = "app"

	// Label values
	masterRole    = "master"
	workerRole    = "worker"
	indexerRole   = "indexer"
	processorRole = "processor"

	// App values
	jobgroupApp = "jobgroup"
	reactorApp  = "reactor"

	// Error message substrings
	noPodTemplateSpecError = "does not have pod template spec definition"
	noPodSpecError         = "does not have pod spec definition"
	noMetadataError        = "does not have pod metadata definition"
	noFragmentedSpecError  = "does not have fragmented pod spec definition"
	noSpecDefinitionError  = "does not have spec definition"
	noScaleError           = "does not have scale definition"
)

var _ = Describe("InterfaceExtractor", func() {
	var (
		ctx context.Context

		pyflowRI   *v1alpha1.ResourceInterface
		jobgroupRI *v1alpha1.ResourceInterface
		reactorRI  *v1alpha1.ResourceInterface

		pyflowFactory   *ComponentFactory
		jobgroupFactory *ComponentFactory
		reactorFactory  *ComponentFactory

		pyflowExtractor   *InterfaceExtractor
		jobgroupExtractor *InterfaceExtractor
		reactorExtractor  *InterfaceExtractor
	)

	BeforeEach(func() {
		ctx = context.Background()

		pyflowRI = types.PyFlowRI()
		jobgroupRI = types.JobGroupRI()
		reactorRI = types.ReactorRI()

		// Create test objects
		pyflowObject := types.NewPyFlowObject()
		jobgroupObject := types.NewJobGroupObject()
		reactorObject := types.NewReactorObject()

		// Initialize factories
		pyflowFactory = NewComponentFactoryFromObject(pyflowRI, pyflowObject)
		jobgroupFactory = NewComponentFactoryFromObject(jobgroupRI, jobgroupObject)
		reactorFactory = NewComponentFactoryFromObject(reactorRI, reactorObject)

		// Initialize extractors
		pyflowExtractor = NewInterfaceExtractor(query.NewDefaultJqEvaluator(pyflowObject))
		jobgroupExtractor = NewInterfaceExtractor(query.NewDefaultJqEvaluator(jobgroupObject))
		reactorExtractor = NewInterfaceExtractor(query.NewDefaultJqEvaluator(reactorObject))
	})

	Describe("ExtractPodTemplateSpec", func() {
		Context("supported workloads", func() {
			Context("PyFlow", func() {
				It("should extract master pod template spec", func() {
					masterComp, err := pyflowFactory.GetComponent(masterComponentName)
					Expect(err).NotTo(HaveOccurred())

					result, err := pyflowExtractor.ExtractPodTemplateSpec(ctx, masterComp.definition)

					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(HaveLen(1))
					Expect(result[0].ObjectMeta.Labels[roleLabel]).To(Equal(masterRole))
					Expect(result[0].Spec.Containers).To(HaveLen(1))
					Expect(result[0].Spec.Containers[0].Name).To(Equal(trainerContainerName))
				})

				It("should extract worker pod template spec", func() {
					workerComp, err := pyflowFactory.GetComponent(workerComponentName)
					Expect(err).NotTo(HaveOccurred())

					result, err := pyflowExtractor.ExtractPodTemplateSpec(ctx, workerComp.definition)

					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(HaveLen(1))
					Expect(result[0].ObjectMeta.Labels[roleLabel]).To(Equal(workerRole))
					Expect(result[0].Spec.Containers[0].Name).To(Equal(trainerContainerName))
				})
			})
		})

		Context("unsupported workloads", func() {
			It("should return error for workloads without PodTemplateSpecPath", func() {
				jobComp, err := jobgroupFactory.GetComponent(jobComponentName)
				Expect(err).NotTo(HaveOccurred())

				_, err = jobgroupExtractor.ExtractPodTemplateSpec(ctx, jobComp.definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noPodTemplateSpecError))
			})

			It("should return error for fragmented workloads", func() {
				serviceComp, err := reactorFactory.GetComponent(serviceComponentName)
				Expect(err).NotTo(HaveOccurred())

				_, err = reactorExtractor.ExtractPodTemplateSpec(ctx, serviceComp.definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noPodTemplateSpecError))
			})
		})
	})

	Describe("ExtractPodSpec", func() {
		Context("supported workloads", func() {
			It("should extract job pod specs", func() {
				jobComp, err := jobgroupFactory.GetComponent(jobComponentName)
				Expect(err).NotTo(HaveOccurred())

				result, err := jobgroupExtractor.ExtractPodSpec(ctx, jobComp.definition)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				Expect(result[0].Containers).To(HaveLen(1))
				Expect(result[0].Containers[0].Name).To(Equal(indexerContainerName))
				Expect(result[1].Containers[0].Name).To(Equal(processorContainerName))
			})
		})

		Context("unsupported workloads", func() {
			It("should return error for template-based workloads", func() {
				masterComp, err := pyflowFactory.GetComponent(masterComponentName)
				Expect(err).NotTo(HaveOccurred())

				_, err = pyflowExtractor.ExtractPodSpec(ctx, masterComp.definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noPodSpecError))
			})

			It("should return error for fragmented workloads", func() {
				serviceComp, err := reactorFactory.GetComponent(serviceComponentName)
				Expect(err).NotTo(HaveOccurred())

				_, err = reactorExtractor.ExtractPodSpec(ctx, serviceComp.definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noPodSpecError))
			})
		})
	})

	Describe("ExtractPodMetadata", func() {
		Context("supported workloads", func() {
			It("should extract job pod metadata", func() {
				jobComp, err := jobgroupFactory.GetComponent(jobComponentName)
				Expect(err).NotTo(HaveOccurred())

				result, err := jobgroupExtractor.ExtractPodMetadata(ctx, jobComp.definition)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				Expect(result[0].Labels[roleLabel]).To(Equal(indexerRole))
				Expect(result[0].Labels[appLabel]).To(Equal(jobgroupApp))
				Expect(result[1].Labels[roleLabel]).To(Equal(processorRole))
			})
		})

		Context("unsupported workloads", func() {
			It("should return error for template-based workloads", func() {
				masterComp, err := pyflowFactory.GetComponent(masterComponentName)
				Expect(err).NotTo(HaveOccurred())

				_, err = pyflowExtractor.ExtractPodMetadata(ctx, masterComp.definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noMetadataError))
			})

			It("should return error for fragmented workloads", func() {
				serviceComp, err := reactorFactory.GetComponent(serviceComponentName)
				Expect(err).NotTo(HaveOccurred())

				_, err = reactorExtractor.ExtractPodMetadata(ctx, serviceComp.definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noMetadataError))
			})
		})
	})

	Describe("ExtractFragmentedPodSpec", func() {
		Context("supported workloads", func() {
			It("should extract fragmented pod specs", func() {
				serviceComp, err := reactorFactory.GetComponent(serviceComponentName)
				Expect(err).NotTo(HaveOccurred())

				result, err := reactorExtractor.ExtractFragmentedPodSpec(ctx, serviceComp.definition)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(3)) // api + worker + cache services

				// Verify we have the expected services (map iteration order is non-deterministic)
				serviceNames := make([]string, len(result))
				for i, spec := range result {
					serviceNames[i] = spec.Labels[serviceNameLabel]
				}
				Expect(serviceNames).To(ConsistOf(apiServiceName, workerServiceName, cacheServiceName))

				// Find each service for detailed assertions
				var apiSpec, workerSpec, cacheSpec *FragmentedPodSpec
				for i := range result {
					switch result[i].Labels[serviceNameLabel] {
					case apiServiceName:
						apiSpec = &result[i]
					case workerServiceName:
						workerSpec = &result[i]
					case cacheServiceName:
						cacheSpec = &result[i]
					}
				}

				// API service assertions
				Expect(apiSpec).NotTo(BeNil())
				Expect(apiSpec.Labels).To(HaveKeyWithValue(appLabel, reactorApp))
				Expect(apiSpec.Labels).To(HaveKeyWithValue(serviceNameLabel, apiServiceName))
				Expect(apiSpec.Labels).To(HaveKeyWithValue("tier", "frontend"))
				Expect(apiSpec.Annotations).To(HaveKeyWithValue("service.beta.kubernetes.io/aws-load-balancer-type", "nlb"))
				Expect(apiSpec.Containers).To(HaveLen(1))
				Expect(apiSpec.Containers[0].Name).To(Equal("api-server"))
				Expect(apiSpec.Containers[0].Image).To(Equal("api:latest"))
				Expect(apiSpec.Containers[0].Ports).To(HaveLen(1))
				Expect(apiSpec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(8080)))
				Expect(apiSpec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PORT", Value: "8080"}))
				Expect(apiSpec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")))
				Expect(apiSpec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("1Gi")))
				Expect(apiSpec.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("1")))
				Expect(apiSpec.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("2Gi")))

				// Worker service assertions
				Expect(workerSpec).NotTo(BeNil())
				Expect(workerSpec.Labels).To(HaveKeyWithValue(appLabel, reactorApp))
				Expect(workerSpec.Labels).To(HaveKeyWithValue(serviceNameLabel, workerServiceName))
				Expect(workerSpec.Labels).To(HaveKeyWithValue("tier", "backend"))
				Expect(workerSpec.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))
				Expect(workerSpec.Annotations).To(HaveKeyWithValue("prometheus.io/port", "9090"))
				Expect(workerSpec.Containers).To(HaveLen(1))
				Expect(workerSpec.Containers[0].Name).To(Equal("worker"))
				Expect(workerSpec.Containers[0].Image).To(Equal("worker:latest"))
				Expect(workerSpec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "WORKER_TYPE", Value: "processor"}))
				Expect(workerSpec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("1")))
				Expect(workerSpec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("2Gi")))
				Expect(workerSpec.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("2")))
				Expect(workerSpec.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("4Gi")))
				gpuResource := corev1.ResourceName("nvidia.com/gpu")
				Expect(workerSpec.Resources.Limits).To(HaveKey(gpuResource))
				Expect(workerSpec.Resources.Limits[gpuResource]).To(Equal(resource.MustParse("1")))

				// Cache service assertions
				Expect(cacheSpec).NotTo(BeNil())
				Expect(cacheSpec.Labels).To(HaveKeyWithValue(appLabel, reactorApp))
				Expect(cacheSpec.Labels).To(HaveKeyWithValue(serviceNameLabel, cacheServiceName))
				Expect(cacheSpec.Labels).To(HaveKeyWithValue("tier", "middleware"))
				// Cache service has NO annotations in test data - validates missing field handling
				Expect(cacheSpec.Annotations).To(BeEmpty())
				Expect(cacheSpec.Containers).To(HaveLen(1))
				Expect(cacheSpec.Containers[0].Name).To(Equal("redis"))
				Expect(cacheSpec.Containers[0].Image).To(Equal("redis:7-alpine"))
				Expect(cacheSpec.Containers[0].Ports).To(HaveLen(1))
				Expect(cacheSpec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(6379)))
				Expect(cacheSpec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")))
				Expect(cacheSpec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("512Mi")))
				Expect(cacheSpec.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")))
				Expect(cacheSpec.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("1Gi")))
			})
		})

		Context("unsupported workloads", func() {
			It("should return error for template-based workloads", func() {
				masterComp, err := pyflowFactory.GetComponent(masterComponentName)
				Expect(err).NotTo(HaveOccurred())

				_, err = pyflowExtractor.ExtractFragmentedPodSpec(ctx, masterComp.definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noFragmentedSpecError))
			})

			It("should return error for array-based workloads", func() {
				jobComp, err := jobgroupFactory.GetComponent(jobComponentName)
				Expect(err).NotTo(HaveOccurred())

				_, err = jobgroupExtractor.ExtractFragmentedPodSpec(ctx, jobComp.definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noFragmentedSpecError))
			})

			It("should handle nil fragmented pod spec definition", func() {
				definition := v1alpha1.ComponentDefinition{
					Name: "test-component",
					SpecDefinition: &v1alpha1.SpecDefinition{
						FragmentedPodSpecDefinition: nil,
					},
				}

				_, err := reactorExtractor.ExtractFragmentedPodSpec(ctx, definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noFragmentedSpecError))
			})

			It("should handle nil spec definition", func() {
				definition := v1alpha1.ComponentDefinition{
					Name:           "test-component",
					SpecDefinition: nil,
				}

				_, err := reactorExtractor.ExtractFragmentedPodSpec(ctx, definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noSpecDefinitionError))
			})
		})
	})

	Describe("ExtractScale", func() {
		Context("PyFlow", func() {
			It("should extract master scale (replicas only)", func() {
				masterComp, err := pyflowFactory.GetComponent(masterComponentName)
				Expect(err).NotTo(HaveOccurred())

				result, err := pyflowExtractor.ExtractScale(ctx, masterComp.definition)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(*result[0].Replicas).To(Equal(int32(1)))
				Expect(result[0].MinReplicas).To(BeNil())
				Expect(result[0].MaxReplicas).To(BeNil())
			})

			It("should extract worker scale", func() {
				workerComp, err := pyflowFactory.GetComponent(workerComponentName)
				Expect(err).NotTo(HaveOccurred())

				result, err := pyflowExtractor.ExtractScale(ctx, workerComp.definition)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].Replicas).To(BeNil())
				Expect(*result[0].MinReplicas).To(Equal(int32(1)))
				Expect(*result[0].MaxReplicas).To(Equal(int32(5)))
			})
		})

		Context("JobGroup", func() {
			It("should extract job scales from array", func() {
				jobComp, err := jobgroupFactory.GetComponent(jobComponentName)
				Expect(err).NotTo(HaveOccurred())

				result, err := jobgroupExtractor.ExtractScale(ctx, jobComp.definition)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))                   // indexer + processor
				Expect(*result[0].Replicas).To(Equal(int32(2))) // indexer has 2 replicas
				Expect(*result[1].Replicas).To(Equal(int32(3))) // processor has 3 replicas
			})

		})

		Context("Reactor", func() {
			It("should extract all service scales from map", func() {
				serviceComp, err := reactorFactory.GetComponent(serviceComponentName)
				Expect(err).NotTo(HaveOccurred())

				result, err := reactorExtractor.ExtractScale(ctx, serviceComp.definition)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(3)) // api + worker + cache services

				// Extract the replica values (map iteration order is non-deterministic)
				replicaCounts := make([]int32, len(result))
				for i, scale := range result {
					replicaCounts[i] = *scale.Replicas
				}

				// Verify we got the expected replica counts from all services
				Expect(replicaCounts).To(ConsistOf(int32(3), int32(5), int32(1))) // api=3, worker=5, cache=1
			})
		})

		Context("unsupported workloads", func() {
			It("should handle nil scale definition", func() {
				definition := v1alpha1.ComponentDefinition{
					Name:            "test-component",
					ScaleDefinition: nil,
				}

				_, err := reactorExtractor.ExtractScale(ctx, definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(noScaleError))
			})
		})
	})

	Describe("Error Handling", func() {
		Context("safeConvertSlice", func() {
			It("should handle conversion errors gracefully", func() {
				// Create a mock evaluator that returns data that can't be converted
				mockEvaluator := query.NewMockQueryEvaluator(gomock.NewController(GinkgoT()))
				extractor := NewInterfaceExtractor(mockEvaluator)

				// Test with incompatible data types that should fail conversion
				mockEvaluator.EXPECT().
					Evaluate(gomock.Any(), "spec.podTemplate").
					Return([]any{
						map[string]any{
							"metadata": "this is a string, not ObjectMeta",
							"spec":     "this is also a string, not PodSpec",
						},
					}, nil)

				definition := v1alpha1.ComponentDefinition{
					Name: "test-component",
					SpecDefinition: &v1alpha1.SpecDefinition{
						PodTemplateSpecPath: ptr.To("spec.podTemplate"),
					},
				}

				_, err := extractor.ExtractPodTemplateSpec(ctx, definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to convert object"))
			})

			It("should handle circular reference errors in JSON conversion", func() {
				// Create a mock evaluator that returns circular reference data
				mockEvaluator := query.NewMockQueryEvaluator(gomock.NewController(GinkgoT()))
				extractor := NewInterfaceExtractor(mockEvaluator)

				// Create a circular reference that would break JSON marshaling
				circularData := make(map[string]any)
				circularData["self"] = circularData

				mockEvaluator.EXPECT().
					Evaluate(gomock.Any(), "spec.resources").
					Return([]any{circularData}, nil)

				definition := v1alpha1.ComponentDefinition{
					Name: "test-component",
					SpecDefinition: &v1alpha1.SpecDefinition{
						FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
							ResourcesPath: ptr.To("spec.resources"),
						},
					},
				}

				_, err := extractor.ExtractFragmentedPodSpec(ctx, definition)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to convert object"))
			})
		})
	})
})
