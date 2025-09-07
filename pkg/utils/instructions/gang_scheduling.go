package instructions

import (
	"context"
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/utils/resource"
)

func InferPodComponent(ctx context.Context, podQuerier *resource.PodQuerier, summary *StructureSummary) (string, error) {
	if len(summary.LeafComponents) == 1 {
		return summary.LeafComponents[0], nil
	}

	// Only check leaf components (have pod definitions)
	for _, componentName := range summary.LeafComponents {
		leafDefinition := summary.ComponentDefs[componentName]

		// Check if pod matches this component's selector
		matches, err := podQuerier.Matches(ctx, leafDefinition.PodSelector)
		if err != nil {
			return "", fmt.Errorf("failed to check if pod matches component %s: %w", componentName, err)
		}
		if matches {
			return componentName, nil
		}
	}

	return "", fmt.Errorf("no component found for pod %s", podQuerier.GetPodName())
}

func GetEffectiveComponent(componentName string, summary *StructureSummary) (string, bool) {
	if summary.GangSchedulingSummary == nil {
		return "", false
	}

	effective, exists := summary.GangSchedulingSummary.EffectiveComponents[componentName]
	return effective, exists
}

func GetPodGroupForComponent(componentName string, summary *StructureSummary) (*v1alpha1.PodGroupDefinition, bool) {
	if summary.GangSchedulingSummary == nil {
		return nil, false
	}

	effective, hasEffective := GetEffectiveComponent(componentName, summary)
	if !hasEffective {
		return nil, false
	}

	group, exists := summary.GangSchedulingSummary.PodGroups[effective]
	return group, exists
}

// CalculateSubtreeScale calculates the aggregated scale for a component subtree
func CalculateSubtreeScale(ctx context.Context, componentName string, factory *resource.ComponentFactory, summary *StructureSummary) (int32, error) {
	// Get this component's scale
	component, err := factory.GetComponent(componentName)
	if err != nil {
		return 0, err
	}

	scales, err := component.GetScale(ctx)
	if err != nil {
		return 0, err
	}

	var currentComponentTotalScale int32
	// Sum all scales for this component (array/map cases)
	for _, scale := range scales {
		currentComponentTotalScale += getEffectiveMinReplicas(&scale)
	}

	// Get children
	children := summary.ChildrenMap[componentName]
	if len(children) == 0 {
		// Leaf component - return its total scale
		return currentComponentTotalScale, nil
	}

	// Parent component - calculate children sum, then multiply by parent scale
	var childrenSum int32
	for _, child := range children {
		childScale, err := CalculateSubtreeScale(ctx, child, factory, summary)
		if err != nil {
			return 0, err
		}
		childrenSum += childScale
	}

	// If child components have no scale definitions, assume the scale is defined in a higher level
	if childrenSum == 0 {
		return currentComponentTotalScale, nil
	}

	// If current component has no scale definitions, carry over the children sum
	if currentComponentTotalScale == 0 {
		return childrenSum, nil
	}

	// If both current component and children have scale definitions, multiply the current component scale by the children sum
	return currentComponentTotalScale * childrenSum, nil
}

// getEffectiveMinReplicas determines the minimum replicas for a component
func getEffectiveMinReplicas(scale *resource.Scale) int32 {
	if scale.MinReplicas != nil && *scale.MinReplicas > 0 {
		return *scale.MinReplicas
	}

	if scale.Replicas != nil && *scale.Replicas > 0 {
		return *scale.Replicas
	}

	return 0 // Assume the scale is defined in a higher level
}
