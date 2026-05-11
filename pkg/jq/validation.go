// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package jq

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/itchyny/gojq"
)

// ValidateJQExpressions recursively validates all the fields of the object tagged with
// 'jq:"validate"' (read-only rules) or 'jq:"validateAction"' (mutation rules).
func ValidateJQExpressions(object any) []error {
	var errs []error
	validateJQExpressionsRecursive(reflect.ValueOf(object), "", &errs)

	return errs
}

func validateJQExpressionsRecursive(val reflect.Value, fieldPath string, errs *[]error) {
	// Handle nil pointers
	if !val.IsValid() {
		return
	}

	// Dereference pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		validateJQExpressionsInStruct(val, fieldPath, errs)
	case reflect.Slice, reflect.Array:
		validateJQExpressionsInSlice(val, fieldPath, errs)
	case reflect.Map:
		validateJQExpressionsInMap(val, fieldPath, errs)
	}
}

func validateJQExpressionsInStruct(val reflect.Value, basePath string, errs *[]error) {
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Build field path for error reporting
		currentPath := buildFieldPath(basePath, fieldType.Name)

		// jq:"validate"       — read-only expressions (no mutation operators)
		// jq:"validateAction" — mutation expressions (assignment operators allowed)
		if jqTag, ok := fieldType.Tag.Lookup("jq"); ok {
			switch jqTag {
			case "validate":
				handleTaggedStructField(field, currentPath, false, errs)
			case "validateAction":
				handleTaggedStructField(field, currentPath, true, errs)
			default:
				validateJQExpressionsRecursive(field, currentPath, errs)
			}
		} else {
			// Recursively validate nested structures
			validateJQExpressionsRecursive(field, currentPath, errs)
		}
	}
}

// handleTaggedStructField validates a jq-tagged field.
// allowMutation controls whether mutation operators (=, |=, +=, …) are permitted.
func handleTaggedStructField(field reflect.Value, currentPath string, allowMutation bool, errs *[]error) {
	tagName := "validate"
	if allowMutation {
		tagName = "validateAction"
	}
	switch {
	case isStringField(field):
		if err := validateStringField(field, currentPath, allowMutation); err != nil {
			*errs = append(*errs, err)
		}
	case isStringSliceField(field):
		for j := 0; j < field.Len(); j++ {
			elem := field.Index(j)
			elemPath := fmt.Sprintf("%s[%d]", currentPath, j)
			if err := validateStringField(elem, elemPath, allowMutation); err != nil {
				*errs = append(*errs, err)
			}
		}
	case isStringMapField(field):
		for _, key := range field.MapKeys() {
			value := field.MapIndex(key)
			valuePath := fmt.Sprintf("%s[%v]", currentPath, key.Interface())
			if err := validateStringField(value, valuePath, allowMutation); err != nil {
				*errs = append(*errs, err)
			}
		}
	default:
		*errs = append(*errs, fmt.Errorf("%s: jq:%q tag can only be used on string, *string, []string, and map[K]string fields", currentPath, tagName))
	}
}

func isStringField(field reflect.Value) bool {
	return field.Kind() == reflect.String || (field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.String)
}

func isStringSliceField(field reflect.Value) bool {
	return field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String
}

func isStringMapField(field reflect.Value) bool {
	return field.Kind() == reflect.Map && field.Type().Elem().Kind() == reflect.String
}

func validateJQExpressionsInSlice(val reflect.Value, basePath string, errs *[]error) {
	for i := 0; i < val.Len(); i++ {
		indexPath := fmt.Sprintf("%s[%d]", basePath, i)
		validateJQExpressionsRecursive(val.Index(i), indexPath, errs)
	}
}

func validateJQExpressionsInMap(val reflect.Value, basePath string, errs *[]error) {
	for _, key := range val.MapKeys() {
		keyPath := fmt.Sprintf("%s[%v]", basePath, key.Interface())
		validateJQExpressionsRecursive(val.MapIndex(key), keyPath, errs)
	}
}

