package resource

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/utils/resource/query"
	"github.com/run-ai/kai-bolt/test/types"
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

	// Labels
	roleLabel        = "role"
	appLabel         = "app"
	serviceNameLabel = "service-name"

	// Role values
	masterRole    = "master"
	workerRole    = "worker"
	indexerRole   = "indexer"
	processorRole = "processor"

	// App values
	pyflowApp   = "pyflow"
	jobgroupApp = "jobgroup"
	reactorApp  = "reactor"

	// Error message substrings
	noPodTemplateSpecError = "does not have pod template spec definition"
	noPodSpecError         = "does not have pod spec definition"
	noMetadataError        = "does not have pod metadata definition"
	noFragmentedSpecError  = "does not have fragmented pod spec definition"
)

var _ = Describe("InterfaceExtractor", func() {
	var (
		ctx context.Context

		// PyFlow fixtures
		pyflowRI        *v1alpha1.ResourceInterface
		pyflowObj       *types.PyFlow
		pyflowExtractor Extractor
		pyflowFactory   *ComponentFactory

		// JobGroup fixtures
		jobgroupRI        *v1alpha1.ResourceInterface
		jobgroupObj       *types.JobGroup
		jobgroupExtractor Extractor
		jobgroupFactory   *ComponentFactory

		// Reactor fixtures
		reactorRI        *v1alpha1.ResourceInterface
		reactorObj       *types.Reactor
		reactorExtractor Extractor
		reactorFactory   *ComponentFactory
	)

	BeforeEach(func() {
		ctx = context.Background()

		pyflowRI = types.PyFlowRI()
		pyflowObj = types.NewPyFlowObject()
		pyflowExtractor = NewInterfaceExtractor(query.NewDefaultJqEvaluator(pyflowObj))
		pyflowFactory = NewComponentFactory(pyflowRI, pyflowExtractor)

		jobgroupRI = types.JobGroupRI()
		jobgroupObj = types.NewJobGroupObject()
		jobgroupExtractor = NewInterfaceExtractor(query.NewDefaultJqEvaluator(jobgroupObj))
		jobgroupFactory = NewComponentFactory(jobgroupRI, jobgroupExtractor)

		reactorRI = types.ReactorRI()
		reactorObj = types.NewReactorObject()
		reactorExtractor = NewInterfaceExtractor(query.NewDefaultJqEvaluator(reactorObj))
		reactorFactory = NewComponentFactory(reactorRI, reactorExtractor)
	})

	Describe("ExtractPodTemplateSpec", func() {
		Context("supported workloads", func() {
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
				Expect(result).To(HaveLen(2)) // indexer + processor
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
				Expect(result).To(HaveLen(2)) // indexer + processor
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
			It("should extract all service fragmented pod specs", func() {
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
		})
	})

	Describe("ExtractScale", func() {
		Context("supported workloads", func() {
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

			It("should extract worker scale (min/max only)", func() {
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

		Context("supported workloads", func() {
			It("should extract job scales from array", func() {
				jobComp, err := jobgroupFactory.GetComponent(jobComponentName)
				Expect(err).NotTo(HaveOccurred())

				result, err := jobgroupExtractor.ExtractScale(ctx, jobComp.definition)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))                   // indexer + processor
				Expect(*result[0].Replicas).To(Equal(int32(2))) // indexer has 2 replicas
				Expect(*result[1].Replicas).To(Equal(int32(3))) // processor has 3 replicas
			})

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
	})
})
