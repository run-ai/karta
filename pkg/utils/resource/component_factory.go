package resource

import (
	"context"
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/utils/resource/query"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate mockgen -source=component_factory.go -destination=extractor_mock.go -package=resource Extractor
type Extractor interface {
	ExtractPodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error)
	ExtractFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]FragmentedPodSpec, error)
	ExtractPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodSpec, error)
	ExtractPodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]metav1.ObjectMeta, error)
	ExtractScale(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]Scale, error)
}

type ComponentFactory struct {
	ri        *v1alpha1.ResourceInterface
	extractor Extractor // Shared extractor instance

	componentDefinitionsByName map[string]v1alpha1.ComponentDefinition // Fast component definition lookup
	componentCaches            map[string]*ComponentCache              // Per-component caches
}

// NewComponentFactory creates a new ResourceInterface-based component factory
func NewComponentFactory(ri *v1alpha1.ResourceInterface, extractor Extractor) *ComponentFactory {
	definitionsByName := make(map[string]v1alpha1.ComponentDefinition)
	componentCaches := make(map[string]*ComponentCache)

	// Create single slice with all components (root + children)
	allDefinitions := append(ri.Spec.StructureDefinition.ChildComponents, ri.Spec.StructureDefinition.RootComponent)
	for _, componentDefinition := range allDefinitions {
		definitionsByName[componentDefinition.Name] = componentDefinition
		componentCaches[componentDefinition.Name] = &ComponentCache{}
	}

	return &ComponentFactory{
		ri:                         ri,
		extractor:                  extractor,
		componentDefinitionsByName: definitionsByName,
		componentCaches:            componentCaches,
	}
}

// NewComponentFactoryFromObject creates a new ResourceInterface-based component factory from a Kubernetes object
func NewComponentFactoryFromObject(ri *v1alpha1.ResourceInterface, object client.Object) *ComponentFactory {
	queryEvaluator := query.NewDefaultJqEvaluator(object)
	extractor := NewInterfaceExtractor(queryEvaluator)
	return NewComponentFactory(ri, extractor)
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
	if f.ri == nil {
		return nil, fmt.Errorf("resource interface is nil")
	}

	return f.GetComponent(f.ri.Spec.StructureDefinition.RootComponent.Name)
}
