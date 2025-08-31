package rid_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/utils/rid"
	"github.com/run-ai/kai-bolt/pkg/utils/rid/query"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PodMatcher", func() {
	var (
		ctx     context.Context
		testPod corev1.Pod
		matcher *rid.PodMatcher
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create a test pod with labels and annotations
		testPod = corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				Labels: map[string]string{
					"component":   "worker",
					"app":         "pytorch",
					"version":     "v1.0",
					"environment": "production",
				},
				Annotations: map[string]string{
					"config": "high-memory",
					"owner":  "team-ai",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "main",
						Image: "pytorch:latest",
					},
				},
			},
		}

		matcher = rid.NewPodMatcher(testPod)
	})

	Describe("NewPodMatcher", func() {
		It("should create a new pod matcher", func() {
			Expect(matcher).ToNot(BeNil())
		})
	})

	Describe("Matches", func() {
		Context("when selector is nil", func() {
			It("should return false", func() {
				matches, err := matcher.Matches(ctx, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeFalse())
			})
		})

		Context("when checking key existence (Value is nil)", func() {
			It("should return true for existing label keys", func() {
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.component",
					Value:   nil,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return false for non-existing label keys", func() {
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.nonexistent",
					Value:   nil,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeFalse())
			})

			It("should return true for existing annotation keys", func() {
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.annotations.config",
					Value:   nil,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return true for existing nested paths", func() {
				selector := &v1alpha1.PodSelector{
					KeyPath: ".spec.containers[0].name",
					Value:   nil,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})
		})

		Context("when checking key-value pairs (Value is specified)", func() {
			It("should return true for matching label values", func() {
				value := "worker"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.component",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return false for non-matching label values", func() {
				value := "master"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.component",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeFalse())
			})

			It("should return true for matching annotation values", func() {
				value := "high-memory"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.annotations.config",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return false for non-matching annotation values", func() {
				value := "low-memory"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.annotations.config",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeFalse())
			})

			It("should handle special characters in values", func() {
				// Update the test pod to have a label with special characters
				testPod.Labels["special"] = "value-with-special_chars.and:colons"
				matcher = rid.NewPodMatcher(testPod)

				value := "value-with-special_chars.and:colons"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.special",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should handle values with quotes", func() {
				// Update the test pod to have a label with quotes
				testPod.Labels["quotes"] = `value-with-"quotes"`
				matcher = rid.NewPodMatcher(testPod)

				value := `value-with-"quotes"`
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.quotes",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return false for non-existing keys with values", func() {
				value := "any-value"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.nonexistent",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeFalse())
			})
		})

		Context("when using complex JQ paths", func() {
			It("should work with array indexing", func() {
				value := "main"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".spec.containers[0].name",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should work with object navigation", func() {
				value := "default"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.namespace",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})
		})

		Context("when JQ expression is invalid", func() {
			It("should return an error for invalid paths", func() {
				value := "any"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".invalid[[[syntax",
					Value:   &value,
				}

				matches, err := matcher.Matches(ctx, selector)
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&query.JQParseError{}))
				Expect(matches).To(BeFalse())
			})
		})
	})
})
