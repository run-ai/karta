package resource

import (
	"context"
	"errors"
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const errGetInstanceIds = "failed to get instance ids"

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

// GetPodTemplateSpec extracts pod template specs mapped by instance id
func (c *Component) GetPodTemplateSpec(ctx context.Context) (map[string]corev1.PodTemplateSpec, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	podTemplateSpecs, err := c.extractor.ExtractPodTemplateSpec(ctx, c.definition)
	if err != nil {
		return nil, fmt.Errorf("failed to extract pod template specs: %w", err)
	}

	return zipWithInstanceIds(instanceIds, podTemplateSpecs)
}

// GetFragmentedPodSpec extracts fragmented pod specs mapped by instance id
func (c *Component) GetFragmentedPodSpec(ctx context.Context) (map[string]FragmentedPodSpec, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	fragmentedPodSpecs, err := c.extractor.ExtractFragmentedPodSpec(ctx, c.definition)
	if err != nil {
		return nil, fmt.Errorf("failed to extract fragmented pod specs: %w", err)
	}

	return zipWithInstanceIds(instanceIds, fragmentedPodSpecs)
}

// GetPodSpec extracts pod specs mapped by instance id
func (c *Component) GetPodSpec(ctx context.Context) (map[string]corev1.PodSpec, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	podSpecs, err := c.extractor.ExtractPodSpec(ctx, c.definition)
	if err != nil {
		return nil, fmt.Errorf("failed to extract pod specs: %w", err)
	}

	return zipWithInstanceIds(instanceIds, podSpecs)
}

// GetPodMetadata extracts pod metadata mapped by instance id
func (c *Component) GetPodMetadata(ctx context.Context) (map[string]metav1.ObjectMeta, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	podMetadata, err := c.extractor.ExtractPodMetadata(ctx, c.definition)
	if err != nil {
		return nil, fmt.Errorf("failed to extract pod metadata: %w", err)
	}

	return zipWithInstanceIds(instanceIds, podMetadata)
}

// GetScale extracts scale data mapped by instance id
func (c *Component) GetScale(ctx context.Context) (map[string]Scale, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	scales, err := c.extractor.ExtractScale(ctx, c.definition)
	if err != nil {
		return nil, fmt.Errorf("failed to extract scales: %w", err)
	}

	return zipWithInstanceIds(instanceIds, scales)
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

// HasInstanceIdDefinition returns true if this component possibily has multiple instances
func (c *Component) HasInstanceIdDefinition() bool {
	return c.definition.InstanceIdPath != nil
}

// GetInstanceIds extracts instance identifiers for this component
func (c *Component) GetInstanceIds(ctx context.Context) ([]string, error) {
	instanceIds, err := c.extractor.ExtractInstanceIds(ctx, c.definition)
	if err != nil {
		// Check if it's a definition not found error (no instanceIdPath)
		var defNotFoundErr DefinitionNotFoundError
		if errors.As(err, &defNotFoundErr) {
			// If no definition was given, assume there is a single instance with empty id
			return []string{""}, nil
		}
		return nil, fmt.Errorf("failed to extract instance ids for component %s: %w", c.name, err)
	}

	return instanceIds, nil
}

// zipWithInstanceIds is a generic method to zip instance IDs with extraction results
func zipWithInstanceIds[T any](instanceIds []string, results []T) (map[string]T, error) {
	if len(instanceIds) != len(results) {
		return nil, fmt.Errorf("instance ids count (%d) does not match results count (%d)", len(instanceIds), len(results))
	}

	zipped := make(map[string]T, len(instanceIds))
	for i, instanceId := range instanceIds {
		zipped[instanceId] = results[i]
	}

	return zipped, nil
}
