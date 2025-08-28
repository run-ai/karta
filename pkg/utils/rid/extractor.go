package rid

import (
	"encoding/json"
	"fmt"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QueryEvaluator interface for query evaluation against data
type QueryEvaluator interface {
	Evaluate(expression string) ([]interface{}, error)
}

// RidExtractor implements extraction using QueryEvaluator
type RidExtractor struct {
	queryEvaluator QueryEvaluator
}

func NewRidExtractor(queryEvaluator QueryEvaluator) *RidExtractor {
	return &RidExtractor{
		queryEvaluator: queryEvaluator,
	}
}

func (e *RidExtractor) ExtractPodTemplateSpec(definition v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, fmt.Errorf("component %s does not have spec definition", definition.Name)
	}

	if definition.SpecDefinition.PodTemplateSpecPath == nil {
		return nil, fmt.Errorf("component %s does not have pod template spec definition", definition.Name)
	}

	var podTemplateSpec []corev1.PodTemplateSpec
	err := extract(definition.SpecDefinition.PodTemplateSpecPath, e.queryEvaluator, &podTemplateSpec)

	return podTemplateSpec, err
}

func (e *RidExtractor) ExtractPodSpec(definition v1alpha1.ComponentDefinition) ([]corev1.PodSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, fmt.Errorf("component %s does not have spec definition", definition.Name)
	}

	if definition.SpecDefinition.PodSpecPath == nil {
		return nil, fmt.Errorf("component %s does not have pod spec definition", definition.Name)
	}

	var podSpec []corev1.PodSpec
	err := extract(definition.SpecDefinition.PodSpecPath, e.queryEvaluator, &podSpec)

	return podSpec, err
}

func (e *RidExtractor) ExtractPodMetadata(definition v1alpha1.ComponentDefinition) ([]metav1.ObjectMeta, error) {
	if definition.SpecDefinition == nil {
		return nil, fmt.Errorf("component %s does not have spec definition", definition.Name)
	}

	if definition.SpecDefinition.MetadataPath == nil {
		return nil, fmt.Errorf("component %s does not have pod metadata definition", definition.Name)
	}

	var podMetadata []metav1.ObjectMeta
	err := extract(definition.SpecDefinition.MetadataPath, e.queryEvaluator, &podMetadata)

	return podMetadata, err
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

func extract[T any](path *string, evaluator QueryEvaluator, out *[]T) error {
	if path == nil {
		return nil
	}

	results, err := evaluator.Evaluate(*path)
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

func (e *RidExtractor) ExtractFragmentedPodSpec(definition v1alpha1.ComponentDefinition) ([]FragmentedPodSpec, error) {
	if definition.SpecDefinition == nil {
		return nil, fmt.Errorf("component %s does not have spec definition", definition.Name)
	}

	if definition.SpecDefinition.FragmentedPodSpecDefinition == nil {
		return nil, fmt.Errorf("component %s does not have fragmented spec definition", definition.Name)
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
		priorityClassNameResults []string
		imageResults             []string
	)

	// Parallel execution - commented out due to gojq concurrency issues
	// var g errgroup.Group
	// g.Go(func() error {
	// 	return extract(fragmentedDefinition.SchedulerNamePath, e.queryEvaluator, &schedulerNameResults)
	// })
	// ... (other g.Go calls)
	// if err := g.Wait(); err != nil {
	// 	return nil, err
	// }

	// Sequential execution for now
	if err := extract(fragmentedDefinition.SchedulerNamePath, e.queryEvaluator, &schedulerNameResults); err != nil {
		return nil, err
	}

	if err := extract(fragmentedDefinition.LabelsPath, e.queryEvaluator, &labelsResults); err != nil {
		return nil, err
	}

	if err := extract(fragmentedDefinition.AnnotationsPath, e.queryEvaluator, &annotationsResults); err != nil {
		return nil, err
	}

	if err := extract(fragmentedDefinition.ResourcesPath, e.queryEvaluator, &resourcesResults); err != nil {
		return nil, err
	}

	if err := extract(fragmentedDefinition.ResourceClaimsPath, e.queryEvaluator, &resourceClaimsResults); err != nil {
		return nil, err
	}

	if err := extract(fragmentedDefinition.PodAffinityPath, e.queryEvaluator, &podAffinityResults); err != nil {
		return nil, err
	}

	if err := extract(fragmentedDefinition.NodeAffinityPath, e.queryEvaluator, &nodeAffinityResults); err != nil {
		return nil, err
	}

	if err := extract(fragmentedDefinition.ContainersPath, e.queryEvaluator, &containersResults); err != nil {
		return nil, err
	}

	if err := extract(fragmentedDefinition.PriorityClassNamePath, e.queryEvaluator, &priorityClassNameResults); err != nil {
		return nil, err
	}

	if err := extract(fragmentedDefinition.ImagePath, e.queryEvaluator, &imageResults); err != nil {
		return nil, err
	}

	specCount := len(schedulerNameResults)
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
			PriorityClassName: safeGetByIndex(priorityClassNameResults, i),
			Image:             safeGetByIndex(imageResults, i),
		}
	}

	return fragmentedSpecs, nil
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
func safeConvertSlice[T any](slice []interface{}) ([]T, error) {
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
func convertViaJSON(src interface{}, dst interface{}) error {
	jsonBytes, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal source: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, dst); err != nil {
		return fmt.Errorf("failed to unmarshal to destination: %w", err)
	}

	return nil
}
