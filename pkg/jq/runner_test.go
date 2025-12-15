package jq

import (
	"context"
	"errors"
	"strings"

	testutils "github.com/run-ai/kai-bolt/test/types/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

type M = map[string]any
type A = []any

var _ = Describe("Runner", func() {
	var (
		ctx        context.Context
		runner     Runner
		testObject M
	)

	BeforeEach(func() {
		ctx = context.Background()
		testObject = M{
			"metadata": M{
				"name":      "test-pod",
				"namespace": "default",
				"labels": M{
					"app":       "web",
					"component": "frontend",
				},
			},
			"spec": M{
				"containers": A{
					M{
						"name":  "web",
						"image": "nginx:1.20",
						"ports": A{
							M{"containerPort": 80},
							M{"containerPort": 443},
						},
					},
					M{
						"name":  "sidecar",
						"image": "busybox:latest",
					},
				},
			},
		}
		runner = NewDefaultRunner(testObject)
	})

	Describe("Basic JQ evaluation", func() {
		It("should evaluate simple path expressions", func() {
			results, err := runner.Evaluate(ctx, ".metadata.name")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(Equal("test-pod"))
		})

		It("should evaluate array access", func() {
			results, err := runner.Evaluate(ctx, ".spec.containers[0].name")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(Equal("web"))
		})

		It("should evaluate array iteration", func() {
			results, err := runner.Evaluate(ctx, ".spec.containers[].name")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(2))
			Expect(results).To(ConsistOf("web", "sidecar"))
		})

		It("should handle nested array iteration", func() {
			results, err := runner.Evaluate(ctx, ".spec.containers[0].ports[].containerPort")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(2))
			Expect(results).To(ConsistOf(float64(80), float64(443)))
		})

		It("should handle non-existent paths", func() {
			results, err := runner.Evaluate(ctx, ".metadata.nonexistent")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(BeNil())
		})

		It("should handle empty results", func() {
			results, err := runner.Evaluate(ctx, ".spec.containers[] | select(.name == \"notfound\")")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(BeEmpty())
		})
	})

	Describe("JQ filters and expressions", func() {
		It("should evaluate boolean expressions", func() {
			results, err := runner.Evaluate(ctx, ".metadata.name == \"test-pod\"")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(BeTrue())
		})

		It("should evaluate select filters", func() {
			results, err := runner.Evaluate(ctx, ".spec.containers[] | select(.name == \"web\") | .image")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(Equal("nginx:1.20"))
		})

		It("should evaluate map operations", func() {
			results, err := runner.Evaluate(ctx, ".spec.containers | map(.name)")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(ConsistOf("web", "sidecar"))
		})

		It("should handle complex expressions", func() {
			results, err := runner.Evaluate(ctx, ".spec.containers | length")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(BeNumerically("==", 2))
		})
	})

	Describe("Error handling", func() {
		It("should return JQParseError for invalid syntax", func() {
			_, err := runner.Evaluate(ctx, ".invalid[[[syntax")
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&JQParseError{}))
		})

		It("should handle context cancellation", func() {
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()

			_, err := runner.Evaluate(cancelCtx, ".metadata.name")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("context canceled"))
		})
	})

	Describe("Result count limits", func() {
		var (
			largeObject M
			limitedExec Runner
			maxResults  = 5
		)

		BeforeEach(func() {
			// Create object with many results programmatically
			largeObject = M{
				"items": make(A, 0),
			}

			// Generate 100 items
			items := largeObject["items"].(A)
			for i := 0; i < 100; i++ {
				items = append(items, M{
					"id":    i,
					"name":  "item-" + string(rune('a'+i%26)),
					"value": i * 10,
				})
			}
			largeObject["items"] = items

			limitedExec = NewRunner(largeObject, &maxResults, nil)
		})

		It("should respect max results limit", func() {
			results, err := limitedExec.Evaluate(ctx, ".items[].id")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("query results exceed the allowed number 5"))
			Expect(results).To(BeNil())
		})

		It("should allow results under the limit", func() {
			results, err := limitedExec.Evaluate(ctx, ".items[0:3][].id")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(3))
			Expect(results).To(ConsistOf(float64(0), float64(1), float64(2)))
		})
	})

	Describe("Timeout limits", func() {
		var (
			fastTimeoutExec Runner
			maxResults      = 1000
			timeoutMs       = 1
		)

		BeforeEach(func() {
			// Create object that might cause slow evaluation
			slowObject := M{
				"data": make(A, 0),
			}

			// Generate data for potential slow operations
			data := slowObject["data"].(A)
			for i := 0; i < 10000; i++ {
				data = append(data, M{
					"id":     i,
					"nested": strings.Repeat("x", 100), // Large strings
				})
			}
			slowObject["data"] = data

			fastTimeoutExec = NewRunner(slowObject, &maxResults, &timeoutMs)
		})

		It("should respect timeout limits for complex operations", func() {
			_, err := fastTimeoutExec.Evaluate(ctx, ".data | map(select(.nested | length > 50)) | length")

			Expect(err).To(HaveOccurred())

			var jqExecError *JQExecutionError
			Expect(errors.As(err, &jqExecError)).To(BeTrue())

			Expect(errors.Is(jqExecError.Unwrap(), context.DeadlineExceeded)).To(BeTrue())
		})

		It("should work with longer timeout for the same operation", func() {
			longerTimeoutMs := 10000
			longerTimeoutExec := NewRunner(testObject, &maxResults, &longerTimeoutMs)

			results, err := longerTimeoutExec.Evaluate(ctx, ".spec.containers[].name")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(2))
		})
	})

	Describe("JSON conversion", func() {
		It("should handle different source object types", func() {
			stringExec := NewDefaultRunner("test-string")
			results, err := stringExec.Evaluate(ctx, ". | length")
			Expect(err).ToNot(HaveOccurred())
			Expect(results[0]).To(BeNumerically("==", 11))

			numberExec := NewDefaultRunner(42)
			results, err = numberExec.Evaluate(ctx, ". + 8")
			Expect(err).ToNot(HaveOccurred())
			Expect(results[0]).To(BeNumerically("==", 50))

			arrayExec := NewDefaultRunner(A{1, 2, 3})
			results, err = arrayExec.Evaluate(ctx, ". | length")
			Expect(err).ToNot(HaveOccurred())
			Expect(results[0]).To(BeNumerically("==", 3))
		})

		It("should handle nil values", func() {
			nilExec := NewDefaultRunner(nil)
			results, err := nilExec.Evaluate(ctx, ".")
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0]).To(BeNil())
		})
	})

	Describe("Default values", func() {
		var runner Runner

		BeforeEach(func() {
			testData := M{
				"name": "test-pod",
				"labels": M{
					"app":     "myapp",
					"version": "v1.0",
				},
				"annotations": M{
					"config": "production",
				},
				"spec": M{
					"containers": A{
						M{
							"name":  "main",
							"image": "nginx:latest",
						},
					},
				},
			}
			runner = NewDefaultRunner(testData)
		})

		Context("with // alternative operator", func() {
			It("should return actual value when path exists", func() {
				results, err := runner.Evaluate(ctx, `.labels.app // "default-app"`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("myapp"))
			})

			It("should return default value when path does not exist", func() {
				results, err := runner.Evaluate(ctx, `.labels.nonexistent // "default-value"`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("default-value"))
			})

			It("should return default value when intermediate path is null", func() {
				results, err := runner.Evaluate(ctx, `.missing.nested.path // "fallback"`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("fallback"))
			})

			It("should handle numeric default values", func() {
				results, err := runner.Evaluate(ctx, `.spec.replicas // 1`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeNumerically("==", 1))
			})

			It("should handle boolean default values", func() {
				results, err := runner.Evaluate(ctx, `.spec.enabled // true`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeTrue())
			})

			It("should handle object default values", func() {
				results, err := runner.Evaluate(ctx, `.status // {"phase": "Unknown"}`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal(M{"phase": "Unknown"}))
			})

			It("should handle array default values", func() {
				results, err := runner.Evaluate(ctx, `.spec.volumes // []`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal(A{}))
			})
		})

		Context("with has() function", func() {
			It("should return true for existing keys", func() {
				results, err := runner.Evaluate(ctx, `.labels | has("app")`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeTrue())
			})

			It("should return false for non-existing keys", func() {
				results, err := runner.Evaluate(ctx, `.labels | has("nonexistent")`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeFalse())
			})

			It("should work with conditional defaults", func() {
				results, err := runner.Evaluate(ctx, `if (.labels | has("environment")) then .labels.environment else "development" end`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("development"))
			})
		})

		Context("with complex default patterns", func() {
			It("should chain multiple default operators", func() {
				results, err := runner.Evaluate(ctx, `.labels.environment // .annotations.environment // "staging"`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal("staging"))
			})

			It("should use defaults in array operations", func() {
				results, err := runner.Evaluate(ctx, `(.spec.containers // []) | length`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeNumerically("==", 1))
			})

			It("should use defaults with map operations", func() {
				results, err := runner.Evaluate(ctx, `(.spec.containers // []) | map(.name // "unnamed")`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(Equal(A{"main"}))
			})

			It("should handle defaults in selections", func() {
				results, err := runner.Evaluate(ctx, `(.spec.containers // []) | map(select(.name // "default" | startswith("m"))) | length`)

				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0]).To(BeNumerically("==", 1))
			})
		})
	})

	Describe("Assign operations (direct assignment)", func() {
		It("should update simple path", func() {
			testData := M{
				"name": "original",
			}
			runner := NewDefaultRunner(testData)

			err := runner.Assign(ctx, ".name", "updated")
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(Equal(M{"name": "updated"}))
		})

		It("should update nested path", func() {
			testData := M{
				"metadata": M{
					"name":    "original",
					"example": "example",
				},
			}
			runner := NewDefaultRunner(testData)

			err := runner.Assign(ctx, ".metadata.name", "updated")
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(Equal(M{"metadata": M{"name": "updated", "example": "example"}}))
		})

		It("should update with alternative operator when primary exists", func() {
			testData := M{
				"primary":  "value1",
				"fallback": "value2",
			}
			runner := NewDefaultRunner(testData)

			err := runner.Assign(ctx, ".primary // .fallback", "updated")
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(Equal(M{"primary": "updated", "fallback": "value2"}))
		})

		It("should update with fallback path when primary missing", func() {
			testData := M{
				"fallback": "value",
			}
			runner := NewDefaultRunner(testData)

			err := runner.Assign(ctx, ".primary // .fallback", "updated")
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.(M)["fallback"]).To(Equal("updated"))
		})

		It("should update array element with specific index", func() {
			testData := M{
				"items": A{M{"name": "a"}, M{"name": "b"}, M{"name": "c"}},
			}
			runner := NewDefaultRunner(testData)

			err := runner.Assign(ctx, ".items[1] | .name", "updated")
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			items := updated.(M)["items"].(A)
			Expect(items).To(Equal(A{M{"name": "a"}, M{"name": "updated"}, M{"name": "c"}}))
		})

		It("should update complex object", func() {
			testData := M{
				"spec": M{
					"replicas": 3,
				},
			}
			runner := NewDefaultRunner(testData)

			newSpec := M{
				"replicas": 5,
				"template": M{"name": "pod"},
			}

			err := runner.Assign(ctx, ".spec", newSpec)
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.(M)["spec"]).To(testutils.BeJSONEquivalentTo(newSpec))
		})

		It("should update struct object (not primitive type)", func() {
			testData := M{
				"spec": M{
					"containers": []corev1.Container{{Name: "test-container"}},
				}}
			runner := NewDefaultRunner(testData)

			err := runner.Assign(ctx, ".spec.containers", []corev1.Container{{Name: "updated"}})
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(testutils.BeJSONEquivalentTo(M{"spec": M{"containers": []corev1.Container{{Name: "updated"}}}}))
		})
	})

	Describe("AssignZip operations (zip assignment)", func() {
		It("should update filtered items", func() {
			testData := M{
				"items": A{
					M{"id": 1, "name": "first"},
					M{"id": 2, "name": "second"},
				},
			}
			runner := NewDefaultRunner(testData)

			err := runner.AssignZip(ctx, ".items[] | select(.id == 1) | .name", A{"updated"})
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			items := updated.(M)["items"].(A)
			Expect(items[0].(M)["name"]).To(Equal("updated"))
			Expect(items[1].(M)["name"]).To(Equal("second"))
		})

		It("should update array items", func() {
			testData := M{
				"items": A{"a", "b", "c"},
			}
			runner := NewDefaultRunner(testData)

			err := runner.AssignZip(ctx, ".items[]", A{"d", "e", "f"})
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			items := updated.(M)["items"].(A)
			Expect(items).To(Equal(A{"d", "e", "f"}))
		})

		It("should update array nested field", func() {
			testData := M{
				"items": A{M{"name": "a"}, M{"name": "b"}, M{"name": "c"}},
			}
			runner := NewDefaultRunner(testData)

			err := runner.AssignZip(ctx, ".items[] | .name", A{"d", "e", "f"})
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			items := updated.(M)["items"].(A)
			Expect(items).To(Equal(A{M{"name": "d"}, M{"name": "e"}, M{"name": "f"}}))
		})

		It("should update array nested field with some nil values", func() {
			testData := M{
				"items": A{M{"name": "a"}, M{"name": "b", "value": 1}, M{"name": "c", "value": 2}},
			}
			runner := NewDefaultRunner(testData)

			err := runner.AssignZip(ctx, ".items[] | .value", A{nil, 3, nil})
			Expect(err).ToNot(HaveOccurred())

			updated, err := runner.GetObject()
			Expect(err).ToNot(HaveOccurred())
			items := updated.(M)["items"].(A)
			Expect(items).To(testutils.BeJSONEquivalentTo(A{M{"name": "a", "value": nil}, M{"name": "b", "value": 3}, M{"name": "c", "value": nil}}))
		})

		It("should throw error if array length mismatch", func() {
			testData := M{
				"items": A{"a", "b", "c"},
			}
			runner := NewDefaultRunner(testData)

			err := runner.AssignZip(ctx, ".items[]", A{"d", "e"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("array length mismatch"))
		})
	})
})
