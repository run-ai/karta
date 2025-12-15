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

// Runner interface for query evaluation against data
type Runner interface {
	Evaluate(ctx context.Context, expression string) ([]any, error)
}

const (
	defaultMaxResults            = 1000
	defaultTimeoutInMilliseconds = 10000
)

// runner handles JQ evaluation against a source object
type runner struct {
	source any

	maxResults   int
	queryTimeout time.Duration

	// Lazy JSON conversion
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
	r := NewDefaultRunner(source).(*runner)

	if queryMaxResults != nil {
		r.maxResults = *queryMaxResults
	}
	if queryTimeoutInMilliseconds != nil {
		r.queryTimeout = time.Duration(*queryTimeoutInMilliseconds) * time.Millisecond
	}

	return r
}

// Evaluate executes a JQ expression
func (r *runner) Evaluate(ctx context.Context, expression string) ([]any, error) {
	// Get JSON data (lazy conversion)
	jsonData, err := r.getJsonData()
	if err != nil {
		return nil, fmt.Errorf("failed to get JSON data: %w", err)
	}

	// Compile the expression to a runnable query
	query, err := r.compile(expression)
	if err != nil {
		return nil, err
	}

	// Execute query
	return r.safeRun(ctx, query, jsonData, expression)
}

func (r *runner) safeRun(ctx context.Context, q *gojq.Code, input any, expression string) ([]any, error) {
	innerCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	iter := q.RunWithContext(innerCtx, input)

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

// getJsonData performs lazy JSON conversion with sync.Once
func (r *runner) getJsonData() (any, error) {
	r.jsonOnce.Do(func() {
		jsonBytes, err := json.Marshal(r.source)
		if err != nil {
			r.jsonErr = fmt.Errorf("failed to marshal source object to JSON: %w", err)
			return
		}

		if err := json.Unmarshal(jsonBytes, &r.jsonData); err != nil {
			r.jsonErr = fmt.Errorf("failed to unmarshal JSON data: %w", err)
			return
		}
	})

	if r.jsonErr != nil {
		return nil, r.jsonErr
	}

	return r.jsonData, nil
}

func (r *runner) compile(expression string) (*gojq.Code, error) {
	parsed, err := gojq.Parse(expression)
	if err != nil {
		return nil, &JQParseError{Expression: expression, Err: err}
	}

	compiled, compileErr := gojq.Compile(parsed)
	if compileErr != nil {
		return nil, &JQCompileError{Expression: expression, Err: compileErr}
	}

	return compiled, nil
}
