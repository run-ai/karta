package query

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/itchyny/gojq"
)

//go:generate mockgen -source=evaluator.go -destination=evaluator_mock.go -package=query QueryEvaluator

// QueryEvaluator interface for query evaluation against data
type QueryEvaluator interface {
	Evaluate(ctx context.Context, expression string) ([]any, error)
}

const (
	defaultMaxResults            = 1000
	defaultTimeoutInMilliseconds = 10000
)

// JqEvaluator handles JQ evaluation against a source object
type JqEvaluator struct {
	source any

	maxResults   int
	queryTimeout time.Duration

	// Lazy JSON conversion
	jsonOnce sync.Once
	jsonData any
	jsonErr  error
}

func NewDefaultJqEvaluator(source any) *JqEvaluator {
	return &JqEvaluator{
		source:       source,
		maxResults:   defaultMaxResults,
		queryTimeout: defaultTimeoutInMilliseconds * time.Millisecond,
	}
}

func NewJqEvaluator(source any, queryMaxResults *int, queryTimeoutInMilliseconds *int) *JqEvaluator {
	e := NewDefaultJqEvaluator(source)

	if queryMaxResults != nil {
		e.maxResults = *queryMaxResults
	}
	if queryTimeoutInMilliseconds != nil {
		e.queryTimeout = time.Duration(*queryTimeoutInMilliseconds) * time.Millisecond
	}

	return e
}

// Evaluate executes a JQ expression
func (e *JqEvaluator) Evaluate(ctx context.Context, expression string) ([]any, error) {
	// Get JSON data (lazy conversion)
	jsonData, err := e.getJsonData()
	if err != nil {
		return nil, fmt.Errorf("failed to get JSON data: %w", err)
	}

	// Compile the expression to a runnable query
	query, err := e.compile(expression)
	if err != nil {
		return nil, err
	}

	// Execute query
	return e.safeRun(ctx, query, jsonData, expression)
}

func (e *JqEvaluator) safeRun(ctx context.Context, q *gojq.Code, input any, expression string) ([]any, error) {
	innerCtx, cancel := context.WithTimeout(ctx, e.queryTimeout)
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

		if len(results) >= e.maxResults {
			return nil, fmt.Errorf("query results exceed the allowed number %d", e.maxResults)
		}
	}
	return results, nil
}

// getJsonData performs lazy JSON conversion with sync.Once
func (e *JqEvaluator) getJsonData() (any, error) {
	e.jsonOnce.Do(func() {
		jsonBytes, err := json.Marshal(e.source)
		if err != nil {
			e.jsonErr = fmt.Errorf("failed to marshal source object to JSON: %w", err)
			return
		}

		if err := json.Unmarshal(jsonBytes, &e.jsonData); err != nil {
			e.jsonErr = fmt.Errorf("failed to unmarshal JSON data: %w", err)
			return
		}
	})

	if e.jsonErr != nil {
		return nil, e.jsonErr
	}

	return e.jsonData, nil
}

func (e *JqEvaluator) compile(expression string) (*gojq.Code, error) {
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
