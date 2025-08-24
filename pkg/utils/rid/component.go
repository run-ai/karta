package rid

import (
	"github.com/run-ai/runai/kai-bolt/pkg/api/optimization/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// ComponentCache holds cached results for a component (no error caching)
type ComponentCache struct {
	// Cache results by method
	podTemplateSpecs   []corev1.PodTemplateSpec
	fragmentedPodSpecs []FragmentedPodSpec
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
	// Check component cache first
	if c.cache.podTemplateSpecs != nil {
		return c.cache.podTemplateSpecs, nil
	}

	templates, err := c.extractor.ExtractPodTemplateSpec(c.definition)
	if err != nil {
		return nil, err
	}

	// Cache successful result
	c.cache.podTemplateSpecs = templates
	return templates, nil
}

// GetFragmentedPodSpec extracts and caches fragmented pod specs for this component
func (c *Component) GetFragmentedPodSpec() ([]FragmentedPodSpec, error) {
	// Check component cache first
	if c.cache.fragmentedPodSpecs != nil {
		return c.cache.fragmentedPodSpecs, nil
	}

	fragmented, err := c.extractor.ExtractFragmentedPodSpec(c.definition)
	if err != nil {
		return nil, err
	}

	// Cache successful result
	c.cache.fragmentedPodSpecs = fragmented
	return fragmented, nil
}

// Name returns the component name
func (c *Component) Name() string {
	return c.name
}

// Definition returns the component definition
func (c *Component) Definition() v1alpha1.ComponentDefinition {
	return c.definition
}
