package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/query"
)

// DefinitionNotFoundError represents an error when a requested definition is not found
type DefinitionNotFoundError string

func (e DefinitionNotFoundError) Error() string {
	return string(e)
}

// InterfaceExtractor implements extraction using QueryEvaluator
type InterfaceExtractor struct {
	queryEvaluator query.QueryEvaluator
}

func NewInterfaceExtractor(queryEvaluator query.QueryEvaluator) *InterfaceExtractor {
	return &InterfaceExtractor{
		queryEvaluator: queryEvaluator,
	}
}

func (e *InterfaceExtractor) ExtractPodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.PodTemplateSpecPath == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have pod template spec definition", definition.Name))
	}

	var podTemplateSpec []corev1.PodTemplateSpec
	err := extract(ctx, definition.SpecDefinition.PodTemplateSpecPath, e.queryEvaluator, &podTemplateSpec)

	return podTemplateSpec, err
}

func (e *InterfaceExtractor) ExtractPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.PodSpecPath == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have pod spec definition", definition.Name))
	}

	var podSpec []corev1.PodSpec
	err := extract(ctx, definition.SpecDefinition.PodSpecPath, e.queryEvaluator, &podSpec)

	return podSpec, err
}

func (e *InterfaceExtractor) ExtractPodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]metav1.ObjectMeta, error) {
	if definition.SpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.MetadataPath == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have pod metadata definition", definition.Name))
	}

	var podMetadata []metav1.ObjectMeta
	err := extract(ctx, definition.SpecDefinition.MetadataPath, e.queryEvaluator, &podMetadata)

	return podMetadata, err
}

func (e *InterfaceExtractor) ExtractScale(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]Scale, error) {
	if definition.ScaleDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have scale definition", definition.Name))
	}

	var (
		replicas    []*int32
		minReplicas []*int32
		maxReplicas []*int32
	)

	scaleCount := 0

	if err := extract(ctx, definition.ScaleDefinition.ReplicasPath, e.queryEvaluator, &replicas); err != nil {
		return nil, err
	}
	scaleCount = max(scaleCount, len(replicas))

	if err := extract(ctx, definition.ScaleDefinition.MinReplicasPath, e.queryEvaluator, &minReplicas); err != nil {
		return nil, err
	}
	scaleCount = max(scaleCount, len(minReplicas))

	if err := extract(ctx, definition.ScaleDefinition.MaxReplicasPath, e.queryEvaluator, &maxReplicas); err != nil {
		return nil, err
	}
	scaleCount = max(scaleCount, len(maxReplicas))

	scales := make([]Scale, scaleCount)
	for i := 0; i < scaleCount; i++ {
		scales[i] = Scale{
			Replicas:    safeGetByIndex(replicas, i),
			MaxReplicas: safeGetByIndex(maxReplicas, i),
			MinReplicas: safeGetByIndex(minReplicas, i),
		}
	}

	return scales, nil
}

func extract[T any](ctx context.Context, path *string, evaluator query.QueryEvaluator, out *[]T) error {
	if path == nil {
		return nil
	}

	results, err := evaluator.Evaluate(ctx, *path)
	if err != nil {
		return err
	}

	converted, err := safeConvertSlice[T](results)
	if err != nil {
		return err
	}

	*out = converted
	return nil
}

