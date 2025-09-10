package resource

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/query"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PodQuerier", func() {
	var (
		ctx     context.Context
		testPod corev1.Pod
		querier *PodQuerier
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

		querier = NewPodQuerier(&testPod)
	})

	Describe("ExtractGroupKeys", func() {
		Context("with valid key paths", func() {
			It("should extract single group key", func() {
				keyPaths := []string{".metadata.labels.app"}

				keys, err := querier.ExtractGroupKeys(ctx, keyPaths)

				Expect(err).ToNot(HaveOccurred())
				Expect(keys).To(HaveLen(1))
				Expect(keys[0]).To(Equal("pytorch"))
			})

			It("should extract multiple group keys", func() {
				keyPaths := []string{
					".metadata.labels.app",
					".metadata.labels.version",
					".metadata.namespace",
				}

				keys, err := querier.ExtractGroupKeys(ctx, keyPaths)

				Expect(err).ToNot(HaveOccurred())
				Expect(keys).To(HaveLen(3))
				Expect(keys[0]).To(Equal("pytorch"))
				Expect(keys[1]).To(Equal("v1.0"))
				Expect(keys[2]).To(Equal("default"))
			})
		})

		Context("with empty key paths", func() {
			It("should return empty slice", func() {
				keys, err := querier.ExtractGroupKeys(ctx, []string{})

				Expect(err).ToNot(HaveOccurred())
				Expect(keys).To(BeEmpty())
			})
		})

		Context("with invalid key paths", func() {
			It("should return error for non-existent path", func() {
				keyPaths := []string{".metadata.labels.nonexistent"}

				keys, err := querier.ExtractGroupKeys(ctx, keyPaths)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("query result is empty"))
				Expect(keys).To(BeNil())
			})

			It("should return error for path returning multiple values", func() {
				keyPaths := []string{".metadata.labels | to_entries | .[].key"}

				keys, err := querier.ExtractGroupKeys(ctx, keyPaths)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("expected single query result"))
				Expect(keys).To(BeNil())
			})
		})
	})

	Describe("PassesFilters", func() {
		Context("with no filters", func() {
			It("should return true", func() {
				passed, err := querier.PassesFilters(ctx, []string{})

				Expect(err).ToNot(HaveOccurred())
				Expect(passed).To(BeTrue())
			})
		})

		Context("with valid filters", func() {
			It("should pass single true filter", func() {
				filters := []string{`.metadata.labels.app == "pytorch"`}

				passed, err := querier.PassesFilters(ctx, filters)

				Expect(err).ToNot(HaveOccurred())
				Expect(passed).To(BeTrue())
			})

			It("should pass multiple true filters (AND logic)", func() {
				filters := []string{
					`.metadata.labels.app == "pytorch"`,
					`.metadata.labels.version == "v1.0"`,
					`.metadata.namespace == "default"`,
				}

				passed, err := querier.PassesFilters(ctx, filters)

				Expect(err).ToNot(HaveOccurred())
				Expect(passed).To(BeTrue())
			})

			It("should handle boolean expression filters", func() {
				filters := []string{
					`.metadata.labels | has("app")`,
					`.spec.containers | length > 0`,
				}

				passed, err := querier.PassesFilters(ctx, filters)

				Expect(err).ToNot(HaveOccurred())
				Expect(passed).To(BeTrue())
			})
		})

		Context("with failing filters", func() {
			It("should fail single false filter", func() {
				filters := []string{`.metadata.labels.app == "wrong-app"`}

				passed, err := querier.PassesFilters(ctx, filters)

				Expect(err).ToNot(HaveOccurred())
				Expect(passed).To(BeFalse())
			})

			It("should fail if any filter fails (AND logic)", func() {
				filters := []string{
					`.metadata.labels.app == "pytorch"`,   // true
					`.metadata.labels.version == "wrong"`, // false
					`.metadata.namespace == "default"`,    // true
				}

				passed, err := querier.PassesFilters(ctx, filters)

				Expect(err).ToNot(HaveOccurred())
				Expect(passed).To(BeFalse())
			})

			It("should fail for non-boolean filter result", func() {
				filters := []string{`.metadata.labels.app`} // returns string, not boolean

				passed, err := querier.PassesFilters(ctx, filters)

				Expect(err).ToNot(HaveOccurred())
				Expect(passed).To(BeFalse())
			})
		})

		Context("with invalid filters", func() {
			It("should return error for invalid JQ expression", func() {
				filters := []string{`.invalid.[syntax`}

				passed, err := querier.PassesFilters(ctx, filters)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to evaluate filter"))
				Expect(passed).To(BeFalse())
			})

			It("should return error for filter returning multiple values", func() {
				filters := []string{`.metadata.labels | to_entries | .[].value == "pytorch"`}

				passed, err := querier.PassesFilters(ctx, filters)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("expected single query result"))
				Expect(passed).To(BeFalse())
			})

			It("should return false for filter with nonexistent field comparison", func() {
				filters := []string{`.metadata.labels.nonexistent == "something"`}

				passed, err := querier.PassesFilters(ctx, filters)

				Expect(err).ToNot(HaveOccurred())
				Expect(passed).To(BeFalse())
			})
		})
	})

	Describe("Matches", func() {
		Context("when selector is nil", func() {
			It("should return false", func() {
				matches, err := querier.Matches(ctx, nil)
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

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return false for non-existing label keys", func() {
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.nonexistent",
					Value:   nil,
				}

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeFalse())
			})

			It("should return true for existing annotation keys", func() {
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.annotations.config",
					Value:   nil,
				}

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return true for existing nested paths", func() {
				selector := &v1alpha1.PodSelector{
					KeyPath: ".spec.containers[0].name",
					Value:   nil,
				}

				matches, err := querier.Matches(ctx, selector)
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

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return false for non-matching label values", func() {
				value := "master"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.component",
					Value:   &value,
				}

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeFalse())
			})

			It("should return true for matching annotation values", func() {
				value := "high-memory"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.annotations.config",
					Value:   &value,
				}

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return false for non-matching annotation values", func() {
				value := "low-memory"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.annotations.config",
					Value:   &value,
				}

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeFalse())
			})

			It("should handle special characters in values", func() {
				// Update the test pod to have a label with special characters
				testPod.Labels["special"] = "value-with-special_chars.and:colons"
				querier = NewPodQuerier(&testPod)

				value := "value-with-special_chars.and:colons"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.special",
					Value:   &value,
				}

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should handle values with quotes", func() {
				// Update the test pod to have a label with quotes
				testPod.Labels["quotes"] = `value-with-"quotes"`
				querier = NewPodQuerier(&testPod)

				value := `value-with-"quotes"`
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.quotes",
					Value:   &value,
				}

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should return false for non-existing keys with values", func() {
				value := "any-value"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.labels.nonexistent",
					Value:   &value,
				}

				matches, err := querier.Matches(ctx, selector)
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

				matches, err := querier.Matches(ctx, selector)
				Expect(err).ToNot(HaveOccurred())
				Expect(matches).To(BeTrue())
			})

			It("should work with object navigation", func() {
				value := "default"
				selector := &v1alpha1.PodSelector{
					KeyPath: ".metadata.namespace",
					Value:   &value,
				}

				matches, err := querier.Matches(ctx, selector)
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

				matches, err := querier.Matches(ctx, selector)
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&query.JQParseError{}))
				Expect(matches).To(BeFalse())
			})
		})
	})
})
