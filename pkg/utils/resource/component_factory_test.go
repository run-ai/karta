package resource

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/test/types"
)

var _ = Describe("ComponentFactory", func() {
	var (
		ctrl          *gomock.Controller
		mockExtractor *MockExtractor
		ri            *v1alpha1.ResourceInterface
		factory       *ComponentFactory
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockExtractor = NewMockExtractor(ctrl)
		ri = types.PyFlowRI()
		factory = NewComponentFactory(ri, mockExtractor)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("factory creation", func() {
		It("should initialize component caches for all components", func() {
			Expect(factory.componentCaches).To(HaveLen(3)) // root + 2 children
			Expect(factory.componentCaches).To(HaveKey("pyflow"))
			Expect(factory.componentCaches).To(HaveKey("master"))
			Expect(factory.componentCaches).To(HaveKey("worker"))
		})
	})

	Context("component access", func() {
		It("should get root component", func() {
			component, err := factory.GetRootComponent()
			Expect(err).NotTo(HaveOccurred())
			Expect(component).NotTo(BeNil())
			Expect(component.name).To(Equal("pyflow"))
		})

		It("should get child components by name", func() {
			master, err := factory.GetComponent("master")
			Expect(err).NotTo(HaveOccurred())
			Expect(master.name).To(Equal("master"))

			worker, err := factory.GetComponent("worker")
			Expect(err).NotTo(HaveOccurred())
			Expect(worker.name).To(Equal("worker"))
		})

		It("should return error for non-existent component", func() {
			component, err := factory.GetComponent("non-existent")
			Expect(err).To(HaveOccurred())
			Expect(component).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("component non-existent not found"))
		})

		It("should return error when ResourceInterface is nil", func() {
			// Note: NewComponentFactory panics with nil RI (by design)
			// Testing GetRootComponent with nil RI after factory creation
			factory.ri = nil // Simulate nil RI scenario
			component, err := factory.GetRootComponent()
			Expect(err).To(HaveOccurred())
			Expect(component).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("resource interface is nil"))
		})
	})

	Context("component sharing", func() {
		It("should share the same extractor instance across components", func() {
			master, err := factory.GetComponent("master")
			Expect(err).NotTo(HaveOccurred())

			worker, err := factory.GetComponent("worker")
			Expect(err).NotTo(HaveOccurred())

			Expect(master.extractor).To(Equal(mockExtractor))
			Expect(worker.extractor).To(Equal(mockExtractor))
		})

		It("should provide separate cache instances per component", func() {
			master, err := factory.GetComponent("master")
			Expect(err).NotTo(HaveOccurred())

			worker, err := factory.GetComponent("worker")
			Expect(err).NotTo(HaveOccurred())

			Expect(master.cache).NotTo(BeIdenticalTo(worker.cache))
		})
	})
})
