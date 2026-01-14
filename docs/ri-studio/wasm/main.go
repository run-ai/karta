//go:build js && wasm
// +build js,wasm

package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"syscall/js"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/jq/execution"
	"github.com/run-ai/kai-bolt/pkg/resource"
)

//go:embed examples/*.yaml
var examplesFS embed.FS

// Response types matching the server models
type ValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

type ExtractResponse struct {
	Success    bool              `json:"success"`
	Errors     []string          `json:"errors,omitempty"`
	Components []ComponentResult `json:"components,omitempty"`
}

type ComponentResult struct {
	Name            string                                `json:"name"`
	Kind            *v1alpha1.GroupVersionKind            `json:"kind,omitempty"`
	OwnerRef        *string                               `json:"ownerRef,omitempty"`
	PodTemplateSpec map[string]corev1.PodTemplateSpec     `json:"podTemplateSpec,omitempty"`
	PodSpec         map[string]corev1.PodSpec             `json:"podSpec,omitempty"`
	PodMetadata     map[string]metav1.ObjectMeta          `json:"podMetadata,omitempty"`
	FragmentedSpec  map[string]resource.FragmentedPodSpec `json:"fragmentedSpec,omitempty"`
	Scale           map[string]resource.Scale             `json:"scale,omitempty"`
	InstanceIds     []string                              `json:"instanceIds,omitempty"`
	Error           string                                `json:"error,omitempty"`
}

type ExampleInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

type ExamplesListResponse struct {
	Examples []ExampleInfo `json:"examples"`
}

type ExampleResponse struct {
	Name string `json:"name"`
	RI   string `json:"ri"`
}

func main() {
	c := make(chan struct{})

	// Register WASM functions
	js.Global().Set("validateRI", js.FuncOf(validateRIWrapper))
	js.Global().Set("extractData", js.FuncOf(extractDataWrapper))
	js.Global().Set("listExamples", js.FuncOf(listExamplesWrapper))
	js.Global().Set("getExample", js.FuncOf(getExampleWrapper))

	// Signal that WASM is ready
	js.Global().Call("postMessage", map[string]interface{}{"type": "wasmReady"})

	<-c
}

// validateRIWrapper is the JavaScript-callable wrapper for RI validation
func validateRIWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorResponse("RI YAML argument is required")
	}

	riYAML := args[0].String()

	response := validateRI(riYAML)

	return jsValue(response)
}

// validateRI validates a Resource Interface YAML
func validateRI(riYAML string) ValidateResponse {
	if strings.TrimSpace(riYAML) == "" {
		return ValidateResponse{
			Valid:  false,
			Errors: []string{"RI definition is empty"},
		}
	}

	// Parse YAML to RI struct
	var ri v1alpha1.ResourceInterface
	if err := yaml.Unmarshal([]byte(riYAML), &ri); err != nil {
		return ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Failed to parse RI YAML: %v", err)},
		}
	}

	// Validate using the existing validator
	validator := v1alpha1.NewRIValidator(&ri)
	if err := validator.Validate(); err != nil {
		return ValidateResponse{
			Valid:  false,
			Errors: formatValidationErrors(err),
		}
	}

	return ValidateResponse{Valid: true}
}

// extractDataWrapper is the JavaScript-callable wrapper for data extraction
func extractDataWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return errorResponse("CR YAML and RI YAML arguments are required")
	}

	crYAML := args[0].String()
	riYAML := args[1].String()

	response := extractData(crYAML, riYAML)

	return jsValue(response)
}

// extractData extracts information from a CR using an RI definition
func extractData(crYAML, riYAML string) ExtractResponse {
	ctx := context.Background()

	// Validate inputs
	if strings.TrimSpace(crYAML) == "" {
		return ExtractResponse{
			Success: false,
			Errors:  []string{"Custom Resource YAML is empty"},
		}
	}

	if strings.TrimSpace(riYAML) == "" {
		return ExtractResponse{
			Success: false,
			Errors:  []string{"Resource Interface YAML is empty"},
		}
	}

	// Parse RI YAML
	var ri v1alpha1.ResourceInterface
	if err := yaml.Unmarshal([]byte(riYAML), &ri); err != nil {
		return ExtractResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("Failed to parse RI YAML: %v", err)},
		}
	}

	// Parse CR YAML to a generic map
	var crData map[string]interface{}
	if err := yaml.Unmarshal([]byte(crYAML), &crData); err != nil {
		return ExtractResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("Failed to parse CR YAML: %v", err)},
		}
	}

	// Extract information from all components
	var componentResults []ComponentResult
	var extractionErrors []string

	// Process root component
	rootResult := extractFromComponent(ctx, crData, ri.Spec.StructureDefinition.RootComponent)
	if rootResult.Error != "" {
		extractionErrors = append(extractionErrors, fmt.Sprintf("Root component '%s': %s", rootResult.Name, rootResult.Error))
	}
	componentResults = append(componentResults, rootResult)

	// Process child components
	for _, childDef := range ri.Spec.StructureDefinition.ChildComponents {
		childResult := extractFromComponent(ctx, crData, childDef)
		if childResult.Error != "" {
			extractionErrors = append(extractionErrors, fmt.Sprintf("Child component '%s': %s", childResult.Name, childResult.Error))
		}
		componentResults = append(componentResults, childResult)
	}

	return ExtractResponse{
		Success:    len(extractionErrors) == 0,
		Errors:     extractionErrors,
		Components: componentResults,
	}
}

