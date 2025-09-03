package rid

import (
	"context"
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Component", func() {
	var (
		ctrl          *gomock.Controller
		mockExtractor *MockExtractor
		component     *Component
		ctx           context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockExtractor = NewMockExtractor(ctrl)
		ctx = context.WithValue(context.Background(), "test-key", "test-value")

		// Create a component with pod template spec definition
		definition := v1alpha1.ComponentDefinition{
			Name: "test-component",
			SpecDefinition: &v1alpha1.SpecDefinition{
				PodTemplateSpecPath: func() *string { s := ".spec.template"; return &s }(),
			},
			PodSelector: &v1alpha1.PodSelector{
				KeyPath: ".metadata.labels.role",
				Value:   func() *string { s := "worker"; return &s }(),
			},
		}

		component = &Component{
			name:       "test-component",
			definition: definition,
			extractor:  mockExtractor,
			cache:      &ComponentCache{},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("basic component properties", func() {
		It("should return component name", func() {
			Expect(component.Name()).To(Equal("test-component"))
		})

		It("should return component definition", func() {
			Expect(component.Definition()).To(Equal(component.definition))
		})

		It("should return pod selector", func() {
			selector := component.GetPodSelector()
			Expect(selector).NotTo(BeNil())
			Expect(selector.KeyPath).To(Equal(".metadata.labels.role"))
			Expect(*selector.Value).To(Equal("worker"))
		})

		It("should return nil pod selector when not defined", func() {
			componentWithoutSelector := &Component{
				definition: v1alpha1.ComponentDefinition{
					Name: "no-selector",
					// No PodSelector
				},
			}
			Expect(componentWithoutSelector.GetPodSelector()).To(BeNil())
		})
	})

	Context("Kind method", func() {
		It("should return GroupVersionKind when Kind is defined", func() {
			componentWithKind := &Component{
				definition: v1alpha1.ComponentDefinition{
					Name: "with-kind",
					Kind: &v1alpha1.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			}

			gvk := componentWithKind.Kind()
			Expect(gvk).NotTo(BeNil())
			Expect(gvk.Group).To(Equal("apps"))
			Expect(gvk.Version).To(Equal("v1"))
			Expect(gvk.Kind).To(Equal("Deployment"))
		})

		It("should return nil when Kind is not defined", func() {
			componentWithoutKind := &Component{
				definition: v1alpha1.ComponentDefinition{
					Name: "no-kind",
					// No Kind
				},
			}
			Expect(componentWithoutKind.Kind()).To(BeNil())
		})
	})

	Context("HasPodDefinition method", func() {
		It("should return true when PodTemplateSpecPath is set", func() {
			Expect(component.HasPodDefinition()).To(BeTrue())
		})

		It("should return true when PodSpecPath is set", func() {
			componentWithPodSpec := &Component{
				definition: v1alpha1.ComponentDefinition{
					Name: "with-pod-spec",
					SpecDefinition: &v1alpha1.SpecDefinition{
						PodSpecPath: func() *string { s := ".spec.podSpec"; return &s }(),
					},
				},
			}
			Expect(componentWithPodSpec.HasPodDefinition()).To(BeTrue())
		})

		It("should return true when FragmentedPodSpecDefinition is set", func() {
			componentWithFragmented := &Component{
				definition: v1alpha1.ComponentDefinition{
					Name: "with-fragmented",
					SpecDefinition: &v1alpha1.SpecDefinition{
						FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{
							LabelsPath: func() *string { s := ".spec.labels"; return &s }(),
						},
					},
				},
			}
			Expect(componentWithFragmented.HasPodDefinition()).To(BeTrue())
		})

		It("should return false when no pod paths are set", func() {
			componentWithoutPods := &Component{
				definition: v1alpha1.ComponentDefinition{
					Name: "no-pods",
					// No SpecDefinition
				},
			}
			Expect(componentWithoutPods.HasPodDefinition()).To(BeFalse())
		})

		It("should return false when SpecDefinition exists but no pod paths", func() {
			componentWithEmptySpec := &Component{
				definition: v1alpha1.ComponentDefinition{
					Name:           "empty-spec",
					SpecDefinition: &v1alpha1.SpecDefinition{
						// No pod-related paths
					},
				},
			}
			Expect(componentWithEmptySpec.HasPodDefinition()).To(BeFalse())
		})
	})

	Context("GetPodTemplateSpec method", func() {
		It("should delegate to extractor and cache results", func() {
			expectedTemplates := []corev1.PodTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "test"},
					},
				},
			}

			mockExtractor.EXPECT().
				ExtractPodTemplateSpec(gomock.Eq(ctx), gomock.Eq(component.definition)).
				Return(expectedTemplates, nil).
				Times(1)

			// First call - hits the extractor
			result1, err := component.GetPodTemplateSpec(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result1).To(Equal(expectedTemplates))

			// Second call - should use cache
			result2, err := component.GetPodTemplateSpec(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result2).To(Equal(expectedTemplates))
		})

		It("should propagate extractor errors", func() {
			expectedError := errors.New("template extraction failed")
			mockExtractor.EXPECT().
				ExtractPodTemplateSpec(gomock.Eq(ctx), gomock.Any()).
				Return(nil, expectedError).
				Times(1)

			result, err := component.GetPodTemplateSpec(ctx)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err).To(Equal(expectedError))
		})
	})

	Context("GetPodSpec method", func() {
		It("should delegate to extractor and cache results", func() {
			expectedSpecs := []corev1.PodSpec{
				{
					Containers: []corev1.Container{
						{Name: "test-container", Image: "test:latest"},
					},
				},
			}

			mockExtractor.EXPECT().
				ExtractPodSpec(gomock.Eq(ctx), gomock.Eq(component.definition)).
				Return(expectedSpecs, nil).
				Times(1)

			result1, err := component.GetPodSpec(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result1).To(Equal(expectedSpecs))

			// Second call should use cache
			result2, err := component.GetPodSpec(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result2).To(Equal(expectedSpecs))
		})

		It("should propagate extractor errors", func() {
			expectedError := errors.New("pod spec extraction failed")
			mockExtractor.EXPECT().
				ExtractPodSpec(gomock.Eq(ctx), gomock.Any()).
				Return(nil, expectedError).
				Times(1)

			result, err := component.GetPodSpec(ctx)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err).To(Equal(expectedError))
		})
	})

	Context("GetPodMetadata method", func() {
		It("should delegate to extractor and cache results", func() {
			expectedMetadata := []metav1.ObjectMeta{
				{
					Labels: map[string]string{"app": "test", "role": "worker"},
				},
			}

			mockExtractor.EXPECT().
				ExtractPodMetadata(gomock.Any(), gomock.Eq(component.definition)).
				Return(expectedMetadata, nil).
				Times(1)

			result1, err := component.GetPodMetadata(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result1).To(Equal(expectedMetadata))

			// Second call should use cache
			result2, err := component.GetPodMetadata(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result2).To(Equal(expectedMetadata))
		})

		It("should propagate extractor errors", func() {
			expectedError := errors.New("metadata extraction failed")
			mockExtractor.EXPECT().
				ExtractPodMetadata(gomock.Eq(ctx), gomock.Any()).
				Return(nil, expectedError).
				Times(1)

			result, err := component.GetPodMetadata(ctx)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err).To(Equal(expectedError))
		})
	})

	Context("GetFragmentedPodSpec method", func() {
		It("should delegate to extractor and cache results", func() {
			expectedFragmented := []FragmentedPodSpec{
				{
					Labels: map[string]string{"app": "test"},
					Containers: []corev1.Container{
						{Name: "test-container", Image: "test:latest"},
					},
				},
			}

			mockExtractor.EXPECT().
				ExtractFragmentedPodSpec(gomock.Any(), gomock.Eq(component.definition)).
				Return(expectedFragmented, nil).
				Times(1)

			result1, err := component.GetFragmentedPodSpec(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result1).To(Equal(expectedFragmented))

			// Second call should use cache
			result2, err := component.GetFragmentedPodSpec(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result2).To(Equal(expectedFragmented))
		})

		It("should propagate extractor errors", func() {
			expectedError := errors.New("fragmented spec extraction failed")
			mockExtractor.EXPECT().
				ExtractFragmentedPodSpec(gomock.Eq(ctx), gomock.Any()).
				Return(nil, expectedError).
				Times(1)

			result, err := component.GetFragmentedPodSpec(ctx)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err).To(Equal(expectedError))
		})
	})

	Context("GetScale method", func() {
		It("should delegate to extractor and cache results", func() {
			expectedScales := []Scale{
				{
					Replicas:    func() *int32 { r := int32(3); return &r }(),
					MinReplicas: func() *int32 { r := int32(1); return &r }(),
					MaxReplicas: func() *int32 { r := int32(10); return &r }(),
				},
			}

			mockExtractor.EXPECT().
				ExtractScale(gomock.Any(), gomock.Eq(component.definition)).
				Return(expectedScales, nil).
				Times(1)

			result1, err := component.GetScale(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result1).To(Equal(expectedScales))

			// Second call should use cache
			result2, err := component.GetScale(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result2).To(Equal(expectedScales))
		})

		It("should propagate extractor errors", func() {
			expectedError := errors.New("scale extraction failed")
			mockExtractor.EXPECT().
				ExtractScale(gomock.Eq(ctx), gomock.Any()).
				Return(nil, expectedError).
				Times(1)

			result, err := component.GetScale(ctx)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err).To(Equal(expectedError))
		})
	})

	Context("caching behavior across methods", func() {
		It("should cache each extraction method independently", func() {
			expectedTemplates := []corev1.PodTemplateSpec{{}}
			expectedScales := []Scale{{Replicas: func() *int32 { r := int32(1); return &r }()}}

			mockExtractor.EXPECT().
				ExtractPodTemplateSpec(gomock.Eq(ctx), gomock.Any()).
				Return(expectedTemplates, nil).
				Times(1)

			mockExtractor.EXPECT().
				ExtractScale(gomock.Eq(ctx), gomock.Any()).
				Return(expectedScales, nil).
				Times(1)

			// Each method should be cached separately
			_, err := component.GetPodTemplateSpec(ctx)
			Expect(err).NotTo(HaveOccurred())

			_, err = component.GetScale(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Second calls should use cache
			_, err = component.GetPodTemplateSpec(ctx)
			Expect(err).NotTo(HaveOccurred())

			_, err = component.GetScale(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not cache failed extractions", func() {
			expectedError := errors.New("extraction failed")
			successResult := []corev1.PodTemplateSpec{{}}

			// First call fails
			mockExtractor.EXPECT().
				ExtractPodTemplateSpec(gomock.Eq(ctx), gomock.Any()).
				Return(nil, expectedError).
				Times(1)

			// Second call succeeds
			mockExtractor.EXPECT().
				ExtractPodTemplateSpec(gomock.Eq(ctx), gomock.Any()).
				Return(successResult, nil).
				Times(1)

			// First call - should fail
			_, err := component.GetPodTemplateSpec(ctx)
			Expect(err).To(HaveOccurred())

			// Second call - should retry (not use cached error)
			result, err := component.GetPodTemplateSpec(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(successResult))
		})
	})

})
