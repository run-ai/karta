package resource

import (
	"context"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Component represents a ResourceInterface component with extraction capabilities
type Component struct {
	name       string
	definition v1alpha1.ComponentDefinition
	extractor  Extractor
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
	Container         corev1.Container            `json:"container,omitempty"`
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

// GetPodTemplateSpec extracts pod template specs for this component
func (c *Component) GetPodTemplateSpec(ctx context.Context) ([]corev1.PodTemplateSpec, error) {
	return c.extractor.ExtractPodTemplateSpec(ctx, c.definition)
}

// GetFragmentedPodSpec extracts fragmented pod specs for this component
func (c *Component) GetFragmentedPodSpec(ctx context.Context) ([]FragmentedPodSpec, error) {
	return c.extractor.ExtractFragmentedPodSpec(ctx, c.definition)
}

// GetPodSpec extracts pod spec for this component
func (c *Component) GetPodSpec(ctx context.Context) ([]corev1.PodSpec, error) {
	return c.extractor.ExtractPodSpec(ctx, c.definition)
}

// GetPodMetadata extracts pod metadata for this component
func (c *Component) GetPodMetadata(ctx context.Context) ([]metav1.ObjectMeta, error) {
	return c.extractor.ExtractPodMetadata(ctx, c.definition)
}

// GetScale extracts scale data for this component
func (c *Component) GetScale(ctx context.Context) ([]Scale, error) {
	return c.extractor.ExtractScale(ctx, c.definition)
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