func validateStringField(field reflect.Value, fieldPath string, allowMutation bool) error {
	// Handle pointer to string (*string)
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil // Optional field, skip validation
		}
		field = field.Elem()
	}

	// Ensure field is a string
	if field.Kind() != reflect.String {
		return fmt.Errorf("%s: jq:'validate' tag can only be used on string or *string fields", fieldPath)
	}

	jqExpression := field.String()
	if jqExpression == "" {
		return nil // Empty jqExpression, skip validation
	}

	parsed, err := gojq.Parse(jqExpression)
	if err != nil {
		return fmt.Errorf("failed to parse JQ expression '%s' at '%s': %w", jqExpression, fieldPath, err)
	}

	err = validateParsedJQ(parsed, allowMutation)
	if err != nil {
		return fmt.Errorf("JQ expression '%s' at '%s' failed validation: %w", jqExpression, fieldPath, err)
	}

	return nil
}

func buildFieldPath(basePath, fieldName string) string {
	if basePath == "" {
		return fieldName
	}
	return fmt.Sprintf("%s.%s", basePath, fieldName)
}

// ValidateParsedJQ checks if a gojq query is read-only and safe.
// It is a convenience wrapper around validateParsedJQ with allowMutation=false.
func ValidateParsedJQ(q *gojq.Query) error {
	return validateParsedJQ(q, false)
}

