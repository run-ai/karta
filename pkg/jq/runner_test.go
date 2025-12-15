package jq

import (
	"context"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("JqEvaluator", func() {
	var (
		ctx        context.Context
		evaluator  *JqEvaluator
		testObject map[string]any
	)

	BeforeEach(func() {
		ctx = context.Background()
		testObject = map[string]any{
			"metadata": map[string]any{
				"name":      "test-pod",
				"namespace": "default",
				"labels": map[string]any{
					"app":       "web",
					"component": "frontend",
				},
			},
			"spec": map[string]any{
				"containers": []any{
					map[string]any{
						"name":  "web",
						"image": "nginx:1.20",
						"ports": []any{
							map[string]any{"containerPort": 80},
							map[string]any{"containerPort": 443},
						},
					},
					map[string]any{
						"name":  "sidecar",
						"image": "busybox:latest",
					},
				},
			},
		}
		evaluator = NewDefaultJqEvaluator(testObject)
	})

	Describe("Basic JQ evaluation", func() {
		It("should evaluate simple path expressions", func() {
			results, err := evaluator.Evaluate(ctx, ".metadata.name")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(Equal("test-pod"))
		})

		It("should evaluate array access", func() {
			results, err := evaluator.Evaluate(ctx, ".spec.containers[0].name")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(Equal("web"))
		})

		It("should evaluate array iteration", func() {
			results, err := evaluator.Evaluate(ctx, ".spec.containers[].name")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(2))
			Expect(results).To(ConsistOf("web", "sidecar"))
		})

		It("should handle nested array iteration", func() {
			results, err := evaluator.Evaluate(ctx, ".spec.containers[0].ports[].containerPort")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(2))
			Expect(results).To(ConsistOf(float64(80), float64(443)))
		})

		It("should handle non-existent paths", func() {
			results, err := evaluator.Evaluate(ctx, ".metadata.nonexistent")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(BeNil())
		})

		It("should handle empty results", func() {
			results, err := evaluator.Evaluate(ctx, ".spec.containers[] | select(.name == \"notfound\")")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(BeEmpty())
		})
	})

	Describe("JQ filters and expressions", func() {
		It("should evaluate boolean expressions", func() {
			results, err := evaluator.Evaluate(ctx, ".metadata.name == \"test-pod\"")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(BeTrue())
		})

		It("should evaluate select filters", func() {
			results, err := evaluator.Evaluate(ctx, ".spec.containers[] | select(.name == \"web\") | .image")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(Equal("nginx:1.20"))
		})

		It("should evaluate map operations", func() {
			results, err := evaluator.Evaluate(ctx, ".spec.containers | map(.name)")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(ConsistOf("web", "sidecar"))
		})

		It("should handle complex expressions", func() {
			results, err := evaluator.Evaluate(ctx, ".spec.containers | length")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(BeNumerically("==", 2))
		})
	})

	Describe("Error handling", func() {
		It("should return JQParseError for invalid syntax", func() {
			_, err := evaluator.Evaluate(ctx, ".invalid[[[syntax")
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&JQParseError{}))
		})

		It("should handle context cancellation", func() {
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()

			_, err := evaluator.Evaluate(cancelCtx, ".metadata.name")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("context canceled"))
		})
	})

	Describe("Result count limits", func() {
		var (
			largeObject map[string]any
			limitedEval *JqEvaluator
			maxResults  = 5
		)

		BeforeEach(func() {
			// Create object with many results programmatically
			largeObject = map[string]any{
				"items": make([]any, 0),
			}

			// Generate 100 items
			items := largeObject["items"].([]any)
			for i := 0; i < 100; i++ {
				items = append(items, map[string]any{
					"id":    i,
					"name":  "item-" + string(rune('a'+i%26)),
					"value": i * 10,
				})
			}
			largeObject["items"] = items

			limitedEval = NewJqEvaluator(largeObject, &maxResults, nil)
		})

		It("should respect max results limit", func() {
			results, err := limitedEval.Evaluate(ctx, ".items[].id")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("query results exceed the allowed number 5"))
			Expect(results).To(BeNil())
		})

		It("should allow results under the limit", func() {
			results, err := limitedEval.Evaluate(ctx, ".items[0:3][].id")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(3))
			Expect(results).To(ConsistOf(float64(0), float64(1), float64(2)))
		})
	})

	Describe("Timeout limits", func() {
		var (
			fastTimeoutEval *JqEvaluator
			maxResults      = 1000
			timeoutMs       = 1 // Very short timeout
		)

		BeforeEach(func() {
			// Create object that might cause slow evaluation
			slowObject := map[string]any{
				"data": make([]any, 0),
			}

			// Generate data for potential slow operations
			data := slowObject["data"].([]any)
			for i := 0; i < 10000; i++ {
				data = append(data, map[string]any{
					"id":     i,
					"nested": strings.Repeat("x", 100), // Large strings
				})
			}
			slowObject["data"] = data

			fastTimeoutEval = NewJqEvaluator(slowObject, &maxResults, &timeoutMs)
		})

		It("should respect timeout limits for complex operations", func() {
			// With 1ms timeout and 10,000 items, this should deterministically timeout
			_, err := fastTimeoutEval.Evaluate(ctx, ".data | map(select(.nested | length > 50)) | length")

			// 1ms timeout should be too short for processing 10,000 items
			Expect(err).To(HaveOccurred())

			// Should be a JQExecutionError wrapping context.DeadlineExceeded
			var jqExecError *JQExecutionError
			Expect(errors.As(err, &jqExecError)).To(BeTrue())

			// The wrapped error should be context.DeadlineExceeded
			Expect(errors.Is(jqExecError.Unwrap(), context.DeadlineExceeded)).To(BeTrue())
		})

		It("should work with longer timeout for the same operation", func() {
			longerTimeoutMs := 10000
			longerTimeoutEval := NewJqEvaluator(testObject, &maxResults, &longerTimeoutMs)

			results, err := longerTimeoutEval.Evaluate(ctx, ".spec.containers[].name")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(2))
		})
	})

	Describe("JSON conversion", func() {
		It("should handle different source object types", func() {
			// Test with string
			stringEval := NewDefaultJqEvaluator("test-string")
			results, err := stringEval.Evaluate(ctx, ". | length")
			Expect(err).ToNot(HaveOccurred())
			Expect(results[0]).To(BeNumerically("==", 11))

			// Test with number
			numberEval := NewDefaultJqEvaluator(42)
			results, err = numberEval.Evaluate(ctx, ". + 8")
			Expect(err).ToNot(HaveOccurred())
			Expect(results[0]).To(BeNumerically("==", 50))

			// Test with array
			arrayEval := NewDefaultJqEvaluator([]any{1, 2, 3})
			results, err = arrayEval.Evaluate(ctx, ". | length")
			Expect(err).ToNot(HaveOccurred())
			Expect(results[0]).To(BeNumerically("==", 3))
		})

		It("should handle nil values", func() {
			nilEval := NewDefaultJqEvaluator(nil)
			results, err := nilEval.Evaluate(ctx, ".")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(BeNil())
		})
	})

	Describe("Default values", func() {
		var eval *JqEvaluator

		BeforeEach(func() {
			testData := map[string]any{
				"name": "test-pod",
				"labels": map[string]any{
					"app":     "myapp",
					"version": "v1.0",
				},
				"annotations": map[string]any{
					"config": "production",
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "main",
							"image": "nginx:latest",
						},
					},
				},
			}
			eval = NewDefaultJqEvaluator(testData)
		})

		Context("with // alternative operator", func() {
			It("should return actual value when path exists", func() {
				results, err := eval.Evaluate(ctx, `.labels.app // "default-app"`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("myapp"))
			})

			It("should return default value when path does not exist", func() {
				results, err := eval.Evaluate(ctx, `.labels.nonexistent // "default-value"`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("default-value"))
			})

			It("should return default value when intermediate path is null", func() {
				results, err := eval.Evaluate(ctx, `.missing.nested.path // "fallback"`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("fallback"))
			})

			It("should handle numeric default values", func() {
				results, err := eval.Evaluate(ctx, `.spec.replicas // 1`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeNumerically("==", 1))
			})

			It("should handle boolean default values", func() {
				results, err := eval.Evaluate(ctx, `.spec.enabled // true`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeTrue())
			})

			It("should handle object default values", func() {
				results, err := eval.Evaluate(ctx, `.status // {"phase": "Unknown"}`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal(map[string]any{"phase": "Unknown"}))
			})

			It("should handle array default values", func() {
				results, err := eval.Evaluate(ctx, `.spec.volumes // []`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal([]any{}))
			})
		})

		Context("with has() function", func() {
			It("should return true for existing keys", func() {
				results, err := eval.Evaluate(ctx, `.labels | has("app")`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeTrue())
			})

			It("should return false for non-existing keys", func() {
				results, err := eval.Evaluate(ctx, `.labels | has("nonexistent")`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeFalse())
			})

			It("should work with conditional defaults", func() {
				results, err := eval.Evaluate(ctx, `if (.labels | has("environment")) then .labels.environment else "development" end`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("development"))
			})
		})

		Context("with complex default patterns", func() {
			It("should chain multiple default operators", func() {
				results, err := eval.Evaluate(ctx, `.labels.environment // .annotations.environment // "staging"`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("staging"))
			})

			It("should use defaults in array operations", func() {
				results, err := eval.Evaluate(ctx, `(.spec.containers // []) | length`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeNumerically("==", 1))
			})

			It("should use defaults with map operations", func() {
				results, err := eval.Evaluate(ctx, `(.spec.containers // []) | map(.name // "unnamed")`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal([]any{"main"}))
			})

			It("should handle defaults in selections", func() {
				results, err := eval.Evaluate(ctx, `(.spec.containers // []) | map(select(.name // "default" | startswith("m"))) | length`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeNumerically("==", 1))
			})
		})
	})
})
