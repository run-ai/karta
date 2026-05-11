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

		// jq:"validate" — read-only expressions (no mutation operators)
		if jqTag, ok := fieldType.Tag.Lookup("jq"); ok {
			if jqTag == "validate" {
				handleTaggedStructField(field, currentPath, errs)
			} else {
				validateJQExpressionsRecursive(field, currentPath, errs)
			}
		} else {
			// Recursively validate nested structures
			validateJQExpressionsRecursive(field, currentPath, errs)
		}
	}
}

// handleTaggedStructField validates a jq:"validate"-tagged field (read-only, no mutation operators).
func handleTaggedStructField(field reflect.Value, currentPath string, errs *[]error) {
	switch {
	case isStringField(field):
		if err := validateStringField(field, currentPath); err != nil {
			*errs = append(*errs, err)
		}
	case isStringSliceField(field):
		for j := 0; j < field.Len(); j++ {
			elem := field.Index(j)
			elemPath := fmt.Sprintf("%s[%d]", currentPath, j)
			if err := validateStringField(elem, elemPath); err != nil {
				*errs = append(*errs, err)
			}
		}
	case isStringMapField(field):
		for _, key := range field.MapKeys() {
			value := field.MapIndex(key)
			valuePath := fmt.Sprintf("%s[%v]", currentPath, key.Interface())
			if err := validateStringField(value, valuePath); err != nil {
				*errs = append(*errs, err)
			}
		}
	default:
		*errs = append(*errs, fmt.Errorf("%s: jq:%q tag can only be used on string, *string, []string, and map[K]string fields", currentPath, "validate"))
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

func validateStringField(field reflect.Value, fieldPath string) error {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil
		}
		field = field.Elem()
	}

	if field.Kind() != reflect.String {
		return fmt.Errorf("%s: jq:'validate' tag can only be used on string or *string fields", fieldPath)
	}

	jqExpression := field.String()
	if jqExpression == "" {
		return nil
	}

	parsed, err := gojq.Parse(jqExpression)
	if err != nil {
		return fmt.Errorf("failed to parse JQ expression '%s' at '%s': %w", jqExpression, fieldPath, err)
	}

	if err := ValidateParsedJQ(parsed); err != nil {
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

// ValidateParsedJQ checks that a parsed JQ query is safe for read-only use.
// It rejects assignment operators, del, dangerous recursive/unbounded built-ins,
// the recursive descent operator, and user-defined functions whose bodies
// contain any of the above.
func ValidateParsedJQ(q *gojq.Query) error {
	if q == nil {
		return nil
	}

	// User-defined functions: def f: body; — walk every body so the blacklist
	// cannot be hidden inside a named helper (e.g. "def evil: del(.x); evil").
	for _, fd := range q.FuncDefs {
		if err := ValidateParsedJQ(fd.Body); err != nil {
			return err
		}
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
				if err := ValidateParsedJQ(arg); err != nil {
					return err
				}
			}
		}

		// Recursive descent operator (..)
		if q.Term.Type == gojq.TermTypeRecurse {
			return errors.New("recursive descent operator '..' is not allowed")
		}

		// Parenthesized expression: (expr)
		if err := ValidateParsedJQ(q.Term.Query); err != nil {
			return err
		}

		// Array constructor: [expr]
		if q.Term.Array != nil {
			if err := ValidateParsedJQ(q.Term.Array.Query); err != nil {
				return err
			}
		}

		// String interpolation: "\(expr)"
		if q.Term.Str != nil {
			for _, strQuery := range q.Term.Str.Queries {
				if err := ValidateParsedJQ(strQuery); err != nil {
					return err
				}
			}
		}

		// Object constructor: {key: val, (keyExpr): val}
		if q.Term.Object != nil {
			for _, kv := range q.Term.Object.KeyVals {
				if err := ValidateParsedJQ(kv.KeyQuery); err != nil {
					return err
				}
				if err := ValidateParsedJQ(kv.Val); err != nil {
					return err
				}
			}
		}

		// if-then-elif*-else-end
		if q.Term.If != nil {
			if err := ValidateParsedJQ(q.Term.If.Cond); err != nil {
				return err
			}
			if err := ValidateParsedJQ(q.Term.If.Then); err != nil {
				return err
			}
			for _, elif := range q.Term.If.Elif {
				if err := ValidateParsedJQ(elif.Cond); err != nil {
					return err
				}
				if err := ValidateParsedJQ(elif.Then); err != nil {
					return err
				}
			}
			if err := ValidateParsedJQ(q.Term.If.Else); err != nil {
				return err
			}
		}

		// try-catch
		if q.Term.Try != nil {
			if err := ValidateParsedJQ(q.Term.Try.Body); err != nil {
				return err
			}
			if err := ValidateParsedJQ(q.Term.Try.Catch); err != nil {
				return err
			}
		}

		// reduce EXPR as $pat (init; update)
		if q.Term.Reduce != nil {
			if err := ValidateParsedJQ(q.Term.Reduce.Query); err != nil {
				return err
			}
			if err := ValidateParsedJQ(q.Term.Reduce.Start); err != nil {
				return err
			}
			if err := ValidateParsedJQ(q.Term.Reduce.Update); err != nil {
				return err
			}
		}

		// foreach EXPR as $pat (init; update [; extract])
		if q.Term.Foreach != nil {
			if err := ValidateParsedJQ(q.Term.Foreach.Query); err != nil {
				return err
			}
			if err := ValidateParsedJQ(q.Term.Foreach.Start); err != nil {
				return err
			}
			if err := ValidateParsedJQ(q.Term.Foreach.Update); err != nil {
				return err
			}
			if err := ValidateParsedJQ(q.Term.Foreach.Extract); err != nil {
				return err
			}
		}

		// Suffix list: index/slice bounds (.x[start:end], .x[idx])
		for _, suffix := range q.Term.SuffixList {
			if suffix.Index != nil {
				if err := ValidateParsedJQ(suffix.Index.Start); err != nil {
					return err
				}
				if err := ValidateParsedJQ(suffix.Index.End); err != nil {
					return err
				}
			}
		}

		// Direct index term: .[expr] or .[start:end]
		if q.Term.Index != nil {
			if err := ValidateParsedJQ(q.Term.Index.Start); err != nil {
				return err
			}
			if err := ValidateParsedJQ(q.Term.Index.End); err != nil {
				return err
			}
		}
	}

	// Binary operator: left op right — reject all assignment operators
	if q.Op > 0 {
		switch q.Op {
		case gojq.OpAssign, gojq.OpModify, gojq.OpUpdateAdd, gojq.OpUpdateSub, gojq.OpUpdateMul, gojq.OpUpdateDiv, gojq.OpUpdateMod, gojq.OpUpdateAlt:
			return fmt.Errorf("modifying operator '%s' is not allowed", q.Op)
		}

		if err := ValidateParsedJQ(q.Left); err != nil {
			return err
		}
		if err := ValidateParsedJQ(q.Right); err != nil {
			return err
		}
	}

	return nil
}



