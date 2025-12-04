package service

import (
	"errors"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
)

// ValidationService handles RI validation logic
type ValidationService struct{}

// NewValidationService creates a new ValidationService
func NewValidationService() *ValidationService {
	return &ValidationService{}
}

// ValidateRI validates a Resource Interface YAML
func (s *ValidationService) ValidateRI(riYAML string) (bool, []string) {
	if strings.TrimSpace(riYAML) == "" {
		return false, []string{"RI definition is empty"}
	}

	// Parse YAML to RI struct
	var ri v1alpha1.ResourceInterface
	if err := yaml.Unmarshal([]byte(riYAML), &ri); err != nil {
		return false, []string{fmt.Sprintf("Failed to parse RI YAML: %v", err)}
	}

	// Validate using the existing validator
	validator := v1alpha1.NewRIValidator(&ri)
	if err := validator.Validate(); err != nil {
		return false, s.formatValidationErrors(err)
	}

	return true, nil
}

// formatValidationErrors converts validation errors into user-friendly messages
func (s *ValidationService) formatValidationErrors(err error) []string {
	if err == nil {
		return nil
	}

	// Handle multiple errors joined together
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




