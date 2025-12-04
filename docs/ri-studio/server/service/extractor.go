package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/kai-bolt/docs/ri-studio/server/models"
	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/query"
	"github.com/run-ai/kai-bolt/pkg/resource"
)

// ExtractorService handles extraction of information from CRs using RIs
type ExtractorService struct{}

// NewExtractorService creates a new ExtractorService
func NewExtractorService() *ExtractorService {
	return &ExtractorService{}
}

// Extract extracts information from a CR using an RI definition
func (s *ExtractorService) Extract(ctx context.Context, crYAML, riYAML string) (*models.ExtractResponse, error) {
	// Validate inputs
	if strings.TrimSpace(crYAML) == "" {
		return &models.ExtractResponse{
			Success: false,
			Errors:  []string{"Custom Resource YAML is empty"},
		}, nil
	}

	if strings.TrimSpace(riYAML) == "" {
		return &models.ExtractResponse{
			Success: false,
			Errors:  []string{"Resource Interface YAML is empty"},
		}, nil
	}

	// Parse RI YAML
	var ri v1alpha1.ResourceInterface
	if err := yaml.Unmarshal([]byte(riYAML), &ri); err != nil {
		return &models.ExtractResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("Failed to parse RI YAML: %v", err)},
		}, nil
	}

	// Parse CR YAML to a generic map
	var crData map[string]interface{}
	if err := yaml.Unmarshal([]byte(crYAML), &crData); err != nil {
		return &models.ExtractResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("Failed to parse CR YAML: %v", err)},
		}, nil
	}

	// Extract information from all components
	var componentResults []models.ComponentResult
	var extractionErrors []string

	// Process root component
	rootResult := s.extractFromComponent(ctx, crData, ri.Spec.StructureDefinition.RootComponent)
	if rootResult.Error != "" {
		extractionErrors = append(extractionErrors, fmt.Sprintf("Root component '%s': %s", rootResult.Name, rootResult.Error))
	}
	componentResults = append(componentResults, rootResult)

	// Process child components
	for _, childDef := range ri.Spec.StructureDefinition.ChildComponents {
		childResult := s.extractFromComponent(ctx, crData, childDef)
		if childResult.Error != "" {
			extractionErrors = append(extractionErrors, fmt.Sprintf("Child component '%s': %s", childResult.Name, childResult.Error))
		}
		componentResults = append(componentResults, childResult)
	}

	return &models.ExtractResponse{
		Success:    len(extractionErrors) == 0,
		Errors:     extractionErrors,
		Components: componentResults,
	}, nil
}

// extractFromComponent extracts information from a single component
func (s *ExtractorService) extractFromComponent(ctx context.Context, crData map[string]interface{}, componentDef v1alpha1.ComponentDefinition) models.ComponentResult {
	result := models.ComponentResult{
		Name: componentDef.Name,
		Kind: componentDef.Kind,
	}

	// Create JQ evaluator for the CR data
	evaluator := query.NewDefaultJqEvaluator(crData)
	extractor := resource.NewInterfaceExtractor(evaluator)

	// Create a component wrapper
	comp := s.createComponent(componentDef, extractor)

	// Extract instance IDs
	instanceIds, err := comp.GetInstanceIds(ctx)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to extract instance IDs: %v", err)
		return result
	}
	result.InstanceIds = instanceIds

	// Try to extract pod template spec
	if componentDef.SpecDefinition != nil && componentDef.SpecDefinition.PodTemplateSpecPath != nil {
		podTemplateSpecs, err := comp.GetPodTemplateSpec(ctx)
		if err == nil && len(podTemplateSpecs) > 0 {
			result.PodTemplateSpec = podTemplateSpecs
		} else if err != nil {
			// Check if it's just a "not found" error
			var defNotFoundErr resource.DefinitionNotFoundError
			if !errors.As(err, &defNotFoundErr) {
				result.Error = appendError(result.Error, fmt.Sprintf("Pod template spec extraction error: %v", err))
			}
		}
	}

	// Try to extract pod spec
	if componentDef.SpecDefinition != nil && componentDef.SpecDefinition.PodSpecPath != nil {
		podSpecs, err := comp.GetPodSpec(ctx)
		if err == nil && len(podSpecs) > 0 {
			result.PodSpec = podSpecs
		} else if err != nil {
			var defNotFoundErr resource.DefinitionNotFoundError
			if !errors.As(err, &defNotFoundErr) {
				result.Error = appendError(result.Error, fmt.Sprintf("Pod spec extraction error: %v", err))
			}
		}

		// Also try to extract pod metadata if metadata path is defined
		if componentDef.SpecDefinition.MetadataPath != nil {
			podMetadata, err := comp.GetPodMetadata(ctx)
			if err == nil && len(podMetadata) > 0 {
				result.PodMetadata = podMetadata
			} else if err != nil {
				var defNotFoundErr resource.DefinitionNotFoundError
				if !errors.As(err, &defNotFoundErr) {
					result.Error = appendError(result.Error, fmt.Sprintf("Pod metadata extraction error: %v", err))
				}
			}
		}
	}

	// Try to extract fragmented pod spec
	if componentDef.SpecDefinition != nil && componentDef.SpecDefinition.FragmentedPodSpecDefinition != nil {
		fragmentedSpecs, err := comp.GetFragmentedPodSpec(ctx)
		if err == nil && len(fragmentedSpecs) > 0 {
			result.FragmentedSpec = fragmentedSpecs
		} else if err != nil {
			var defNotFoundErr resource.DefinitionNotFoundError
			if !errors.As(err, &defNotFoundErr) {
				result.Error = appendError(result.Error, fmt.Sprintf("Fragmented pod spec extraction error: %v", err))
			}
		}
	}

	// Try to extract scale information
	if componentDef.ScaleDefinition != nil {
		scales, err := comp.GetScale(ctx)
		if err == nil && len(scales) > 0 {
			result.Scale = scales
		} else if err != nil {
			var defNotFoundErr resource.DefinitionNotFoundError
			if !errors.As(err, &defNotFoundErr) {
				result.Error = appendError(result.Error, fmt.Sprintf("Scale extraction error: %v", err))
			}
		}
	}

	return result
}

