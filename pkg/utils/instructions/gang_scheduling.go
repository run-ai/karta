package instructions

import (
	"context"
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/utils/resource"
	corev1 "k8s.io/api/core/v1"
)

func InferPodComponent(ctx context.Context, pod *corev1.Pod, summary *StructureSummary) (string, error) {
	if len(summary.LeafComponents) == 1 {
		return summary.LeafComponents[0], nil
	}

	// PodQuerier is lazy, no problem to init first
	podQuerier := resource.NewPodQuerier(*pod)

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

	return "", fmt.Errorf("no component found for pod %s", pod.Name)
}

func GetEffectiveComponent(componentName string, summary *StructureSummary) (string, bool) {
	effective, exists := summary.EffectiveGangSchedulingComponents[componentName]
	return effective, exists
}
