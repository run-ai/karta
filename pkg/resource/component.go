package resource

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
)

const errGetInstanceIds = "failed to get instance ids"

// Component represents a ResourceInterface component with extraction capabilities
type Component struct {
	name       string
	definition v1alpha1.ComponentDefinition
	extractor  Extractor
}

type FragmentedPodSpec struct {
	SchedulerName     string                       `json:"schedulerName,omitempty"`
	Labels            map[string]string            `json:"labels,omitempty"`
	Annotations       map[string]string            `json:"annotations,omitempty"`
	Resources         *corev1.ResourceRequirements `json:"resources,omitempty"`
	ResourceClaims    []corev1.PodResourceClaim    `json:"resourceClaims,omitempty"`
	PodAffinity       *corev1.PodAffinity          `json:"podAffinity,omitempty"`
	NodeAffinity      *corev1.NodeAffinity         `json:"nodeAffinity,omitempty"`
	Containers        []corev1.Container           `json:"containers,omitempty"`
	Container         *corev1.Container            `json:"container,omitempty"`
	PriorityClassName string                       `json:"priorityClassName,omitempty"`
	Image             string                       `json:"image,omitempty"`
}

type Scale struct {
	Replicas    *int32 `json:"replicas,omitempty"`
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
}

type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Status struct {
	// Phase is the raw phase/state string extracted from the component
	Phase *string `json:"phase,omitempty"`

	// Conditions are the extracted Kubernetes-style conditions
	Conditions []Condition `json:"conditions,omitempty"`

	// MatchedStatus is the ResourceStatus that was matched based on StatusMappings
	MatchedStatus v1alpha1.ResourceStatus `json:"matchedStatus"`
}

// InstanceSummary represents all extracted data for a single instance
type InstanceSummary struct {
	PodTemplateSpec   *corev1.PodTemplateSpec `json:"podTemplateSpec,omitempty"`
	PodSpec           *corev1.PodSpec         `json:"podSpec,omitempty"`
	FragmentedPodSpec *FragmentedPodSpec      `json:"fragmentedPodSpec,omitempty"`

	Metadata *metav1.ObjectMeta `json:"metadata,omitempty"`

	Scale *Scale `json:"scale,omitempty"`
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
	podTemplateSpecs, err := c.extractor.ExtractPodTemplateSpec(ctx, c.definition)
	if err != nil {
		if isDefinitionNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to extract pod template specs: %w", err)
	}

	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	return zipWithInstanceIds(instanceIds, podTemplateSpecs)
}

// GetFragmentedPodSpec extracts fragmented pod specs mapped by instance id
func (c *Component) GetFragmentedPodSpec(ctx context.Context) (map[string]FragmentedPodSpec, error) {
	fragmentedPodSpecs, err := c.extractor.ExtractFragmentedPodSpec(ctx, c.definition)
	if err != nil {
		if isDefinitionNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to extract fragmented pod specs: %w", err)
	}

	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	return zipWithInstanceIds(instanceIds, fragmentedPodSpecs)
}

// GetPodSpec extracts pod specs mapped by instance id
func (c *Component) GetPodSpec(ctx context.Context) (map[string]corev1.PodSpec, error) {
	podSpecs, err := c.extractor.ExtractPodSpec(ctx, c.definition)
	if err != nil {
		if isDefinitionNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to extract pod specs: %w", err)
	}

	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	return zipWithInstanceIds(instanceIds, podSpecs)
}

// GetPodMetadata extracts pod metadata mapped by instance id
func (c *Component) GetPodMetadata(ctx context.Context) (map[string]metav1.ObjectMeta, error) {
	podMetadata, err := c.extractor.ExtractPodMetadata(ctx, c.definition)
	if err != nil {
		if isDefinitionNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to extract pod metadata: %w", err)
	}

	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	return zipWithInstanceIds(instanceIds, podMetadata)
}

// GetScale extracts scale data mapped by instance id
func (c *Component) GetScale(ctx context.Context) (map[string]Scale, error) {
	scales, err := c.extractor.ExtractScale(ctx, c.definition)
	if err != nil {
		if isDefinitionNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to extract scales: %w", err)
	}

	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	return zipWithInstanceIds(instanceIds, scales)
}

// GetStatus extracts status information from the component
// Note: Status is typically defined only on the root component, not on instances
func (c *Component) GetStatus(ctx context.Context) (*Status, error) {
	status, err := c.extractor.ExtractStatus(ctx, c.definition)
	if err != nil {
		if isDefinitionNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to extract status: %w", err)
	}

	return status, nil
}

// GetInstanceSummaries aggregates all extraction results into a map of instance summaries
func (c *Component) GetInstanceSummaries(ctx context.Context) (map[string]InstanceSummary, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetInstanceIds, err)
	}

	podTemplateSpecs, err := c.GetPodTemplateSpec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod template specs: %w", err)
	}

	podSpecs, err := c.GetPodSpec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod specs: %w", err)
	}

	fragmentedPodSpecs, err := c.GetFragmentedPodSpec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get fragmented pod specs: %w", err)
	}

	metadata, err := c.GetPodMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metadata: %w", err)
	}

	scales, err := c.GetScale(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get scales: %w", err)
	}

	summaries := make(map[string]InstanceSummary, len(instanceIds))
	for _, instanceID := range instanceIds {
		summary := InstanceSummary{}

		if podTemplateSpecs != nil {
			if pts, ok := podTemplateSpecs[instanceID]; ok {
				summary.PodTemplateSpec = &pts
			}
		}

		if podSpecs != nil {
			if ps, ok := podSpecs[instanceID]; ok {
				summary.PodSpec = &ps
			}
		}

		if fragmentedPodSpecs != nil {
			if fps, ok := fragmentedPodSpecs[instanceID]; ok {
				summary.FragmentedPodSpec = &fps
			}
		}

		if metadata != nil {
			if md, ok := metadata[instanceID]; ok {
				summary.Metadata = &md
			}
		}

		if scales != nil {
			if scale, ok := scales[instanceID]; ok {
				summary.Scale = &scale
			}
		}

		summaries[instanceID] = summary
	}

	return summaries, nil
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
		if isDefinitionNotFoundError(err) {
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

func isDefinitionNotFoundError(err error) bool {
	var defNotFoundErr DefinitionNotFoundError
	return errors.As(err, &defNotFoundErr)
}
