package rid

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/utils/rid/query"
	corev1 "k8s.io/api/core/v1"
)

// PodMatcher handles matching pods against selectors
type PodMatcher struct {
	queryEvaluator QueryEvaluator
}

// NewPodMatcher creates a new pod matcher for the given pod
func NewPodMatcher(pod corev1.Pod) *PodMatcher {
	return &PodMatcher{
		queryEvaluator: query.NewDefaultJqEvaluator(pod),
	}
}

// Matches returns true if the pod matches the given selector
func (pm *PodMatcher) Matches(ctx context.Context, selector *v1alpha1.PodSelector) (bool, error) {
	if selector == nil {
		return false, nil
	}

	if selector.Value == nil {
		// Existence check: key should exist and not be null
		return pm.checkKeyExists(ctx, selector.KeyPath)
	} else {
		// Equality check: key should equal the specified value
		return pm.checkKeyEquals(ctx, selector.KeyPath, *selector.Value)
	}
}

// checkKeyExists returns true if the key exists and is not null
func (pm *PodMatcher) checkKeyExists(ctx context.Context, keyPath string) (bool, error) {
	results, err := pm.queryEvaluator.Evaluate(ctx, keyPath)
	if err != nil {
		return false, err
	}

	// Key exists if we get any non-null result
	for _, result := range results {
		if result != nil {
			return true, nil
		}
	}
	return false, nil
}

// checkKeyEquals returns true if the key equals the expected value
func (pm *PodMatcher) checkKeyEquals(ctx context.Context, keyPath, expectedValue string) (bool, error) {
	serializedValue, err := serializeForJQ(expectedValue)
	if err != nil {
		return false, fmt.Errorf("failed to serialize selector value: %w", err)
	}

	query := fmt.Sprintf("%s == %s", keyPath, serializedValue)
	results, err := pm.queryEvaluator.Evaluate(ctx, query)
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

// serializeForJQ serializes a value for use in JQ expressions
func serializeForJQ(value string) (string, error) {
	// Use JSON marshaling to properly escape the string for JQ
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
