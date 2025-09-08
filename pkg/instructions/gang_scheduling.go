package instructions

import (
	"context"
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/resource"
)

// PodGroupingEffectiveComponent contains the effective component information for pod grouping
type PodGroupingEffectiveComponent struct {
	EffectiveComponent string                             // the actual effective component name
	PodGroupName       string                             // the pod group name this belongs to
	MemberDefinition   *v1alpha1.PodGroupMemberDefinition // the specific member definition with filters
}

// GetPodGroupingEffectiveComponent - Main entry point for pod grouping plugin
func GetPodGroupingEffectiveComponent(ctx context.Context, podQuerier *resource.PodQuerier, summary *StructureSummary) (*PodGroupingEffectiveComponent, error) {
	// Infer which component this pod belongs to
	componentName, err := inferPodComponent(ctx, podQuerier, summary)
	if err != nil {
		return nil, err
	}

	// Get effective component candidate for gang scheduling (if any)
	candidate, err := getEffectiveComponentForPod(ctx, podQuerier, componentName, summary)
	if err != nil {
		return nil, err
	}

	// candidate can be nil if none was found
	return candidate, nil
}

// inferPodComponent infers the component name for the given pod
func inferPodComponent(ctx context.Context, podQuerier *resource.PodQuerier, summary *StructureSummary) (string, error) {
	if len(summary.leafComponents) == 1 {
		return summary.leafComponents[0], nil
	}

	// Only check leaf components (have pod definitions)
	for _, componentName := range summary.leafComponents {
		leafDefinition := summary.componentDefinitionsByName[componentName]

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

// getEffectiveComponentForPod dynamically determines the effective component for a specific pod
func getEffectiveComponentForPod(ctx context.Context, podQuerier *resource.PodQuerier, componentName string, summary *StructureSummary) (*PodGroupingEffectiveComponent, error) {
	if summary.gangSchedulingSummary == nil {
		return nil, nil
	}

	// Get pre-computed and pre-sorted candidates for this component
	candidates, exists := summary.gangSchedulingSummary.effectiveComponentCandidates[componentName]
	if !exists || len(candidates) == 0 {
		return nil, nil
	}

	// Try each candidate (already sorted by priority), return first match
	for _, candidate := range candidates {
		// If no filters, always matches
		if len(candidate.member.Filters) == 0 {
			return &PodGroupingEffectiveComponent{
				EffectiveComponent: candidate.effectiveComponent,
				PodGroupName:       candidate.podGroupName,
				MemberDefinition:   candidate.member,
			}, nil
		}

		// Check if pod passed all filters (ANDed)
		passed, err := podQuerier.PassesFilters(ctx, candidate.member.Filters)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate filters for component %s in group %s: %w", candidate.effectiveComponent, candidate.podGroupName, err)
		}

		if passed {
			return &PodGroupingEffectiveComponent{
				EffectiveComponent: candidate.effectiveComponent,
				PodGroupName:       candidate.podGroupName,
				MemberDefinition:   candidate.member,
			}, nil
		}
	}

	// No matches found
	return nil, nil
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
	children := summary.childrenMap[componentName]
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
