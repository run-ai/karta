package rid

import (
	"context"

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
	scale              []Scale
}

// Component represents a RID component with extraction capabilities
type Component struct {
	name       string
	definition v1alpha1.ComponentDefinition
	extractor  Extractor
	cache      *ComponentCache
}

type FragmentedPodSpec struct {
	SchedulerName     string                      `json:"schedulerName,omitempty"`
	Labels            map[string]string           `json:"labels,omitempty"`
	Annotations       map[string]string           `json:"annotations,omitempty"`
	Resources         corev1.ResourceRequirements `json:"resources,omitempty"`
	ResourceClaims    []corev1.PodResourceClaim   `json:"resourceClaims,omitempty"`
	PodAffinity       *corev1.PodAffinity         `json:"podAffinity,omitempty"`
	NodeAffinity      *corev1.NodeAffinity        `json:"nodeAffinity,omitempty"`
	Containers        []corev1.Container          `json:"containers,omitempty"`
	PriorityClassName string                      `json:"priorityClassName,omitempty"`
	Image             string                      `json:"image,omitempty"`
}

type Scale struct {
	Replicas    *int32 `json:"replicas,omitempty"`
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
}

// Name returns the component name
func (c *Component) Name() string {
	return c.name
}

// Kind returns the component gvk
func (c *Component) Kind() *metav1.GroupVersionKind {
	if c.definition.Kind == nil {
		return nil
	}

	return &metav1.GroupVersionKind{
		Group:   c.definition.Kind.Group,
		Version: c.definition.Kind.Version,
		Kind:    c.definition.Kind.Kind,
	}
}

// Definition returns the component definition
func (c *Component) Definition() v1alpha1.ComponentDefinition {
	return c.definition
}

// GetPodTemplateSpec extracts and caches pod template specs for this component
func (c *Component) GetPodTemplateSpec(ctx context.Context) ([]corev1.PodTemplateSpec, error) {
	return getField(ctx, c, &c.cache.podTemplateSpecs, c.extractor.ExtractPodTemplateSpec)
}

// GetFragmentedPodSpec extracts and caches fragmented pod specs for this component
func (c *Component) GetFragmentedPodSpec(ctx context.Context) ([]FragmentedPodSpec, error) {
	return getField(ctx, c, &c.cache.fragmentedPodSpecs, c.extractor.ExtractFragmentedPodSpec)
}

// GetPodSpec extracts and caches pod spec for this component
func (c *Component) GetPodSpec(ctx context.Context) ([]corev1.PodSpec, error) {
	return getField(ctx, c, &c.cache.podSpecs, c.extractor.ExtractPodSpec)
}

// GetPodMetadata extracts and caches pod metadata for this component
func (c *Component) GetPodMetadata(ctx context.Context) ([]metav1.ObjectMeta, error) {
	return getField(ctx, c, &c.cache.podMetadata, c.extractor.ExtractPodMetadata)
}

// GetScale extracts and caches scale data for this component
func (c *Component) GetScale(ctx context.Context) ([]Scale, error) {
	return getField(ctx, c, &c.cache.scale, c.extractor.ExtractScale)
}

// HasPodDefinition returns true if this component defines pods
func (c *Component) HasPodDefinition() bool {
	spec := c.definition.SpecDefinition
	if spec == nil {
		return false
	}

	return spec.PodTemplateSpecPath != nil ||
		spec.PodSpecPath != nil ||
		spec.FragmentedPodSpecDefinition != nil
}

// GetPodSelector returns the pod selector for this component
func (c *Component) GetPodSelector() *v1alpha1.PodSelector {
	return c.definition.PodSelector
}

func getField[T any](ctx context.Context, component *Component, cacheEntry *[]T, extractionFn func(context.Context, v1alpha1.ComponentDefinition) ([]T, error)) ([]T, error) {
	// Check component cache first
	if *cacheEntry != nil {
		return *cacheEntry, nil
	}

	extracted, err := extractionFn(ctx, component.definition)
	if err != nil {
		return nil, err
	}

	// Cache successful result
	*cacheEntry = extracted
	return extracted, nil
}
