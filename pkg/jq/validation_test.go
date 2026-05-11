// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package jq

import (
	"strings"

	"github.com/itchyny/gojq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type TestStruct struct {
	RequiredPath string            `jq:"validate"`
	OptionalPath *string           `jq:"validate"`
	SlicePaths   []string          `jq:"validate"`
	MapPaths     map[string]string `jq:"validate"`
	RegularField string
	NestedStruct NestedTestStruct
	StructSlice  []NestedTestStruct
}

type NestedTestStruct struct {
	NestedPath string `jq:"validate"`
	NoTagField string
}

type StructWithUntagged struct {
	TaggedPath        string `jq:"validate"`
	UntaggedString    string
	UntaggedPtrString *string
	UntaggedStrings   []string
	UntaggedStringMap map[string]string
}

type ActionTestStruct struct {
	SuspendActions []string `jq:"validateAction"`
	ResumeActions  []string `jq:"validateAction"`
	ReadOnlyPath   string   `jq:"validate"`
}

var _ = Describe("JQ Validation", func() {
	Describe("ValidateJQExpressions", func() {
		Context("valid JQ expressions", func() {
			It("should pass with simple path expressions", func() {
				obj := TestStruct{
					RequiredPath: ".spec.template",
					OptionalPath: stringPtr(".spec.replicas"),
					SlicePaths:   []string{".spec.containers[0]", ".metadata.labels"},
					MapPaths:     map[string]string{"key1": ".spec.replicas", "key2": ".metadata.name"},
					NestedStruct: NestedTestStruct{
						NestedPath: ".status.phase",
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(BeEmpty())
			})

			It("should ignore non-tagged fields even with dangerous JQ expressions", func() {
				obj := TestStruct{
					RequiredPath: ".spec.template",
					RegularField: "del(.dangerous)", // This should be ignored since it's not tagged
					NestedStruct: NestedTestStruct{
						NestedPath: ".status.phase",
						NoTagField: "recurse(.also.dangerous)", // This should also be ignored
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(BeEmpty()) // Should pass because non-tagged fields are ignored
			})

			It("should pass with complex but safe expressions", func() {
				obj := TestStruct{
					RequiredPath: ".spec.template | select(.metadata.labels.app == \"test\")",
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(BeEmpty())
			})

			It("should skip validation for empty expressions", func() {
				obj := TestStruct{
					RequiredPath: "",
					OptionalPath: stringPtr(""),
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(BeEmpty())
			})

			It("should skip validation for nil pointer fields", func() {
				obj := TestStruct{
					RequiredPath: ".spec.template",
					OptionalPath: nil,
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(BeEmpty())
			})
		})

		Context("invalid JQ syntax", func() {
			It("should fail with malformed JQ expression", func() {
				obj := TestStruct{
					RequiredPath: ".spec.template.[",
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("failed to parse JQ expression '.spec.template.[' at 'RequiredPath'"))
				Expect(errs[0].Error()).To(ContainSubstring("RequiredPath"))
			})

			It("should fail with unclosed brackets", func() {
				obj := TestStruct{
					SlicePaths: []string{".spec.containers[0"},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("failed to parse JQ expression '.spec.containers[0' at 'SlicePaths[0]'"))
				Expect(errs[0].Error()).To(ContainSubstring("SlicePaths[0]"))
			})
		})

		DescribeTable("dangerous JQ expressions",
			func(jqExpression, expectedErrorSubstring string) {
				obj := TestStruct{
					RequiredPath: jqExpression,
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring(jqExpression + "' at 'RequiredPath'"))
				Expect(errs[0].Error()).To(ContainSubstring(expectedErrorSubstring))
			},
			// Dangerous functions
			Entry("should reject del function", "del(.spec.template)", "del function is not allowed"),
			Entry("should reject recurse function", "recurse(.children[])", "function 'recurse'"),
			Entry("should reject walk function", "walk(if type == \"object\" then . else empty end)", "function 'walk'"),
			Entry("should reject paths function", "paths", "function 'paths'"),
			Entry("should reject range function", "range(1000000)", "function 'range'"),
			Entry("should reject repeat function", "repeat(.)", "function 'repeat'"),

			// Recursive descent operator
			Entry("should reject recursive descent operator", ".. | .name", "recursive descent operator"),

			// Assignment and update operators
			Entry("should reject assignment operator =", ".spec.replicas = 5", "modifying operator"),
			Entry("should reject update operator +=", ".spec.replicas += 1", "modifying operator"),
			Entry("should reject update operator -=", ".spec.replicas -= 1", "modifying operator"),
			Entry("should reject update operator *=", ".spec.replicas *= 2", "modifying operator"),
			Entry("should reject update operator /=", ".spec.replicas /= 2", "modifying operator"),
			Entry("should reject update operator %=", ".spec.replicas %= 3", "modifying operator"),
			Entry("should reject update operator //=", ".spec.replicas //= 1", "modifying operator"),
			Entry("should reject modify operator |=", ".spec.replicas |= . + 1", "modifying operator"),
		)

		Context("complex expressions with pipes", func() {
			It("should validate expressions in pipe chains", func() {
				obj := TestStruct{
					RequiredPath: ".spec.containers | map(select(.name == \"main\")) | .[0]",
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(BeEmpty())
			})

			It("should reject dangerous functions in pipe chains", func() {
				obj := TestStruct{
					RequiredPath: ".spec | recurse | .name",
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring(".spec | recurse | .name' at 'RequiredPath'"))
				Expect(errs[0].Error()).To(ContainSubstring("function 'recurse'"))
			})

			It("should reject assignment in pipe chains", func() {
				obj := TestStruct{
					RequiredPath: ".spec.containers[0] | .image = \"new-image\"",
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("at 'RequiredPath'"))
				Expect(errs[0].Error()).To(ContainSubstring("modifying operator"))
			})
		})

		Context("nested structure validation", func() {
			It("should validate nested struct fields", func() {
				obj := TestStruct{
					RequiredPath: ".spec.template",
					NestedStruct: NestedTestStruct{
						NestedPath: "del(.status)",
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("del(.status)' at 'NestedStruct.NestedPath'"))
				Expect(errs[0].Error()).To(ContainSubstring("del function is not allowed"))
			})

			It("should validate struct slices", func() {
				obj := TestStruct{
					RequiredPath: ".spec.template",
					StructSlice: []NestedTestStruct{
						{NestedPath: ".valid.path"},
						{NestedPath: "recurse(.invalid)"},
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("recurse(.invalid)' at 'StructSlice[1].NestedPath'"))
				Expect(errs[0].Error()).To(ContainSubstring("function 'recurse'"))
			})
		})

		Context("slice validation", func() {
			It("should validate all elements in string slices", func() {
				obj := TestStruct{
					SlicePaths: []string{".valid.path", "del(.invalid)", ".another.valid"},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("del(.invalid)' at 'SlicePaths[1]'"))
				Expect(errs[0].Error()).To(ContainSubstring("del function is not allowed"))
			})
		})

		Context("map validation", func() {
			It("should validate all values in string maps", func() {
				obj := TestStruct{
					MapPaths: map[string]string{
						"valid":   ".spec.replicas",
						"invalid": "del(.spec.template)",
						"another": ".metadata.name",
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("del(.spec.template)' at 'MapPaths[invalid]'"))
				Expect(errs[0].Error()).To(ContainSubstring("del function is not allowed"))
			})

			It("should pass with valid map values", func() {
				obj := TestStruct{
					MapPaths: map[string]string{
						"replicas": ".spec.replicas",
						"name":     ".metadata.name",
						"phase":    ".status.phase",
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(BeEmpty())
			})

			It("should handle empty maps", func() {
				obj := TestStruct{
					MapPaths: map[string]string{},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(BeEmpty())
			})

			It("should validate multiple invalid map values", func() {
				obj := TestStruct{
					MapPaths: map[string]string{
						"first":  "del(.spec)",
						"second": "recurse(.status)",
						"third":  ".valid.path",
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(2))

				for _, err := range errs {
					errStr := err.Error()
					if strings.Contains(errStr, "MapPaths[first]") {
						Expect(errStr).To(ContainSubstring("del(.spec)' at 'MapPaths[first]'"))
						Expect(errStr).To(ContainSubstring("del function is not allowed"))
					} else if strings.Contains(errStr, "MapPaths[second]") {
						Expect(errStr).To(ContainSubstring("recurse(.status)' at 'MapPaths[second]'"))
						Expect(errStr).To(ContainSubstring("function 'recurse'"))
					}
				}
			})
		})

		Context("multiple validation errors", func() {
			It("should aggregate all validation errors", func() {
				obj := TestStruct{
					RequiredPath: "del(.spec)",
					OptionalPath: stringPtr("recurse(.status)"),
					SlicePaths:   []string{".valid", "paths", "walk(.invalid)"},
					MapPaths:     map[string]string{"key": "range(1000)"},
					NestedStruct: NestedTestStruct{
						NestedPath: "repeat(.)",
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(6)) // Updated to 6 to include the map error

				// Check that we have all expected errors
				Expect(errs).To(ConsistOf(
					MatchError(ContainSubstring("del function is not allowed")),
					MatchError(ContainSubstring("function 'recurse' may produce excessive output")),
					MatchError(ContainSubstring("function 'paths' may produce excessive output")),
					MatchError(ContainSubstring("function 'walk' may produce excessive output")),
					MatchError(ContainSubstring("function 'range' may produce excessive output")),
					MatchError(ContainSubstring("function 'repeat' may produce excessive output")),
				))
			})
		})

		Context("field path reporting", func() {
			It("should include correct field paths in error messages", func() {
				obj := TestStruct{
					RequiredPath: "del(.root)",
					MapPaths:     map[string]string{"mapkey": "walk(.map)"},
					NestedStruct: NestedTestStruct{
						NestedPath: "recurse(.nested)",
					},
					StructSlice: []NestedTestStruct{
						{NestedPath: "paths"},
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(4))

				// Check that we have all expected errors
				Expect(errs).To(ConsistOf(
					MatchError(ContainSubstring("del(.root)' at 'RequiredPath'")),
					MatchError(ContainSubstring("walk(.map)' at 'MapPaths[mapkey]'")),
					MatchError(ContainSubstring("recurse(.nested)' at 'NestedStruct.NestedPath'")),
					MatchError(ContainSubstring("paths' at 'StructSlice[0].NestedPath'")),
				))
			})

			It("should report correct paths for optional pointer fields", func() {
				obj := TestStruct{
					OptionalPath: stringPtr("del(.optional)"),
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("del(.optional)' at 'OptionalPath'"))
				Expect(errs[0].Error()).To(ContainSubstring("del function is not allowed"))
			})

			It("should report correct paths for slice elements", func() {
				obj := TestStruct{
					SlicePaths: []string{
						".valid.path",
						"del(.first.invalid)",
						".another.valid",
						"recurse(.second.invalid)",
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(2))

				// Check that we have both expected errors
				Expect(errs).To(ConsistOf(
					MatchError(ContainSubstring("del(.first.invalid)' at 'SlicePaths[1]'")),
					MatchError(ContainSubstring("recurse(.second.invalid)' at 'SlicePaths[3]'")),
				))
			})

			It("should report correct paths for deeply nested structures", func() {
				obj := TestStruct{
					StructSlice: []NestedTestStruct{
						{NestedPath: ".valid"},
						{NestedPath: "del(.invalid.deep)"},
						{NestedPath: ".also.valid"},
					},
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("del(.invalid.deep)' at 'StructSlice[1].NestedPath'"))
				Expect(errs[0].Error()).To(ContainSubstring("del function is not allowed"))
			})
		})

		Context("untagged fields", func() {
			It("should ignore all untagged fields while validating tagged fields", func() {
				obj := StructWithUntagged{
					TaggedPath:        "del(.invalid)",                   // This should be caught
					UntaggedString:    "del(.dangerous1)",                // Should be ignored
					UntaggedPtrString: stringPtr("recurse(.dangerous2)"), // Should be ignored
					UntaggedStrings: []string{
						"del(.dangerous3)",
						"recurse(.dangerous4)",
						"walk(.dangerous5)",
					}, // Should be ignored
					UntaggedStringMap: map[string]string{
						"key1": "paths",
						"key2": "range(1000)",
						"key3": "repeat(.)",
					}, // Should be ignored
				}

				errs := ValidateJQExpressions(obj)
				Expect(errs).To(HaveLen(1))
				Expect(errs[0].Error()).To(ContainSubstring("del(.invalid)' at 'TaggedPath'"))
				Expect(errs[0].Error()).To(ContainSubstring("del function is not allowed"))
			})
		})
	})
})

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

var _ = Describe("ValidateParsedJQ", func() {
	parseOrFail := func(expr string) *gojq.Query {
		q, err := gojq.Parse(expr)
		Expect(err).NotTo(HaveOccurred())
		return q
	}

	Describe("allowMutation=false (read-only mode)", func() {
		It("should accept a plain path expression", func() {
			Expect(ValidateParsedJQ(parseOrFail(".spec.replicas"), false)).To(Succeed())
		})

		It("should reject mutation operators", func() {
			err := ValidateParsedJQ(parseOrFail(".spec.suspend = true"), false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("modifying operator"))
		})

		It("should reject del", func() {
			err := ValidateParsedJQ(parseOrFail("del(.spec)"), false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("del function is not allowed"))
		})

		It("should reject recursive descent", func() {
			err := ValidateParsedJQ(parseOrFail(".. | .name"), false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("'..' is not allowed"))
		})

		It("should return nil for a nil query", func() {
			Expect(ValidateParsedJQ(nil, false)).To(Succeed())
		})
	})

	Describe("allowMutation=true (action mode)", func() {
		It("should accept assignment operators", func() {
			Expect(ValidateParsedJQ(parseOrFail(".spec.suspend = true"), true)).To(Succeed())
		})

		It("should accept update-modify operators", func() {
			Expect(ValidateParsedJQ(parseOrFail(".spec.replicas |= . + 1"), true)).To(Succeed())
		})

		It("should still reject del", func() {
			err := ValidateParsedJQ(parseOrFail("del(.spec)"), true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("del function is not allowed"))
		})

		It("should still reject range", func() {
			err := ValidateParsedJQ(parseOrFail("range(5)"), true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("function 'range'"))
		})

		It("should still reject recursive descent", func() {
			err := ValidateParsedJQ(parseOrFail(".. | .name"), true)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("'..' is not allowed"))
		})
	})

	Describe("user-defined functions (FuncDefs) cannot hide blacklisted operations", func() {
		DescribeTable("should reject expressions where a def body contains a blacklisted operation",
			func(expr, expectedSubstring string) {
				err := ValidateParsedJQ(parseOrFail(expr), true)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			Entry("del hidden in a named function", "def evil: del(.x); evil", "del function is not allowed"),
			Entry("recurse hidden in a named function", "def evil: recurse; evil", "function 'recurse'"),
			Entry("recursive descent hidden in a named function", "def evil: ..; evil | .name", "'..' is not allowed"),
			Entry("range hidden in a named function", "def evil: range(1000000); evil", "function 'range'"),
			Entry("del in nested inner function", "def outer: def inner: del(.x); inner; outer", "del function is not allowed"),
			Entry("del passed as argument via higher-order function", "def apply(f): f; apply(del(.x))", "del function is not allowed"),
		)

		It("should accept a safe user-defined function", func() {
			Expect(ValidateParsedJQ(parseOrFail("def double: . * 2; .spec.replicas | double"), true)).To(Succeed())
		})
	})
})

var _ = Describe("ValidateActionExpression", func() {
	Context("valid mutation expressions", func() {
		DescribeTable("should accept assignment and update operators",
			func(expr string) {
				err := ValidateActionExpression(expr)
				Expect(err).NotTo(HaveOccurred())
			},
			Entry("simple assignment", ".spec.suspend = true"),
			Entry("update-add operator", ".spec.replicas += 1"),
			Entry("update-sub operator", ".spec.replicas -= 1"),
			Entry("update-mul operator", ".spec.replicas *= 2"),
			Entry("update-div operator", ".spec.replicas /= 2"),
			Entry("update-mod operator", ".spec.replicas %= 3"),
			Entry("update-alt operator", ".spec.field //= null"),
			Entry("modify operator", ".spec.suspend |= not"),
			Entry("piped assignment", ".spec | .suspend = true"),
			Entry("nested field assignment", ".spec.runPolicy.suspend = true"),
		)

		It("should accept empty expression", func() {
			err := ValidateActionExpression("")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should accept a simple read-only path expression", func() {
			err := ValidateActionExpression(".spec.suspend")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("invalid expressions — dangerous built-ins rejected at top level", func() {
		DescribeTable("should reject dangerous recursive/unbounded operations",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			Entry("del function", "del(.spec.suspend)", "del function is not allowed"),
			Entry("recurse function", "recurse(.children[])", "function 'recurse'"),
			Entry("walk function", "walk(if type == \"object\" then . else empty end)", "function 'walk'"),
			Entry("paths function", "paths", "function 'paths'"),
			Entry("range function", "range(1000000)", "function 'range'"),
			Entry("repeat function", "repeat(.)", "function 'repeat'"),
			Entry("recursive descent operator", ".. | .name", "recursive descent operator"),
		)

		It("should reject malformed JQ syntax", func() {
			err := ValidateActionExpression(".spec.suspend = [")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse JQ expression"))
		})
	})

	Context("AST traversal — nested constructs cannot bypass the blacklist", func() {
		DescribeTable("Array constructor",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			Entry("del nested in array", "[del(.x)]", "del function is not allowed"),
			Entry("recurse nested in array", "[recurse]", "function 'recurse'"),
		)

		DescribeTable("Object constructor",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			Entry("del in object value", "{a: del(.x)}", "del function is not allowed"),
			Entry("del in computed object key", "{(del(.x)): .}", "del function is not allowed"),
			Entry("recurse in object value", "{a: recurse}", "function 'recurse'"),
		)

		DescribeTable("if-then-elif-else",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			Entry("del in if condition", "if del(.x) then . end", "del function is not allowed"),
			Entry("del in then branch", "if . then del(.x) end", "del function is not allowed"),
			Entry("del in else branch", "if . then . else del(.x) end", "del function is not allowed"),
			Entry("del in elif condition", "if . then . elif del(.x) then . end", "del function is not allowed"),
			Entry("del in elif then branch", "if . then . elif . then del(.x) end", "del function is not allowed"),
		)

		DescribeTable("try-catch",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			Entry("del in try body", "try del(.x)", "del function is not allowed"),
			Entry("del in catch handler", "try . catch del(.x)", "del function is not allowed"),
		)

		DescribeTable("reduce",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			Entry("del as iterable expression", "reduce del(.x) as $x (0; .)", "del function is not allowed"),
			Entry("del as initial accumulator", "reduce .[] as $x (del(.x); .)", "del function is not allowed"),
			Entry("del in update step", "reduce .[] as $x (0; del(.x))", "del function is not allowed"),
		)

		DescribeTable("foreach",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			Entry("del as iterable expression", "foreach del(.x) as $x (0; .)", "del function is not allowed"),
			Entry("del as initial state", "foreach .[] as $x (del(.x); .)", "del function is not allowed"),
			Entry("del in update step", "foreach .[] as $x (0; del(.x))", "del function is not allowed"),
			Entry("del in extract step", "foreach .[] as $x (0; .; del(.x))", "del function is not allowed"),
		)

		DescribeTable("parenthesized expression (Term.Query)",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			Entry("del in parens", "(del(.x))", "del function is not allowed"),
			Entry("recurse in parens", "(recurse)", "function 'recurse'"),
		)

		DescribeTable("SuffixList index bounds",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
			// .[expr] / .[start:end] — dangerous function on Term.Index directly
			Entry("banned function in direct index", ".[range(5)]", "function 'range'"),
			Entry("banned function in direct slice start", ".[range(5):0]", "function 'range'"),
			Entry("banned function in direct slice end", ".[0:range(5)]", "function 'range'"),
			// .foo[expr] — dangerous function in SuffixList Index
			Entry("banned function in suffix index", ".spec[range(5)]", "function 'range'"),
			Entry("banned function in suffix slice start", ".spec[range(5):0]", "function 'range'"),
			Entry("banned function in suffix slice end", ".spec[0:range(5)]", "function 'range'"),
		)

		DescribeTable("string interpolation (Term.Str.Queries)",
			func(expr, expectedSubstring string) {
				err := ValidateActionExpression(expr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedSubstring))
			},
		Entry("del nested in interpolated string", `"value: \(del(.x))"`, "del function is not allowed"),
		Entry("range nested in interpolated string", `"count: \(range(5))"`, "function 'range'"),
	)
})

var _ = Describe("jq:validateAction tag", func() {
	Describe("ValidateJQExpressions with validateAction tag", func() {
		It("should accept mutation operators in validateAction-tagged fields", func() {
			obj := ActionTestStruct{
				SuspendActions: []string{".spec.suspend = true", ".spec.replicas |= . * 0"},
				ResumeActions:  []string{".spec.suspend = false"},
				ReadOnlyPath:   ".metadata.name",
			}
			errs := ValidateJQExpressions(obj)
			Expect(errs).To(BeEmpty())
		})

		It("should still reject dangerous functions in validateAction-tagged fields", func() {
			obj := ActionTestStruct{
				SuspendActions: []string{"del(.spec)"},
			}
			errs := ValidateJQExpressions(obj)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Error()).To(ContainSubstring("del function is not allowed"))
		})

		It("should still reject recursive descent in validateAction-tagged fields", func() {
			obj := ActionTestStruct{
				ResumeActions: []string{".. | .spec?"},
			}
			errs := ValidateJQExpressions(obj)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Error()).To(ContainSubstring("'..' is not allowed"))
		})

		It("should still reject mutation operators in validate-tagged fields", func() {
			obj := ActionTestStruct{
				SuspendActions: []string{".spec.suspend"},
				ResumeActions:  []string{".spec.suspend"},
				ReadOnlyPath:   ".spec.replicas = 0",
			}
			errs := ValidateJQExpressions(obj)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Error()).To(ContainSubstring("modifying operator"))
		})

		It("should report errors for all invalid entries in a slice", func() {
			obj := ActionTestStruct{
				SuspendActions: []string{".spec.suspend = true", "del(.x)", "range(5) | ."},
			}
			errs := ValidateJQExpressions(obj)
			Expect(errs).To(HaveLen(2))
		})
	})
})
})