func (e *InterfaceExtractor) ExtractFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]FragmentedPodSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have spec definition", definition.Name))
	}

	if definition.SpecDefinition.FragmentedPodSpecDefinition == nil {
		return nil, DefinitionNotFoundError(fmt.Sprintf("component %s does not have fragmented pod spec definition", definition.Name))
	}

	fragmentedDefinition := definition.SpecDefinition.FragmentedPodSpecDefinition

	var (
		schedulerNameResults     []string
		labelsResults            []map[string]string
		annotationsResults       []map[string]string
		resourcesResults         []corev1.ResourceRequirements
		resourceClaimsResults    [][]corev1.PodResourceClaim
		podAffinityResults       []*corev1.PodAffinity
		nodeAffinityResults      []*corev1.NodeAffinity
		containersResults        [][]corev1.Container
		containerResults         []corev1.Container
		priorityClassNameResults []string
		imageResults             []string
	)

	specCount := 0

	if err := extract(ctx, fragmentedDefinition.SchedulerNamePath, e.queryEvaluator, &schedulerNameResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(schedulerNameResults))

	if err := extract(ctx, fragmentedDefinition.LabelsPath, e.queryEvaluator, &labelsResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(labelsResults))

	if err := extract(ctx, fragmentedDefinition.AnnotationsPath, e.queryEvaluator, &annotationsResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(annotationsResults))

	if err := extract(ctx, fragmentedDefinition.ResourcesPath, e.queryEvaluator, &resourcesResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(resourcesResults))

	if err := extract(ctx, fragmentedDefinition.ResourceClaimsPath, e.queryEvaluator, &resourceClaimsResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(resourceClaimsResults))

	if err := extract(ctx, fragmentedDefinition.PodAffinityPath, e.queryEvaluator, &podAffinityResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(podAffinityResults))

	if err := extract(ctx, fragmentedDefinition.NodeAffinityPath, e.queryEvaluator, &nodeAffinityResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(nodeAffinityResults))

	if err := extract(ctx, fragmentedDefinition.ContainersPath, e.queryEvaluator, &containersResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(containersResults))

	if err := extract(ctx, fragmentedDefinition.ContainerPath, e.queryEvaluator, &containerResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(containerResults))

	if err := extract(ctx, fragmentedDefinition.PriorityClassNamePath, e.queryEvaluator, &priorityClassNameResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(priorityClassNameResults))

	if err := extract(ctx, fragmentedDefinition.ImagePath, e.queryEvaluator, &imageResults); err != nil {
		return nil, err
	}
	specCount = max(specCount, len(imageResults))

	fragmentedSpecs := make([]FragmentedPodSpec, specCount)
	for i := 0; i < specCount; i++ {
		fragmentedSpecs[i] = FragmentedPodSpec{
			SchedulerName:     safeGetByIndex(schedulerNameResults, i),
			Labels:            safeGetByIndex(labelsResults, i),
			Annotations:       safeGetByIndex(annotationsResults, i),
			Resources:         safeGetByIndex(resourcesResults, i),
			ResourceClaims:    safeGetByIndex(resourceClaimsResults, i),
			PodAffinity:       safeGetByIndex(podAffinityResults, i),
			NodeAffinity:      safeGetByIndex(nodeAffinityResults, i),
			Containers:        safeGetByIndex(containersResults, i),
			Container:         safeGetByIndex(containerResults, i),
			PriorityClassName: safeGetByIndex(priorityClassNameResults, i),
			Image:             safeGetByIndex(imageResults, i),
		}
	}

	return fragmentedSpecs, nil
}

func (e *InterfaceExtractor) ExtractInstanceIds(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]string, error) {
	if definition.InstanceIdPath == nil {
		return nil, DefinitionNotFoundError("no instance id path defined")
	}

	var instanceIds []string
	err := extract(ctx, definition.InstanceIdPath, e.queryEvaluator, &instanceIds)
	if err != nil {
		return nil, err
	}

	// Validate all instance ids are not empty
	if lo.Contains(instanceIds, "") {
		return nil, fmt.Errorf("instance id path contained empty string values [%s]", strings.Join(instanceIds, ","))
	}

	return instanceIds, nil
}

// safeGetByIndex Generic function for safely retrieving a slice element.
// Returns zero value if slice is nil or index is out of range
func safeGetByIndex[T any](slice []T, index int) T {
	var zero T

	if slice == nil {
		return zero
	}

	if index < 0 || index >= len(slice) {
		return zero
	}

	return slice[index]
}

// safeConvertSlice Generic type conversion for slice objects
func safeConvertSlice[T any](slice []any) ([]T, error) {
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

		// For complex types (structs), use JSON marshaling/unmarshaling
		var converted T
		if err := convertViaJSON(object, &converted); err != nil {
			return nil, fmt.Errorf("failed to convert object at index %d to type %T: %w", i, converted, err)
		}
		convertedResults[i] = converted
	}

	return convertedResults, nil
}

// convertViaJSON converts between types using JSON marshaling
func convertViaJSON(src any, dst any) error {
	jsonBytes, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal source: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, dst); err != nil {
		return fmt.Errorf("failed to unmarshal to destination: %w", err)
	}

	return nil
}
