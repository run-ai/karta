package query

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/itchyny/gojq"
)

const (
	defaultMaxResults            = 1000
	defaultTimeoutInMilliseconds = 2000
)

// JqEvaluator handles JQ evaluation against a source object with query compilation caching
type JqEvaluator struct {
	source any

	maxResults   int
	queryTimeout time.Duration

	// cache for compiled JQ queries
	compilationCache map[string]*gojq.Code

	// Lazy JSON conversion
	jsonOnce sync.Once
	jsonData any
	jsonErr  error
}

func NewDefaultJqEvaluator(source any) *JqEvaluator {
	return &JqEvaluator{
		source:           source,
		compilationCache: make(map[string]*gojq.Code),
		maxResults:       defaultMaxResults,
		queryTimeout:     defaultTimeoutInMilliseconds * time.Millisecond,
	}
}

func NewJqEvaluator(source any, queryMaxResults *int, queryTimeoutInMilliseconds *int) *JqEvaluator {
	jq := NewDefaultJqEvaluator(source)

	if queryMaxResults != nil {
		jq.maxResults = *queryMaxResults
	}
	if queryTimeoutInMilliseconds != nil {
		jq.queryTimeout = time.Duration(*queryTimeoutInMilliseconds) * time.Millisecond
	}

	return jq
}

// Evaluate executes a JQ expression with compilation caching
func (jq *JqEvaluator) Evaluate(ctx context.Context, expression string) ([]any, error) {
	// Get JSON data (lazy conversion)
	jsonData, err := jq.getJsonData()
	if err != nil {
		return nil, fmt.Errorf("failed to get JSON data: %w", err)
	}

	// Compile the expression to a runnable query
	query, err := jq.compile(expression)
	if err != nil {
		return nil, err
	}

	// Execute query
	return jq.safeRun(ctx, query, jsonData, expression)
}

func (jq *JqEvaluator) safeRun(ctx context.Context, q *gojq.Code, input any, expression string) ([]any, error) {
	innerCtx, cancel := context.WithTimeout(ctx, jq.queryTimeout)
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

		if len(results) >= jq.maxResults {
			return nil, fmt.Errorf("query results exceed the allowed number %d", jq.maxResults)
		}
	}
	return results, nil
}

// getJsonData performs lazy JSON conversion with sync.Once
func (jq *JqEvaluator) getJsonData() (any, error) {
	jq.jsonOnce.Do(func() {
		jsonBytes, err := json.Marshal(jq.source)
		if err != nil {
			jq.jsonErr = fmt.Errorf("failed to marshal source object to JSON: %w", err)
			return
		}

		if err := json.Unmarshal(jsonBytes, &jq.jsonData); err != nil {
			jq.jsonErr = fmt.Errorf("failed to unmarshal JSON data: %w", err)
			return
		}
	})

	if jq.jsonErr != nil {
		return nil, jq.jsonErr
	}

	return jq.jsonData, nil
}

func (jq *JqEvaluator) compile(expression string) (*gojq.Code, error) {
	// The expression could be long, create hash key for cache lookup
	cacheKey := jq.getCacheKey(expression)

	// Check compiled query cache using hash
	compiled, exists := jq.compilationCache[cacheKey]
	if exists {
		return compiled, nil
	}

	parsed, err := gojq.Parse(expression)
	if err != nil {
		return nil, &JQParseError{Expression: expression, Err: err}
	}

	compiled, compileErr := gojq.Compile(parsed)
	if compileErr != nil {
		return nil, &JQCompileError{Expression: expression, Err: compileErr}
	}

	// Cache the compiled query
	jq.compilationCache[cacheKey] = compiled

	return compiled, nil
}

func (jq *JqEvaluator) getCacheKey(expression string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(expression)))
}
