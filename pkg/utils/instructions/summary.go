package instructions

import (
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
)

// StructureSummary provides a pre-computed summary of ResourceInterface structure
// for efficient navigation and lookup operations
type StructureSummary struct {
	RI                                *v1alpha1.ResourceInterface
	ParentMap                         map[string]string                        // child component name -> parent component name
	ChildrenMap                       map[string][]string                      // parent component name -> list of child component names
	ComponentDefs                     map[string]*v1alpha1.ComponentDefinition // component name -> component definition
	LeafComponents                    []string                                 // list of component names that have pod definitions
	EffectiveGangSchedulingComponents map[string]string                        // component name -> effective gang scheduling component name
}

// NewStructureSummary creates a new StructureSummary by analyzing the ResourceInterface structure
func NewStructureSummary(ri *v1alpha1.ResourceInterface) (*StructureSummary, error) {
	if ri == nil {
		return nil, fmt.Errorf("resource interface cannot be nil")
	}

	summary := &StructureSummary{
		RI:                                ri,
		ParentMap:                         make(map[string]string),
		ChildrenMap:                       make(map[string][]string),
		ComponentDefs:                     make(map[string]*v1alpha1.ComponentDefinition),
		LeafComponents:                    make([]string, 0),
		EffectiveGangSchedulingComponents: make(map[string]string),
	}

	if err := summary.build(); err != nil {
		return nil, fmt.Errorf("failed to build structure summary: %w", err)
	}

	return summary, nil
}

// buildMaps constructs all the lookup maps and metadata from the ResourceInterface
func (s *StructureSummary) build() error {
	// Process root component
	rootComponent := s.RI.Spec.StructureDefinition.RootComponent
	s.ComponentDefs[rootComponent.Name] = &rootComponent

	// Check if root has pod definition
	if hasPodDefinition(rootComponent) {
		s.LeafComponents = append(s.LeafComponents, rootComponent.Name)
	}

	// Process child components
	for _, childComponent := range s.RI.Spec.StructureDefinition.ChildComponents {
		// Add to component definitions map
		s.ComponentDefs[childComponent.Name] = &childComponent

		// Check if child has pod definition
		if hasPodDefinition(childComponent) {
			s.LeafComponents = append(s.LeafComponents, childComponent.Name)
		}

		// Build parent-child relationships (only for child components with OwnerRef)
		if childComponent.OwnerRef == nil {
			return fmt.Errorf("child component %s must have OwnerRef", childComponent.Name)
		}

		parentName := *childComponent.OwnerRef

		// Add to parent map
		s.ParentMap[childComponent.Name] = parentName

		// Add to children map
		s.ChildrenMap[parentName] = append(s.ChildrenMap[parentName], childComponent.Name)
	}

	// Build a map of component name to effective component name for gang scheduling
	var err error
	s.EffectiveGangSchedulingComponents, err = buildGangSchedulingEffectiveComponents(s.RI, s.ParentMap)
	if err != nil {
		return fmt.Errorf("failed to build gang scheduling effective components: %w", err)
	}

	return nil
}

// hasPodDefinition returns true if the component has any pod-related definition
func hasPodDefinition(component v1alpha1.ComponentDefinition) bool {
	if component.SpecDefinition == nil {
		return false
	}

	return component.SpecDefinition.PodTemplateSpecPath != nil ||
		component.SpecDefinition.PodSpecPath != nil ||
		component.SpecDefinition.FragmentedPodSpecDefinition != nil
}

func buildGangSchedulingEffectiveComponents(ri *v1alpha1.ResourceInterface, parentMap map[string]string) (map[string]string, error) {
	effectiveComponents := make(map[string]string)

	if ri.Spec.Instructions.GangScheduling == nil {
		return effectiveComponents, nil
	}

	// Build a map of component name to pod group name, of all components that are part of any pod group
	memberToGroupMap := make(map[string]string)
	for _, group := range ri.Spec.Instructions.GangScheduling.PodGroups {
		for _, member := range group.Members {
			memberToGroupMap[member.ComponentName] = group.Name
		}
	}

	// For every component, find its effective component (the component/parent component that is part of any pod group definition)
	for _, component := range ri.Spec.StructureDefinition.ChildComponents {
		if _, ok := memberToGroupMap[component.Name]; ok {
			effectiveComponent, err := findEffectiveComponent(component.Name, parentMap, memberToGroupMap)
			if err != nil {
				return effectiveComponents, err
			}
			effectiveComponents[component.Name] = effectiveComponent
		}
	}

	return effectiveComponents, nil
}

func findEffectiveComponent(startComponent string, parentMap map[string]string, memberToGroupMap map[string]string) (string, error) {
	current := startComponent

	for current != "" {
		// Check if current component is mentioned in any pod group definition
		if _, ok := memberToGroupMap[current]; ok {
			return current, nil
		}

		// Move to parent using direct map access
		current = parentMap[current]
	}

	return "", fmt.Errorf("no effective component found for %s", startComponent)
}
