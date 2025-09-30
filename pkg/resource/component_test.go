package resource

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
		ctx           context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockExtractor = NewMockExtractor(ctrl)
		ctx = context.Background()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("Basic Properties", func() {
		var component *Component

		BeforeEach(func() {
			definition := v1alpha1.ComponentDefinition{
				Name: "test-component",
				Kind: &v1alpha1.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				PodSelector: &v1alpha1.PodSelector{
					ComponentTypeSelector: &v1alpha1.ComponentTypeSelector{
						KeyPath: ".metadata.labels.role",
						Value:   stringPtr("worker"),
					},
				},
			}

			component = &Component{
				name:       "test-component",
				definition: definition,
				extractor:  mockExtractor,
			}
		})

		It("should return component name", func() {
			Expect(component.Name()).To(Equal("test-component"))
		})

		It("should return component definition", func() {
			Expect(component.Definition()).To(Equal(component.definition))
		})

		It("should return GroupVersionKind when defined", func() {
			gvk := component.Kind()
			Expect(gvk).NotTo(BeNil())
			Expect(gvk.Group).To(Equal("apps"))
			Expect(gvk.Version).To(Equal("v1"))
			Expect(gvk.Kind).To(Equal("Deployment"))
		})

		It("should return nil Kind when not defined", func() {
			componentWithoutKind := &Component{
				definition: v1alpha1.ComponentDefinition{Name: "no-kind"},
			}
			Expect(componentWithoutKind.Kind()).To(BeNil())
		})

		It("should return pod selector", func() {
			selector := component.GetPodSelector()
			Expect(selector).NotTo(BeNil())
			Expect(selector.ComponentTypeSelector.KeyPath).To(Equal(".metadata.labels.role"))
			Expect(*selector.ComponentTypeSelector.Value).To(Equal("worker"))
		})

		It("should return nil pod selector when not defined", func() {
			componentWithoutSelector := &Component{
				definition: v1alpha1.ComponentDefinition{Name: "no-selector"},
			}
			Expect(componentWithoutSelector.GetPodSelector()).To(BeNil())
		})
	})

	Context("Pod Definition Detection", func() {
		It("should detect pod definition with PodTemplateSpecPath", func() {
			component := &Component{
				definition: v1alpha1.ComponentDefinition{
					SpecDefinition: &v1alpha1.SpecDefinition{
						PodTemplateSpecPath: stringPtr(".spec.template"),
					},
				},
			}
			Expect(component.HasPodDefinition()).To(BeTrue())
		})

		It("should detect pod definition with PodSpecPath", func() {
			component := &Component{
				definition: v1alpha1.ComponentDefinition{
					SpecDefinition: &v1alpha1.SpecDefinition{
						PodSpecPath: stringPtr(".spec.podSpec"),
					},
				},
			}
			Expect(component.HasPodDefinition()).To(BeTrue())
		})

		It("should detect pod definition with FragmentedPodSpecDefinition", func() {
			component := &Component{
				definition: v1alpha1.ComponentDefinition{
					SpecDefinition: &v1alpha1.SpecDefinition{
						FragmentedPodSpecDefinition: &v1alpha1.FragmentedPodSpecDefinition{},
					},
				},
			}
			Expect(component.HasPodDefinition()).To(BeTrue())
		})

		It("should return false when no pod definition exists", func() {
			component := &Component{
				definition: v1alpha1.ComponentDefinition{
					SpecDefinition: &v1alpha1.SpecDefinition{
						// No pod-related paths
					},
				},
			}
			Expect(component.HasPodDefinition()).To(BeFalse())
		})

		It("should return false when SpecDefinition is nil", func() {
			component := &Component{
				definition: v1alpha1.ComponentDefinition{
					// No SpecDefinition
				},
			}
			Expect(component.HasPodDefinition()).To(BeFalse())
		})
	})

	Context("Instance ID Logic", func() {
		It("should return true for HasInstanceIdDefinition when InstanceIdPath is defined", func() {
			component := &Component{
				definition: v1alpha1.ComponentDefinition{
					InstanceIdPath: stringPtr(".spec.jobs[].name"),
				},
			}
			Expect(component.HasInstanceIdDefinition()).To(BeTrue())
		})

		It("should return false for HasInstanceIdDefinition when InstanceIdPath is not defined", func() {
			component := &Component{
				definition: v1alpha1.ComponentDefinition{
					// No InstanceIdPath
				},
			}
			Expect(component.HasInstanceIdDefinition()).To(BeFalse())
		})

		Context("GetInstanceIds", func() {
			var component *Component

			BeforeEach(func() {
				component = &Component{
					name:       "test-component",
					definition: v1alpha1.ComponentDefinition{Name: "test-component"},
					extractor:  mockExtractor,
				}
			})

			It("should return extracted instance IDs for multi-instance component", func() {
				expectedIds := []string{"job-1", "job-2", "job-3"}

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(expectedIds, nil).
					Times(1)

				result, err := component.GetInstanceIds(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedIds))
			})

			It("should return empty string for single-instance component", func() {
				expectedError := DefinitionNotFoundError("no instance id path defined")

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(nil, expectedError).
					Times(1)

				result, err := component.GetInstanceIds(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal([]string{""}))
			})

			It("should propagate non-DefinitionNotFoundError errors", func() {
				expectedError := errors.New("extraction failed")

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(nil, expectedError).
					Times(1)

				result, err := component.GetInstanceIds(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to extract instance ids"))
				Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
				Expect(result).To(BeNil())
			})
		})
	})

	Context("Extraction Methods", func() {
		var component *Component

		BeforeEach(func() {
			component = &Component{
				name:       "test-component",
				definition: v1alpha1.ComponentDefinition{Name: "test-component"},
				extractor:  mockExtractor,
			}
		})

		Context("GetPodTemplateSpec", func() {
			It("should zip instance IDs with pod template specs for multi-instance", func() {
				instanceIds := []string{"job-1", "job-2"}
				templates := []corev1.PodTemplateSpec{
					{ObjectMeta: metav1.ObjectMeta{Name: "template-1"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "template-2"}},
				}

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(instanceIds, nil)

				mockExtractor.EXPECT().
					ExtractPodTemplateSpec(ctx, component.definition).
					Return(templates, nil)

				result, err := component.GetPodTemplateSpec(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				Expect(result["job-1"]).To(Equal(templates[0]))
				Expect(result["job-2"]).To(Equal(templates[1]))
			})

			It("should handle single instance with empty string ID", func() {
				instanceIds := []string{""}
				templates := []corev1.PodTemplateSpec{
					{ObjectMeta: metav1.ObjectMeta{Name: "single-template"}},
				}

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(instanceIds, nil)

				mockExtractor.EXPECT().
					ExtractPodTemplateSpec(ctx, component.definition).
					Return(templates, nil)

				result, err := component.GetPodTemplateSpec(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[""]).To(Equal(templates[0]))
			})

			It("should propagate instance ID extraction errors", func() {
				expectedError := errors.New("instance extraction failed")

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(nil, expectedError)

				result, err := component.GetPodTemplateSpec(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get instance ids"))
				Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
				Expect(result).To(BeNil())
			})

			It("should propagate template extraction errors", func() {
				instanceIds := []string{"job-1"}
				expectedError := errors.New("template extraction failed")

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(instanceIds, nil)

				mockExtractor.EXPECT().
					ExtractPodTemplateSpec(ctx, component.definition).
					Return(nil, expectedError)

				result, err := component.GetPodTemplateSpec(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to extract pod template specs"))
				Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
				Expect(result).To(BeNil())
			})

			It("should return error when counts mismatch", func() {
				instanceIds := []string{"job-1", "job-2"}
				templates := []corev1.PodTemplateSpec{
					{ObjectMeta: metav1.ObjectMeta{Name: "only-one"}},
				}

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(instanceIds, nil)

				mockExtractor.EXPECT().
					ExtractPodTemplateSpec(ctx, component.definition).
					Return(templates, nil)

				result, err := component.GetPodTemplateSpec(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("instance ids count (2) does not match results count (1)"))
				Expect(result).To(BeNil())
			})
		})

		Context("GetScale", func() {
			It("should zip instance IDs with scales", func() {
				instanceIds := []string{"worker-1", "worker-2"}
				scales := []Scale{
					{Replicas: int32Ptr(3)},
					{Replicas: int32Ptr(5)},
				}

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(instanceIds, nil)

				mockExtractor.EXPECT().
					ExtractScale(ctx, component.definition).
					Return(scales, nil)

				result, err := component.GetScale(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(2))
				Expect(result["worker-1"]).To(Equal(scales[0]))
				Expect(result["worker-2"]).To(Equal(scales[1]))
			})
		})

		Context("GetPodSpec", func() {
			It("should zip instance IDs with pod specs", func() {
				instanceIds := []string{"spec-1"}
				specs := []corev1.PodSpec{
					{Containers: []corev1.Container{{Name: "test-container"}}},
				}

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(instanceIds, nil)

				mockExtractor.EXPECT().
					ExtractPodSpec(ctx, component.definition).
					Return(specs, nil)

				result, err := component.GetPodSpec(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result["spec-1"]).To(Equal(specs[0]))
			})
		})

		Context("GetPodMetadata", func() {
			It("should zip instance IDs with pod metadata", func() {
				instanceIds := []string{"meta-1"}
				metadata := []metav1.ObjectMeta{
					{Name: "test-pod", Labels: map[string]string{"app": "test"}},
				}

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(instanceIds, nil)

				mockExtractor.EXPECT().
					ExtractPodMetadata(ctx, component.definition).
					Return(metadata, nil)

				result, err := component.GetPodMetadata(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result["meta-1"]).To(Equal(metadata[0]))
			})
		})

		Context("GetFragmentedPodSpec", func() {
			It("should zip instance IDs with fragmented pod specs", func() {
				instanceIds := []string{"frag-1"}
				fragSpecs := []FragmentedPodSpec{
					{SchedulerName: "test-scheduler"},
				}

				mockExtractor.EXPECT().
					ExtractInstanceIds(ctx, component.definition).
					Return(instanceIds, nil)

				mockExtractor.EXPECT().
					ExtractFragmentedPodSpec(ctx, component.definition).
					Return(fragSpecs, nil)

				result, err := component.GetFragmentedPodSpec(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result["frag-1"]).To(Equal(fragSpecs[0]))
			})
		})
	})
})

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}
