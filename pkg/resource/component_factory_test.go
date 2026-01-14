package resource

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

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

	Context("IsContainSpecDefinition", func() {
		Context("components with spec definitions", func() {
			It("should return true when child components have PodTemplateSpecPath", func() {
				// PyFlowRI has master/worker with PodTemplateSpecPath
				result, err := factory.IsContainSpecDefinition()
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should return true when child components have PodSpecPath", func() {
				// JobGroupRI has job component with PodSpecPath
				factory.ri = types.JobGroupRI()
				factory = NewComponentFactory(factory.ri, mockAccessor)

				result, err := factory.IsContainSpecDefinition()
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should return true when child components have FragmentedPodSpecDefinition", func() {
				// ReactorRI has service component with FragmentedPodSpecDefinition
				factory.ri = types.ReactorRI()
				factory = NewComponentFactory(factory.ri, mockAccessor)

				result, err := factory.IsContainSpecDefinition()
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should return true when root component has spec definition", func() {
				factory.ri = riWithRootSpecOnly()
				factory = NewComponentFactory(factory.ri, mockAccessor)

				result, err := factory.IsContainSpecDefinition()
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})
		})

		Context("components without spec definitions", func() {
			It("should return false when all components have nil SpecDefinition", func() {
				factory.ri = riWithNoSpecs()
				factory = NewComponentFactory(factory.ri, mockAccessor)

				result, err := factory.IsContainSpecDefinition()
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})

			It("should return false when all components have empty SpecDefinition", func() {
				factory.ri = riWithEmptySpecs()
				factory = NewComponentFactory(factory.ri, mockAccessor)

				result, err := factory.IsContainSpecDefinition()
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})
	})
})

// Helper functions for test ResourceInterface instances

// riWithNoSpecs creates a ResourceInterface where all components have nil SpecDefinition
func riWithNoSpecs() *v1alpha1.ResourceInterface {
	return &v1alpha1.ResourceInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name: "no-specs",
		},
		Spec: v1alpha1.ResourceInterfaceSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "root",
					Kind: &v1alpha1.GroupVersionKind{
						Group:   "test.example.com",
						Version: "v1",
						Kind:    "NoSpecs",
					},
					// SpecDefinition is nil
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "child1",
						OwnerRef: ptr.To("root"),
						// SpecDefinition is nil
					},
					{
						Name:     "child2",
						OwnerRef: ptr.To("root"),
						// SpecDefinition is nil
					},
				},
			},
		},
	}
}

// riWithEmptySpecs creates a ResourceInterface where all components have empty SpecDefinition
func riWithEmptySpecs() *v1alpha1.ResourceInterface {
	return &v1alpha1.ResourceInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty-specs",
		},
		Spec: v1alpha1.ResourceInterfaceSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "root",
					Kind: &v1alpha1.GroupVersionKind{
						Group:   "test.example.com",
						Version: "v1",
						Kind:    "EmptySpecs",
					},
					SpecDefinition: &v1alpha1.SpecDefinition{
						// All fields nil
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:           "child1",
						OwnerRef:       ptr.To("root"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							// All fields nil
						},
					},
					{
						Name:           "child2",
						OwnerRef:       ptr.To("root"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							// All fields nil
						},
					},
				},
			},
		},
	}
}

// riWithRootSpecOnly creates a ResourceInterface where only root has spec definition
func riWithRootSpecOnly() *v1alpha1.ResourceInterface {
	return &v1alpha1.ResourceInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name: "root-spec-only",
		},
		Spec: v1alpha1.ResourceInterfaceSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "root",
					Kind: &v1alpha1.GroupVersionKind{
						Group:   "test.example.com",
						Version: "v1",
						Kind:    "RootSpecOnly",
					},
					SpecDefinition: &v1alpha1.SpecDefinition{
						PodTemplateSpecPath: ptr.To(".spec.template"),
					},
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "child1",
						OwnerRef: ptr.To("root"),
						// SpecDefinition is nil
					},
					{
						Name:     "child2",
						OwnerRef: ptr.To("root"),
						// SpecDefinition is nil
					},
				},
			},
		},
	}
}

// riWithPartialChildSpecs creates a ResourceInterface where only one child has spec definition
func riWithPartialChildSpecs() *v1alpha1.ResourceInterface {
	return &v1alpha1.ResourceInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name: "partial-child-specs",
		},
		Spec: v1alpha1.ResourceInterfaceSpec{
			StructureDefinition: v1alpha1.StructureDefinition{
				RootComponent: v1alpha1.ComponentDefinition{
					Name: "root",
					Kind: &v1alpha1.GroupVersionKind{
						Group:   "test.example.com",
						Version: "v1",
						Kind:    "PartialChildSpecs",
					},
					// SpecDefinition is nil
				},
				ChildComponents: []v1alpha1.ComponentDefinition{
					{
						Name:     "child-with-spec",
						OwnerRef: ptr.To("root"),
						SpecDefinition: &v1alpha1.SpecDefinition{
							PodSpecPath: ptr.To(".spec.child1.podSpec"),
						},
					},
					{
						Name:     "child-without-spec",
						OwnerRef: ptr.To("root"),
						// SpecDefinition is nil
					},
				},
			},
		},
	}
}
