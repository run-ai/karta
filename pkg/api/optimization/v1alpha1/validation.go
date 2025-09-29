package v1alpha1

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/itchyny/gojq"
)

type RIValidator struct {
	RI            *ResourceInterface
	rootComponent ComponentDefinition
	allComponents map[string]ComponentDefinition
}

func (v *RIValidator) Validate() error {
	if v.RI == nil {
		return errors.New("resource interface is nil")
	}

	var errs []error

	if initErrs := v.initialize(); initErrs != nil {
		errs = append(errs, initErrs...)
	}

	if specErrs := v.validateStructureDefinition(); specErrs != nil {
		errs = append(errs, specErrs...)
	}

	if instructionErrs := v.validateInstructions(); instructionErrs != nil {
		errs = append(errs, instructionErrs...)
	}

	if jqErrs := v.validateJQ(); jqErrs != nil {
		errs = append(errs, jqErrs...)
	}

	return errors.Join(errs...)
}

func (v *RIValidator) initialize() []error {
	var errs []error

	v.rootComponent = v.RI.Spec.StructureDefinition.RootComponent
	v.allComponents = make(map[string]ComponentDefinition)
	for _, component := range append(v.RI.Spec.StructureDefinition.ChildComponents, v.RI.Spec.StructureDefinition.RootComponent) {
		if _, ok := v.allComponents[component.Name]; ok {
			errs = append(errs, fmt.Errorf("component name %s is not unique", component.Name))
		}
		v.allComponents[component.Name] = component
	}

	return errs
}

func (v *RIValidator) validateStructureDefinition() []error {
	var errs []error

	// Root component validation
	errs = append(errs, v.validateRootComponent()...)

	// Child components validation
	for _, component := range v.RI.Spec.StructureDefinition.ChildComponents {
		// Must have non-empty owner ref
		if component.OwnerRef == nil || *component.OwnerRef == "" {
			errs = append(errs, fmt.Errorf("child component \"%s\" has no owner ref", component.Name))
		}

		// Owner ref must point to an existing component
		if _, ok := v.allComponents[*component.OwnerRef]; !ok {
			errs = append(errs, fmt.Errorf("child component \"%s\" has owner ref to non-existing component \"%s\"", component.Name, *component.OwnerRef))
		}

		// Component validation
		errs = append(errs, v.validateComponent(component)...)
	}

	return errs
}

func (v *RIValidator) validateRootComponent() []error {
	var errs []error

	// Has full gvk
	if v.rootComponent.Kind == nil ||
		v.rootComponent.Kind.Group == "" || v.rootComponent.Kind.Version == "" || v.rootComponent.Kind.Kind == "" {
		errs = append(errs, fmt.Errorf("root component must have full gvk"))
	}

	// No owner ref
	if v.rootComponent.OwnerRef != nil {
		errs = append(errs, fmt.Errorf("root component cannot have owner ref"))
	}

	// Has status definition
	if v.rootComponent.StatusDefinition == nil {
		errs = append(errs, fmt.Errorf("root component must have status definition"))
	}

	// Component validation
	errs = append(errs, v.validateComponent(v.rootComponent)...)

	return errs
}

func (v *RIValidator) validateComponent(component ComponentDefinition) []error {
	var errs []error

	// Non-empty name
	if component.Name == "" {
		errs = append(errs, fmt.Errorf("component name is empty"))
	}

	// Mutually exclusive pod spec definitions
	if component.SpecDefinition != nil {
		counter := 0

		if component.SpecDefinition.PodTemplateSpecPath != nil {
			counter++
		}
		if component.SpecDefinition.PodSpecPath != nil {
			counter++
		}
		if component.SpecDefinition.FragmentedPodSpecDefinition != nil {
			counter++
		}

		if counter > 1 {
			errs = append(errs, fmt.Errorf("component \"%s\" has multiple pod spec definitions", component.Name))
		}
	}

	// Component's PodSelector has instance selector if has the component has instance id path defined
	if component.InstanceIdPath != nil &&
		(component.PodSelector == nil || component.PodSelector.ComponentInstanceSelector == nil) {
		errs = append(errs, fmt.Errorf("component \"%s\" has instance id path but no pod component instance selector", component.Name))
	}

	return errs
}

func (v *RIValidator) validateInstructions() []error {
	return v.validateGangScheduling()
}

func (v *RIValidator) validateGangScheduling() []error {
	if v.RI.Spec.Instructions.GangScheduling == nil {
		return nil
	}

	// All member components are defined
	var errs []error
	for _, group := range v.RI.Spec.Instructions.GangScheduling.PodGroups {
		for _, member := range group.Members {
			if _, ok := v.allComponents[member.ComponentName]; !ok {
				errs = append(errs, fmt.Errorf("pod-group member component \"%s\" is not defined", member.ComponentName))
			}
		}
	}
	return errs
}

