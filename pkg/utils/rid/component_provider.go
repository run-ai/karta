package rid

import (
	"context"
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/utils/rid/query"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Extractor interface for extracting typed data from component definitions
type Extractor interface {
	ExtractPodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error)
	ExtractFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]FragmentedPodSpec, error)
	ExtractPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodSpec, error)
	ExtractPodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]metav1.ObjectMeta, error)
	ExtractScale(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]Scale, error)
}

// ComponentProvider implements ComponentProvider using RID definitions
type ComponentProvider struct {
	rid       *v1alpha1.ResourceInterpretationDefinition
	extractor Extractor // Shared extractor instance

	componentDefinitionsByName map[string]v1alpha1.ComponentDefinition // Fast component definition lookup
	componentCaches            map[string]*ComponentCache              // Per-component caches
}

// NewComponentProvider creates a new RID-based component provider
func NewComponentProvider(rid *v1alpha1.ResourceInterpretationDefinition, object client.Object) *ComponentProvider {
	// Create shared query evaluator (singleton)
	queryEvaluator := query.NewDefaultJqEvaluator(object)

	// Create shared extractor
	extractor := NewComponentExtractor(queryEvaluator)

	// Initialize component maps
	definitionsByName := make(map[string]v1alpha1.ComponentDefinition)
	componentCaches := make(map[string]*ComponentCache)

	// Create single slice with all components (root + children)
	allDefinitions := append(rid.Spec.StructureDefinition.ChildComponents, rid.Spec.StructureDefinition.RootComponent)
	for _, componentDefinition := range allDefinitions {
		definitionsByName[componentDefinition.Name] = componentDefinition
		componentCaches[componentDefinition.Name] = &ComponentCache{}
	}

	return &ComponentProvider{
		rid:                        rid,
		extractor:                  extractor,
		componentDefinitionsByName: definitionsByName,
		componentCaches:            componentCaches,
	}
}

// GetComponent retrieves a component by name
func (p *ComponentProvider) GetComponent(name string) (*Component, error) {
	definition, exists := p.componentDefinitionsByName[name]
	if !exists {
		return nil, fmt.Errorf("component %s not found", name)
	}

	// Cache is guaranteed to exist - pre-initialized in constructor
	cache := p.componentCaches[name]

	return &Component{
		name:       name,
		definition: definition,
		extractor:  p.extractor, // Shared extractor
		cache:      cache,
	}, nil
}

// GetRootComponent retrieves the root component
func (p *ComponentProvider) GetRootComponent() (*Component, error) {
	if p.rid == nil {
		return nil, fmt.Errorf("rid is nil")
	}

	return p.GetComponent(p.rid.Spec.StructureDefinition.RootComponent.Name)
}