// createComponent creates a component wrapper (mimics the private Component struct)
func (s *ExtractorService) createComponent(def v1alpha1.ComponentDefinition, extractor resource.Extractor) *componentWrapper {
	return &componentWrapper{
		definition: def,
		extractor:  extractor,
	}
}

// componentWrapper wraps a component definition with an extractor
type componentWrapper struct {
	definition v1alpha1.ComponentDefinition
	extractor  resource.Extractor
}

func (c *componentWrapper) GetInstanceIds(ctx context.Context) ([]string, error) {
	instanceIds, err := c.extractor.ExtractInstanceIds(ctx, c.definition)
	if err != nil {
		var defNotFoundErr resource.DefinitionNotFoundError
		if errors.As(err, &defNotFoundErr) {
			return []string{""}, nil
		}
		return nil, err
	}
	return instanceIds, nil
}

func (c *componentWrapper) GetPodTemplateSpec(ctx context.Context) (map[string]corev1.PodTemplateSpec, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, err
	}

	podTemplateSpecs, err := c.extractor.ExtractPodTemplateSpec(ctx, c.definition)
	if err != nil {
		return nil, err
	}

	result := make(map[string]corev1.PodTemplateSpec)
	for i, id := range instanceIds {
		if i < len(podTemplateSpecs) {
			result[id] = podTemplateSpecs[i]
		}
	}
	return result, nil
}

func (c *componentWrapper) GetPodSpec(ctx context.Context) (map[string]corev1.PodSpec, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, err
	}

	podSpecs, err := c.extractor.ExtractPodSpec(ctx, c.definition)
	if err != nil {
		return nil, err
	}

	result := make(map[string]corev1.PodSpec)
	for i, id := range instanceIds {
		if i < len(podSpecs) {
			result[id] = podSpecs[i]
		}
	}
	return result, nil
}

func (c *componentWrapper) GetPodMetadata(ctx context.Context) (map[string]metav1.ObjectMeta, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, err
	}

	podMetadata, err := c.extractor.ExtractPodMetadata(ctx, c.definition)
	if err != nil {
		return nil, err
	}

	result := make(map[string]metav1.ObjectMeta)
	for i, id := range instanceIds {
		if i < len(podMetadata) {
			result[id] = podMetadata[i]
		}
	}
	return result, nil
}

func (c *componentWrapper) GetFragmentedPodSpec(ctx context.Context) (map[string]resource.FragmentedPodSpec, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, err
	}

	fragmentedSpecs, err := c.extractor.ExtractFragmentedPodSpec(ctx, c.definition)
	if err != nil {
		return nil, err
	}

	result := make(map[string]resource.FragmentedPodSpec)
	for i, id := range instanceIds {
		if i < len(fragmentedSpecs) {
			result[id] = fragmentedSpecs[i]
		}
	}
	return result, nil
}

func (c *componentWrapper) GetScale(ctx context.Context) (map[string]resource.Scale, error) {
	instanceIds, err := c.GetInstanceIds(ctx)
	if err != nil {
		return nil, err
	}

	scales, err := c.extractor.ExtractScale(ctx, c.definition)
	if err != nil {
		return nil, err
	}

	result := make(map[string]resource.Scale)
	for i, id := range instanceIds {
		if i < len(scales) {
			result[id] = scales[i]
		}
	}
	return result, nil
}

// appendError appends an error message to an existing error string
func appendError(existing, new string) string {
	if existing == "" {
		return new
	}
	return existing + "; " + new
}