// ValidateJQPaths recursively validates all fields tagged with 'jq:"validate"'
func (v *RIValidator) validateJQ() []error {
	var errs []error
	validateJQRecursive(reflect.ValueOf(v.RI), "", &errs)

	return errs
}

func validateJQRecursive(val reflect.Value, fieldPath string, errs *[]error) {
	// Handle nil pointers
	if !val.IsValid() {
		return
	}

	// Dereference pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		validateJQForStruct(val, fieldPath, errs)
	case reflect.Slice, reflect.Array:
		validateJQForSlice(val, fieldPath, errs)
	case reflect.Map:
		validateJQForMap(val, fieldPath, errs)
	}
}

func validateJQForStruct(val reflect.Value, basePath string, errs *[]error) {
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Build field path for error reporting
		currentPath := buildFieldPath(basePath, fieldType.Name)

		// Check if field has jq:"validate" tag
		if jqTag, ok := fieldType.Tag.Lookup("jq"); ok && jqTag == "validate" {
			if err := validateJQField(field, currentPath); err != nil {
				*errs = append(*errs, err)
			}
		}

		// Recursively validate nested structures
		validateJQRecursive(field, currentPath, errs)
	}
}

func validateJQForSlice(val reflect.Value, basePath string, errs *[]error) {
	for i := 0; i < val.Len(); i++ {
		indexPath := fmt.Sprintf("%s[%d]", basePath, i)
		validateJQRecursive(val.Index(i), indexPath, errs)
	}
}

func validateJQForMap(val reflect.Value, basePath string, errs *[]error) {
	for _, key := range val.MapKeys() {
		keyPath := fmt.Sprintf("%s[%v]", basePath, key.Interface())
		validateJQRecursive(val.MapIndex(key), keyPath, errs)
	}
}

func validateJQField(field reflect.Value, fieldPath string) error {
	// Handle pointer to string (*string)
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil // Optional field, skip validation
		}
		field = field.Elem()
	}

	// Ensure field is a string
	if field.Kind() != reflect.String {
		return fmt.Errorf("%s: jq:\"validate\" tag can only be used on string or *string fields", fieldPath)
	}

	jqExpression := field.String()
	if jqExpression == "" {
		return nil // Empty jqExpression, skip validation
	}

	parsed, err := gojq.Parse(jqExpression)
	if err != nil {
		return fmt.Errorf("failed to parse JQ expression \"%s\" at \"%s\": %w", jqExpression, fieldPath, err)
	}

	err = validatedParsedJQ(parsed)
	if err != nil {
		return fmt.Errorf("JQ expression \"%s\" at \"%s\" failed validation: %w", jqExpression, fieldPath, err)
	}

	return nil
}

func buildFieldPath(basePath, fieldName string) string {
	if basePath == "" {
		return fieldName
	}
	return fmt.Sprintf("%s.%s", basePath, fieldName)
}

// validateUserQuery checks if a gojq query is read-only and safe
func validatedParsedJQ(q *gojq.Query) error {
	if q == nil {
		return nil
	}

	if q.Term != nil {
		if q.Term.Func != nil {
			// Reject mutating and recursion-related functions
			switch q.Term.Func.Name {
			case "del":
				return errors.New("del function is not allowed")
			case "recurse", "walk", "repeat":
				return fmt.Errorf("function %q is not allowed", q.Func)
			case "range", "paths", "leaf_paths":
				return fmt.Errorf("function %q may produce excessive output and is not allowed", q.Func)
			}

			for _, arg := range q.Term.Func.Args {
				err := validatedParsedJQ(arg)
				if err != nil {
					return err
				}
			}
		}

		if q.Term.Type == gojq.TermTypeRecurse {
			return errors.New("recursive descent operator '..' is not allowed")
		}
	}

	if q.Op > 0 {
		switch q.Op {
		case gojq.OpAssign, gojq.OpModify, gojq.OpUpdateAdd, gojq.OpUpdateSub, gojq.OpUpdateMul, gojq.OpUpdateDiv, gojq.OpUpdateMod, gojq.OpUpdateAlt:
			return fmt.Errorf("modifying jq operator \"%s\" is not allowed", q.Op)
		}

		err := validatedParsedJQ(q.Left)
		if err != nil {
			return err
		}

		err = validatedParsedJQ(q.Right)
		if err != nil {
			return err
		}

	}

	return nil
}
