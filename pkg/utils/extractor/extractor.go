package extractor

import (
	"encoding/json"
	"fmt"

	"github.com/run-ai/runai/kai-bolt/pkg/api/optimization/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// QueryEvaluator interface for query evaluation against data
type QueryEvaluator interface {
	Evaluate(expression string) ([]interface{}, error)
}

// Extractor interface for component data extraction
type Extractor interface {
	GetComponent(name string) (*Component, error)
	ExtractPodTemplateSpecs(component v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error)
}

type ComponentExtractor struct {
	rid            *v1alpha1.ResourceInterpretationDefinition
	object         client.Object
	queryEvaluator QueryEvaluator

	componentDefinitionsByName map[string]v1alpha1.ComponentDefinition // Fast component definition lookup
	componentCaches            map[string]*ComponentCache              // Per-component caches
}

type ComponentCache struct {
	// Cache results by method (no error caching)
	podTemplateSpecs []corev1.PodTemplateSpec
}

type Component struct {
	name       string
	definition v1alpha1.ComponentDefinition
	extractor  Extractor
	cache      *ComponentCache
}

func NewComponentExtractor(rid *v1alpha1.ResourceInterpretationDefinition, object client.Object) *ComponentExtractor {
	// Initialize both maps
	definitionsByName := make(map[string]v1alpha1.ComponentDefinition)
	componentCaches := make(map[string]*ComponentCache)

	allDefinitions := append(rid.Spec.StructureDefinition.ChildComponents, rid.Spec.StructureDefinition.RootComponent)
	for _, componentDefinition := range allDefinitions {
		definitionsByName[componentDefinition.Name] = componentDefinition
		componentCaches[componentDefinition.Name] = &ComponentCache{}
	}

	return &ComponentExtractor{
		rid:                        rid,
		object:                     object,
		componentDefinitionsByName: definitionsByName,
		componentCaches:            componentCaches,
		queryEvaluator:             NewJqEvaluator(object),
	}
}

func (e *ComponentExtractor) GetComponent(name string) (*Component, error) {
	definition, exists := e.componentDefinitionsByName[name]
	if !exists {
		return nil, fmt.Errorf("component %s not found", name)
	}

	// Cache is guaranteed to exist - pre-initialized in constructor
	cache := e.componentCaches[name]

	return &Component{
		name:       name,
		definition: definition,
		extractor:  e,
		cache:      cache,
	}, nil
}

func (c *Component) GetPodTemplateSpec() ([]corev1.PodTemplateSpec, error) {
	// Check component cache first
	if c.cache.podTemplateSpecs != nil {
		return c.cache.podTemplateSpecs, nil
	}

	templates, err := c.extractor.ExtractPodTemplateSpecs(c.definition)
	if err != nil {
		return nil, err
	}

	// Cache successful result
	c.cache.podTemplateSpecs = templates
	return templates, nil
}

func (e *ComponentExtractor) ExtractPodTemplateSpecs(definition v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, fmt.Errorf("component %s does not have spec definition", definition.Name)
	}

	if definition.SpecDefinition.PodTemplateSpecPath != nil {
		results, err := e.queryEvaluator.Evaluate(*definition.SpecDefinition.PodTemplateSpecPath)
		if err != nil {
			return nil, err
		}

		return safeConvertSliceType[corev1.PodTemplateSpec](results)
	}

	// try build from fragmented

	return nil, nil
}

// Generic type conversion for slice objects
func safeConvertSliceType[T any](slice []interface{}) ([]T, error) {
	if slice == nil {
		return nil, nil
	}

	convertedResults := make([]T, len(slice))
	for i, object := range slice {
		// First try direct type assertion (for simple types)
		if converted, ok := object.(T); ok {
			convertedResults[i] = converted
			continue
		}

		// For complex types, object is map[string]interface{}. use JSON marshaling/unmarshaling to convert
		var converted T
		if err := jsonConversion(object, &converted); err != nil {
			return nil, fmt.Errorf("failed to convert object at index %d to type %T: %w", i, converted, err)
		}
		convertedResults[i] = converted
	}

	return convertedResults, nil
}

// jsonConversion converts between types using JSON marshaling
func jsonConversion(src interface{}, dst interface{}) error {
	jsonBytes, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal source: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, dst); err != nil {
		return fmt.Errorf("failed to unmarshal to destination: %w", err)
	}

	return nil
}
