package query

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/itchyny/gojq"
)

// JqEvaluator handles JQ evaluation against a source object with query compilation caching
type JqEvaluator struct {
	source interface{}

	// cache for compiled JQ queries
	compilationCache map[string]*gojq.Code

	// Lazy JSON conversion
	jsonOnce sync.Once
	jsonData interface{}
	jsonErr  error
}

func NewJqEvaluator(source interface{}) *JqEvaluator {
	return &JqEvaluator{
		source:           source,
		compilationCache: make(map[string]*gojq.Code),
	}
}

// Evaluate executes a JQ expression with compilation caching
func (jq *JqEvaluator) Evaluate(expression string) ([]interface{}, error) {
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
	var results []interface{}
	iter := query.Run(jsonData)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, &JQExecutionError{Expression: expression, Err: err}
		}
		results = append(results, v)
	}

	return results, nil
}

// getJsonData performs lazy JSON conversion with sync.Once
func (jq *JqEvaluator) getJsonData() (interface{}, error) {
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
