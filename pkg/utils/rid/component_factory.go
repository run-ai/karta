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

type Extractor interface {
	ExtractPodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error)
	ExtractFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]FragmentedPodSpec, error)
	ExtractPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodSpec, error)
	ExtractPodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]metav1.ObjectMeta, error)
	ExtractScale(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]Scale, error)
}

type ComponentFactory struct {
	rid       *v1alpha1.ResourceInterpretationDefinition
	extractor Extractor // Shared extractor instance

	componentDefinitionsByName map[string]v1alpha1.ComponentDefinition // Fast component definition lookup
	componentCaches            map[string]*ComponentCache              // Per-component caches
}

// NewComponentFactory creates a new RID-based component factory
func NewComponentFactory(rid *v1alpha1.ResourceInterpretationDefinition, object client.Object) *ComponentFactory {
	queryEvaluator := query.NewDefaultJqEvaluator(object)
	extractor := NewRidExtractor(queryEvaluator)

	definitionsByName := make(map[string]v1alpha1.ComponentDefinition)
	componentCaches := make(map[string]*ComponentCache)

	// Create single slice with all components (root + children)
	allDefinitions := append(rid.Spec.StructureDefinition.ChildComponents, rid.Spec.StructureDefinition.RootComponent)
	for _, componentDefinition := range allDefinitions {
		definitionsByName[componentDefinition.Name] = componentDefinition
		componentCaches[componentDefinition.Name] = &ComponentCache{}
	}

	return &ComponentFactory{
		rid:                        rid,
		extractor:                  extractor,
		componentDefinitionsByName: definitionsByName,
		componentCaches:            componentCaches,
	}
}

// GetComponent retrieves a component by name
func (f *ComponentFactory) GetComponent(name string) (*Component, error) {
	definition, exists := f.componentDefinitionsByName[name]
	if !exists {
		return nil, fmt.Errorf("component %s not found", name)
	}

	// Cache is guaranteed to exist - pre-initialized in constructor
	cache := f.componentCaches[name]

	return &Component{
		name:       name,
		definition: definition,
		extractor:  f.extractor,
		cache:      cache,
	}, nil
}

// GetRootComponent retrieves the root component
func (f *ComponentFactory) GetRootComponent() (*Component, error) {
	if f.rid == nil {
		return nil, fmt.Errorf("rid is nil")
	}

	return f.GetComponent(f.rid.Spec.StructureDefinition.RootComponent.Name)
}
