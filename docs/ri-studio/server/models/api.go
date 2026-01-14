package models

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/resource"
)

// ValidateRequest represents a request to validate an RI definition
type ValidateRequest struct {
	RI string `json:"ri"` // YAML string
}

// ValidateResponse represents the validation result
type ValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// ExtractRequest represents a request to extract information from a CR using an RI
type ExtractRequest struct {
	CR string `json:"cr"` // YAML string of the Custom Resource
	RI string `json:"ri"` // YAML string of the Resource Interface
}

// ExtractResponse represents the extraction result
type ExtractResponse struct {
	Success    bool              `json:"success"`
	Errors     []string          `json:"errors,omitempty"`
	Components []ComponentResult `json:"components,omitempty"`
}

// ComponentResult represents the extracted information for a single component
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

// ExamplesListResponse represents the list of available examples
type ExamplesListResponse struct {
	Examples []ExampleInfo `json:"examples"`
}

// ExampleInfo contains metadata about an example
type ExampleInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

// ExampleResponse represents a full example with both CR and RI
type ExampleResponse struct {
	Name string `json:"name"`
	CR   string `json:"cr,omitempty"`   // YAML string
	RI   string `json:"ri"`             // YAML string
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