// validateParsedJQ recursively checks that a parsed JQ query is safe.
//
// When allowMutation is false the function also rejects assignment operators
// (=, |=, +=, …). When allowMutation is true those operators are permitted
// because they are the purpose of the expression (e.g. suspendActions).
//
// In both modes the following are always rejected:
//   - del function
//   - range, paths, recurse, walk, repeat (excessive-output built-ins)
//   - recursive descent operator (..)
//
// The walk covers every AST node that can contain a nested *Query:
// binary Op (Left/Right), Func args, Term.Query, Array.Query, Object key/value
// queries, If/Elif/Else branches, Try body and catch, Reduce and Foreach
// sub-expressions, SuffixList index bounds, Term.Index bounds, and
// Term.Str interpolated queries.
func validateParsedJQ(q *gojq.Query, allowMutation bool) error {
	if q == nil {
		return nil
	}

	if q.Term != nil {
		// Function call: check blacklist then recurse into args.
		if q.Term.Func != nil {
			f := q.Term.Func

			switch f.Name {
			case "del":
				return errors.New("del function is not allowed")
			case "range", "paths", "recurse", "walk", "repeat":
				return fmt.Errorf("function '%s' may produce excessive output and is not allowed", f.Name)
			}

			for _, arg := range f.Args {
				if err := validateParsedJQ(arg, allowMutation); err != nil {
					return err
				}
			}
		}

		// Recursive descent operator (..)
		if q.Term.Type == gojq.TermTypeRecurse {
			return errors.New("recursive descent operator '..' is not allowed")
		}

		// Parenthesized expression: (expr)
		if err := validateParsedJQ(q.Term.Query, allowMutation); err != nil {
			return err
		}

		// Array constructor: [expr]
		if q.Term.Array != nil {
			if err := validateParsedJQ(q.Term.Array.Query, allowMutation); err != nil {
				return err
			}
		}

		// String interpolation: "\(expr)" — Str.Queries holds one *Query per
		// interpolated segment; plain string literals have Queries == nil.
		if q.Term.Str != nil {
			for _, strQuery := range q.Term.Str.Queries {
				if err := validateParsedJQ(strQuery, allowMutation); err != nil {
					return err
				}
			}
		}

		// Object constructor: {key: val, (keyExpr): val}
		if q.Term.Object != nil {
			for _, kv := range q.Term.Object.KeyVals {
				if err := validateParsedJQ(kv.KeyQuery, allowMutation); err != nil {
					return err
				}
				if err := validateParsedJQ(kv.Val, allowMutation); err != nil {
					return err
				}
			}
		}

		// if-then-elif*-else-end
		if q.Term.If != nil {
			if err := validateParsedJQ(q.Term.If.Cond, allowMutation); err != nil {
				return err
			}
			if err := validateParsedJQ(q.Term.If.Then, allowMutation); err != nil {
				return err
			}
			for _, elif := range q.Term.If.Elif {
				if err := validateParsedJQ(elif.Cond, allowMutation); err != nil {
					return err
				}
				if err := validateParsedJQ(elif.Then, allowMutation); err != nil {
					return err
				}
			}
			if err := validateParsedJQ(q.Term.If.Else, allowMutation); err != nil {
				return err
			}
		}

		// try-catch
		if q.Term.Try != nil {
			if err := validateParsedJQ(q.Term.Try.Body, allowMutation); err != nil {
				return err
			}
			if err := validateParsedJQ(q.Term.Try.Catch, allowMutation); err != nil {
				return err
			}
		}

		// reduce EXPR as $pat (init; update)
		if q.Term.Reduce != nil {
			if err := validateParsedJQ(q.Term.Reduce.Query, allowMutation); err != nil {
				return err
			}
			if err := validateParsedJQ(q.Term.Reduce.Start, allowMutation); err != nil {
				return err
			}
			if err := validateParsedJQ(q.Term.Reduce.Update, allowMutation); err != nil {
				return err
			}
		}

		// foreach EXPR as $pat (init; update [; extract])
		if q.Term.Foreach != nil {
			if err := validateParsedJQ(q.Term.Foreach.Query, allowMutation); err != nil {
				return err
			}
			if err := validateParsedJQ(q.Term.Foreach.Start, allowMutation); err != nil {
				return err
			}
			if err := validateParsedJQ(q.Term.Foreach.Update, allowMutation); err != nil {
				return err
			}
			if err := validateParsedJQ(q.Term.Foreach.Extract, allowMutation); err != nil {
				return err
			}
		}

		// Suffix list: index/slice bounds (.x[start:end], .x[idx])
		for _, suffix := range q.Term.SuffixList {
			if suffix.Index != nil {
				if err := validateParsedJQ(suffix.Index.Start, allowMutation); err != nil {
					return err
				}
				if err := validateParsedJQ(suffix.Index.End, allowMutation); err != nil {
					return err
				}
			}
		}

		// Direct index term: .[expr] or .[start:end] where the index is on Term.Index itself
		// (SuffixList only captures trailing suffixes after a named field, e.g. .foo[expr])
		if q.Term.Index != nil {
			if err := validateParsedJQ(q.Term.Index.Start, allowMutation); err != nil {
				return err
			}
			if err := validateParsedJQ(q.Term.Index.End, allowMutation); err != nil {
				return err
			}
		}
	}

	// Binary operator: left op right
	if q.Op > 0 {
		if !allowMutation {
			switch q.Op {
			case gojq.OpAssign, gojq.OpModify, gojq.OpUpdateAdd, gojq.OpUpdateSub, gojq.OpUpdateMul, gojq.OpUpdateDiv, gojq.OpUpdateMod, gojq.OpUpdateAlt:
				return fmt.Errorf("modifying operator '%s' is not allowed", q.Op)
			}
		}

		if err := validateParsedJQ(q.Left, allowMutation); err != nil {
			return err
		}
		if err := validateParsedJQ(q.Right, allowMutation); err != nil {
			return err
		}
	}

	return nil
}

// ValidateActionExpression validates a mutation-safe JQ expression intended for manifest
// patching (e.g. suspendActions/resumeActions in a SuspendDefinition). Unlike ValidateParsedJQ,
// assignment operators (=, |=, +=, etc.) are permitted because they are the purpose of the
// expression. Dangerous recursive/unbounded operations are still rejected.
func ValidateActionExpression(expr string) error {
	if expr == "" {
		return nil
	}

	parsed, err := gojq.Parse(expr)
	if err != nil {
		return fmt.Errorf("failed to parse JQ expression '%s': %w", expr, err)
	}

	return validateParsedJQ(parsed, true)
}
