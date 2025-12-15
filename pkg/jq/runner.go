package jq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/itchyny/gojq"
)

//go:generate mockgen -source=runner.go -destination=runner_mock.go -package=jq Runner

type Extractor interface {
	Extract(ctx context.Context, expression string) ([]any, error)
}

type Assigner interface {
	// Assign assigns a value to a given expression. e.g .name = "updated"
	Assign(ctx context.Context, expression string, value any) error
	// AssignZip assigns an array of values to a given array expression using zip operation. e.g .items[] = ["a", "b", "c"]
	// The length of the values array must match the length of the array expression.
	AssignZip(ctx context.Context, expression string, values []any) error
}

type Runner interface {
	Extractor
	Assigner
	GetObject() (any, error)
}

const (
	defaultMaxResults            = 1000
	defaultTimeoutInMilliseconds = 10000
)

type runner struct {
	source any

	maxResults   int
	queryTimeout time.Duration

	jsonOnce sync.Once
	jsonData any
	jsonErr  error
}

func NewDefaultRunner(source any) Runner {
	return &runner{
		source:       source,
		maxResults:   defaultMaxResults,
		queryTimeout: defaultTimeoutInMilliseconds * time.Millisecond,
	}
}

func NewRunner(source any, queryMaxResults *int, queryTimeoutInMilliseconds *int) Runner {
	r := &runner{
		source:       source,
		maxResults:   defaultMaxResults,
		queryTimeout: defaultTimeoutInMilliseconds * time.Millisecond,
	}

	if queryMaxResults != nil {
		r.maxResults = *queryMaxResults
	}
	if queryTimeoutInMilliseconds != nil {
		r.queryTimeout = time.Duration(*queryTimeoutInMilliseconds) * time.Millisecond
	}

	return r
}

func (r *runner) Extract(ctx context.Context, expression string) ([]any, error) {
	jsonData, err := r.getJsonData()
	if err != nil {
		return nil, fmt.Errorf("failed to get JSON data: %w", err)
	}

	query, err := r.compile(expression, nil)
	if err != nil {
		return nil, err
	}

	return r.safeRunWithVariables(ctx, query, jsonData, expression, nil)
}

func (r *runner) Assign(ctx context.Context, expression string, value any) error {
	updateExpression := fmt.Sprintf(`(%s) = $val`, expression)
	return r.assignWithExpression(ctx, updateExpression, []string{"$val"}, []any{value})
}

func (r *runner) AssignZip(ctx context.Context, expression string, values []any) error {
	// JQ expression to update array items using zip operation while verifying the length of the number of matched keys and  values array
	updateExpression := fmt.Sprintf(`
		[path(%s)] as $paths |
		if ($paths | length) == ($val | length) then
			reduce range($paths | length) as $i (.; setpath($paths[$i]; $val[$i]))
		else
			error("array length mismatch: expected " + ($paths | length | tostring) + " values but got " + ($val | length | tostring))
		end
	`, expression)

	return r.assignWithExpression(ctx, updateExpression, []string{"$val"}, []any{values})
}

func (r *runner) assignWithExpression(ctx context.Context, updateExpression string, variables []string, values []any) error {
	// Validate that variables and values have the same length
	if len(variables) != len(values) {
		return fmt.Errorf("variables and values length mismatch: %d variables but %d values", len(variables), len(values))
	}

	jsonData, err := r.getJsonData()
	if err != nil {
		return fmt.Errorf("failed to get JSON data: %w", err)
	}

	// Convert all values to primitives
	convertedValues := make([]any, len(values))
	for i, val := range values {
		converted, err := convertToPrimitive(val)
		if err != nil {
			return fmt.Errorf("failed to convert value to primitive: %w", err)
		}
		convertedValues[i] = converted
	}

	query, err := r.compile(updateExpression, variables)
	if err != nil {
		return &JQCompileError{Expression: updateExpression, Err: err}
	}

	results, err := r.safeRunWithVariables(ctx, query, jsonData, updateExpression, convertedValues)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("update query returned no results")
	}

	r.jsonData = results[0]
	return nil
}

func (r *runner) GetObject() (any, error) {
	return r.getJsonData()
}

func (r *runner) safeRunWithVariables(ctx context.Context, q *gojq.Code, input any, expression string, variables []any) ([]any, error) {
	innerCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	iter := q.RunWithContext(innerCtx, input, variables...)

	var results []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, &JQExecutionError{Expression: expression, Err: err}
		}
		results = append(results, v)

		if len(results) >= r.maxResults {
			return nil, fmt.Errorf("query results exceed the allowed number %d", r.maxResults)
		}
	}
	return results, nil
}

func (r *runner) getJsonData() (any, error) {
	r.jsonOnce.Do(func() {
		converted, err := convertToPrimitive(r.source)
		if err != nil {
			r.jsonErr = fmt.Errorf("failed to convert source to primitive: %w", err)
			return
		}
		r.jsonData = converted
	})

	if r.jsonErr != nil {
		return nil, r.jsonErr
	}

	return r.jsonData, nil
}

func (r *runner) compile(expression string, variables []string) (*gojq.Code, error) {
	parsed, err := gojq.Parse(expression)
	if err != nil {
		return nil, &JQParseError{Expression: expression, Err: err}
	}

	compiled, compileErr := gojq.Compile(parsed, gojq.WithVariables(variables))
	if compileErr != nil {
		return nil, &JQCompileError{Expression: expression, Err: compileErr}
	}

	return compiled, nil
}

func convertToPrimitive(value any) (any, error) {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal value to JSON: %w", err)
	}

	var result any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}
	return result, nil
}
