package resource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/query"
	corev1 "k8s.io/api/core/v1"
)

// PodQuerier handles JQ-based querying operations against pods
type PodQuerier struct {
	pod            *corev1.Pod
	queryEvaluator query.QueryEvaluator
}

func NewPodQuerier(pod *corev1.Pod) *PodQuerier {
	return &PodQuerier{
		pod:            pod,
		queryEvaluator: query.NewDefaultJqEvaluator(pod),
	}
}

func (pq *PodQuerier) GetPodName() string {
	return pq.pod.Name
}

// Matches returns true if the pod matches the given selector
func (pq *PodQuerier) Matches(ctx context.Context, selector *v1alpha1.PodSelector) (bool, error) {
	if selector == nil {
		return false, nil
	}

	if selector.Value == nil {
		// Existence check: key should exist and not be nil
		return pq.checkKeyExists(ctx, selector.KeyPath)
	} else {
		// Equality check: key should equal the specified value
		return pq.checkKeyValue(ctx, selector.KeyPath, *selector.Value)
	}
}

// checkKeyExists returns true if the key exists
func (pq *PodQuerier) checkKeyExists(ctx context.Context, keyPath string) (bool, error) {
	results, err := pq.queryEvaluator.Evaluate(ctx, keyPath)
	if err != nil {
		return false, err
	}

	// Key exists if we get any non-nil result
	for _, result := range results {
		if result != nil {
			return true, nil
		}
	}
	return false, nil
}

// checkKeyValue returns true if the key equals the expected value
func (pq *PodQuerier) checkKeyValue(ctx context.Context, keyPath, expectedValue string) (bool, error) {
	serializedValue, err := serialize(expectedValue)
	if err != nil {
		return false, fmt.Errorf("failed to serialize selector value: %w", err)
	}

	query := fmt.Sprintf("%s == %s", keyPath, serializedValue)
	results, err := pq.queryEvaluator.Evaluate(ctx, query)
	if err != nil {
		return false, err
	}

	// Equality check: look for explicit true
	for _, result := range results {
		if result == true {
			return true, nil
		}
	}
	return false, nil
}

// ExtractGroupKeys extracts grouping key values from the pod using the provided JQ paths
func (pq *PodQuerier) ExtractGroupKeys(ctx context.Context, keyPaths []string) ([]string, error) {
	if len(keyPaths) == 0 {
		return []string{}, nil
	}

	groupKeys := make([]string, 0, len(keyPaths))

	for _, keyPath := range keyPaths {
		results, err := pq.queryEvaluator.Evaluate(ctx, keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate group key path %q: %w", keyPath, err)
		}

		if err := validateSingleQueryResult(results); err != nil {
			return nil, fmt.Errorf("group key path %q returned an invalid value %v: %w", keyPath, results, err)
		}

		groupKeys = append(groupKeys, fmt.Sprintf("%v", results[0]))
	}

	return groupKeys, nil
}

// PassesFilters returns true if the pod passes all the provided JQ filter expressions
// All filters must pass (AND logic) for the method to return true
func (pq *PodQuerier) PassesFilters(ctx context.Context, filters []string) (bool, error) {
	if len(filters) == 0 {
		return true, nil
	}

	for _, filter := range filters {
		results, err := pq.queryEvaluator.Evaluate(ctx, filter)
		if err != nil {
			return false, fmt.Errorf("failed to evaluate filter %q: %w", filter, err)
		}

		if err := validateSingleQueryResult(results); err != nil {
			return false, fmt.Errorf("filter %q returned an invalid value %v: %w", filter, results, err)
		}

		if results[0] != true {
			return false, nil
		}
	}

	return true, nil
}

func validateSingleQueryResult(results []any) error {
	if len(results) != 1 {
		return fmt.Errorf("expected single query result, got %d", len(results))
	}
	if results[0] == nil || results[0] == "" {
		return fmt.Errorf("query result is empty %v", results[0])
	}
	return nil
}

// serialize serializes a value for use in JQ expressions
func serialize(value string) (string, error) {
	// Use JSON marshaling to properly escape the string for JQ
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
