package rid

import (
	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentCache holds cached results for a component (no error caching)
type ComponentCache struct {
	// Cache results by method
	podTemplateSpecs   []corev1.PodTemplateSpec
	fragmentedPodSpecs []FragmentedPodSpec
	podSpecs           []corev1.PodSpec
	podMetadata        []metav1.ObjectMeta
}

// Component represents a RID component with extraction capabilities
type Component struct {
	name       string
	definition v1alpha1.ComponentDefinition
	extractor  Extractor
	cache      *ComponentCache
}

// GetPodTemplateSpec extracts and caches pod template specs for this component
func (c *Component) GetPodTemplateSpec() ([]corev1.PodTemplateSpec, error) {
	return getField(c, &c.cache.podTemplateSpecs, c.extractor.ExtractPodTemplateSpec)
}

// GetFragmentedPodSpec extracts and caches fragmented pod specs for this component
func (c *Component) GetFragmentedPodSpec() ([]FragmentedPodSpec, error) {
	return getField(c, &c.cache.fragmentedPodSpecs, c.extractor.ExtractFragmentedPodSpec)
}

// GetPodSpec extracts and caches pod spec for this component
func (c *Component) GetPodSpec() ([]corev1.PodSpec, error) {
	return getField(c, &c.cache.podSpecs, c.extractor.ExtractPodSpec)
}

// GetPodMetadata extracts and caches pod metadata for this component
func (c *Component) GetPodMetadata() ([]metav1.ObjectMeta, error) {
	return getField(c, &c.cache.podMetadata, c.extractor.ExtractPodMetadata)
}

func getField[T any](component *Component, cacheEntry *[]T, extractionFn func(v1alpha1.ComponentDefinition) ([]T, error)) ([]T, error) {
	// Check component cache first
	if *cacheEntry != nil {
		return *cacheEntry, nil
	}

	extracted, err := extractionFn(component.definition)
	if err != nil {
		return nil, err
	}

	// Cache successful result
	*cacheEntry = extracted
	return extracted, nil
}

// Name returns the component name
func (c *Component) Name() string {
	return c.name
}

// Definition returns the component definition
func (c *Component) Definition() v1alpha1.ComponentDefinition {
	return c.definition
}
