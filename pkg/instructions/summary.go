package instructions

import (
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
)

// StructureSummary provides a pre-computed summary of ResourceInterface structure
// for efficient navigation and lookup operations
type StructureSummary struct {
	ri                    *v1alpha1.ResourceInterface
	parentMap             map[string]string                        // child component name -> parent component name
	childrenMap           map[string][]string                      // parent component name -> list of child component names
	componentDefs         map[string]*v1alpha1.ComponentDefinition // component name -> component definition
	leafComponents        []string                                 // list of component names that have pod definitions
	gangSchedulingSummary *gangSchedulingSummary                   // summary of gang scheduling instructions
}

type gangSchedulingSummary struct {
	effectiveComponents map[string]string                       // component name -> effective gang scheduling component name
	podGroups           map[string]*v1alpha1.PodGroupDefinition // component name -> pod group definition
}

// NewStructureSummary creates a new StructureSummary by analyzing the ResourceInterface structure
func NewStructureSummary(ri *v1alpha1.ResourceInterface) (*StructureSummary, error) {
	if ri == nil {
		return nil, fmt.Errorf("resource interface cannot be nil")
	}

	summary := &StructureSummary{
		ri:             ri,
		parentMap:      make(map[string]string),
		childrenMap:    make(map[string][]string),
		componentDefs:  make(map[string]*v1alpha1.ComponentDefinition),
		leafComponents: make([]string, 0),
	}

	if err := summary.build(); err != nil {
		return nil, fmt.Errorf("failed to build structure summary: %w", err)
	}

	return summary, nil
}

// buildMaps constructs all the lookup maps and metadata from the ResourceInterface
func (s *StructureSummary) build() error {
	// Process root component
	rootComponent := s.ri.Spec.StructureDefinition.RootComponent
	s.componentDefs[rootComponent.Name] = &rootComponent

	// Check if root has pod definition
	if hasPodDefinition(rootComponent) {
		s.leafComponents = append(s.leafComponents, rootComponent.Name)
	}

	// Process child components
	for _, childComponent := range s.ri.Spec.StructureDefinition.ChildComponents {
		// Add to component definitions map
		s.componentDefs[childComponent.Name] = &childComponent

		// Check if child has pod definition
		if hasPodDefinition(childComponent) {
			s.leafComponents = append(s.leafComponents, childComponent.Name)
		}

		// Build parent-child relationships (only for child components with OwnerRef)
		if childComponent.OwnerRef == nil {
			return fmt.Errorf("child component %s must have OwnerRef", childComponent.Name)
		}

		parentName := *childComponent.OwnerRef

		// Add to parent map
		s.parentMap[childComponent.Name] = parentName

		// Add to children map
		s.childrenMap[parentName] = append(s.childrenMap[parentName], childComponent.Name)
	}

	if s.ri.Spec.Instructions.GangScheduling != nil {
		s.gangSchedulingSummary = &gangSchedulingSummary{
			podGroups: make(map[string]*v1alpha1.PodGroupDefinition),
		}

		// Build a map of component name to pod group name, of all components that are part of any pod group
		for _, group := range s.ri.Spec.Instructions.GangScheduling.PodGroups {
			for _, member := range group.Members {
				s.gangSchedulingSummary.podGroups[member.ComponentName] = &group
			}
		}

		// Build a map of component name to effective component name for gang scheduling
		var err error
		s.gangSchedulingSummary.effectiveComponents, err = buildGangSchedulingEffectiveComponents(s.ri, s.gangSchedulingSummary.podGroups, s.parentMap)
		if err != nil {
			return fmt.Errorf("failed to build gang scheduling effective components: %w", err)
		}
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

func buildGangSchedulingEffectiveComponents(ri *v1alpha1.ResourceInterface, memberToGroupMap map[string]*v1alpha1.PodGroupDefinition, parentMap map[string]string) (map[string]string, error) {
	effectiveComponents := make(map[string]string)

	if ri.Spec.Instructions.GangScheduling == nil {
		return effectiveComponents, nil
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

func findEffectiveComponent(startComponent string, parentMap map[string]string, memberToGroupMap map[string]*v1alpha1.PodGroupDefinition) (string, error) {
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
