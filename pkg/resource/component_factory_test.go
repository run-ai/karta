package resource

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/test/types"
	testutils "github.com/run-ai/kai-bolt/test/types/jsonutils"
)

var _ = Describe("ComponentFactory", func() {
	var (
		ctrl         *gomock.Controller
		mockAccessor *MockComponentAccessor
		ri           *v1alpha1.ResourceInterface
		factory      *ComponentFactory
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockAccessor = NewMockComponentAccessor(ctrl)
		ri = types.PyFlowRI()
		factory = NewComponentFactory(ri, mockAccessor)
	})

	AfterEach(func() {
		ctrl.Finish()
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
		It("should share the same accessor instance across components", func() {
			master, err := factory.GetComponent("master")
			Expect(err).NotTo(HaveOccurred())

			worker, err := factory.GetComponent("worker")
			Expect(err).NotTo(HaveOccurred())

			Expect(master.accessor).To(Equal(mockAccessor))
			Expect(worker.accessor).To(Equal(mockAccessor))
		})
	})

	Context("GetChildComponents", func() {
		It("should get all child components", func() {
			components, err := factory.GetChildComponents()
			Expect(err).NotTo(HaveOccurred())
			Expect(components).NotTo(BeNil())
			Expect(components).To(HaveLen(2))

			Expect([]string{components[0].name, components[1].name}).To(ConsistOf("master", "worker"))
		})

		It("should return error when ResourceInterface is nil", func() {
			factory.ri = nil
			components, err := factory.GetChildComponents()
			Expect(err).To(HaveOccurred())
			Expect(components).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("resource interface is nil"))
		})
	})

	Context("GetResource", func() {
		It("should return updated client.Object", func() {
			var object map[string]interface{}
			convertViaJSON(types.NewPyFlowObject(), &object)

			mockAccessor.EXPECT().
				GetObject().
				Return(object, nil)

			result, err := factory.GetResource()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result).To(testutils.BeJSONEquivalentTo(object))
		})

		It("should propagate GetObject errors", func() {
			expectedError := errors.New("failed to get updated data")

			mockAccessor.EXPECT().
				GetObject().
				Return(nil, expectedError)

			result, err := factory.GetResource()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedError))
			Expect(result).To(BeNil())
		})

		It("should return error when object is not client.Object", func() {
			nonClientObject := map[string]interface{}{
				"kind": "SomeKind",
			}

			mockAccessor.EXPECT().
				GetObject().
				Return(nonClientObject, nil)

			result, err := factory.GetResource()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid Kubernetes object"))
			Expect(result).To(BeNil())
		})
	})
})