// extractFromComponent extracts information from a single component
func extractFromComponent(ctx context.Context, crData map[string]interface{}, componentDef v1alpha1.ComponentDefinition) ComponentResult {
	result := ComponentResult{
		Name:     componentDef.Name,
		Kind:     componentDef.Kind,
		OwnerRef: componentDef.OwnerRef,
	}

	// Create JQ evaluator for the CR data
	jqRunner := execution.NewDefaultRunner(crData)
	accessor := resource.NewAccessor(jqRunner)

	// Create a component wrapper
	comp := createComponent(componentDef, accessor)

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

// createComponent creates a component wrapper
func createComponent(def v1alpha1.ComponentDefinition, accessor resource.ComponentAccessor) *componentWrapper {
	return &componentWrapper{
		definition: def,
		accessor:   accessor,
	}
}

// componentWrapper wraps a component definition with an extractor
type componentWrapper struct {
	definition v1alpha1.ComponentDefinition
	accessor   resource.ComponentAccessor
}

func (c *componentWrapper) GetInstanceIds(ctx context.Context) ([]string, error) {
	instanceIds, err := c.accessor.ExtractInstanceIds(ctx, c.definition)
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

	podTemplateSpecs, err := c.accessor.ExtractPodTemplateSpec(ctx, c.definition)
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

	podSpecs, err := c.accessor.ExtractPodSpec(ctx, c.definition)
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

	podMetadata, err := c.accessor.ExtractPodMetadata(ctx, c.definition)
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

	fragmentedSpecs, err := c.accessor.ExtractFragmentedPodSpec(ctx, c.definition)
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

	scales, err := c.accessor.ExtractScale(ctx, c.definition)
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

// listExamplesWrapper is the JavaScript-callable wrapper for listing examples
func listExamplesWrapper(this js.Value, args []js.Value) interface{} {
	response := listExamples()
	return jsValue(response)
}

// listExamples lists all available examples
func listExamples() ExamplesListResponse {
	entries, err := examplesFS.ReadDir("examples")
	if err != nil {
		return ExamplesListResponse{Examples: []ExampleInfo{}}
	}

	var examples []ExampleInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml")
		examples = append(examples, ExampleInfo{
			Name:        name,
			DisplayName: formatDisplayName(name),
			Description: fmt.Sprintf("Example RI for %s", formatDisplayName(name)),
		})
	}

	return ExamplesListResponse{Examples: examples}
}

// getExampleWrapper is the JavaScript-callable wrapper for getting an example
func getExampleWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorResponse("Example name argument is required")
	}

	name := args[0].String()
	response, err := getExample(name)
	if err != nil {
		return errorResponse(err.Error())
	}

	return jsValue(response)
}

// getExample loads a specific example
func getExample(name string) (*ExampleResponse, error) {
	// Sanitize the name
	name = strings.TrimSuffix(name, ".yaml")

	riContent, err := examplesFS.ReadFile(fmt.Sprintf("examples/%s.yaml", name))
	if err != nil {
		return nil, fmt.Errorf("example '%s' not found: %v", name, err)
	}

	return &ExampleResponse{
		Name: name,
		RI:   string(riContent),
	}, nil
}

// Helper functions

// formatValidationErrors converts validation errors into user-friendly messages
func formatValidationErrors(err error) []string {
	if err == nil {
		return nil
	}

	var errorMessages []string

	// Try to unwrap joined errors
	var joinedErr interface{ Unwrap() []error }
	if errors.As(err, &joinedErr) {
		unwrapped := joinedErr.Unwrap()
		for _, e := range unwrapped {
			errorMessages = append(errorMessages, e.Error())
		}
	} else {
		// Single error
		errorMessages = append(errorMessages, err.Error())
	}

	return errorMessages
}

// formatDisplayName converts a filename to a display name
func formatDisplayName(name string) string {
	// Convert kebab-case or snake_case to Title Case
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return strings.Join(words, " ")
}

// appendError appends an error message to an existing error string
func appendError(existing, new string) string {
	if existing == "" {
		return new
	}
	return existing + "; " + new
}

// jsValue converts a Go value to a JavaScript value
func jsValue(v interface{}) interface{} {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to serialize response: %v", err))
	}

	var result interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return errorResponse(fmt.Sprintf("Failed to deserialize response: %v", err))
	}

	return js.ValueOf(result)
}

// errorResponse creates an error response object
func errorResponse(message string) interface{} {
	return js.ValueOf(map[string]interface{}{
		"error": message,
	})
}
