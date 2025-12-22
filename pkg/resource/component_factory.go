package resource

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/run-ai/kai-bolt/pkg/api/optimization/v1alpha1"
	"github.com/run-ai/kai-bolt/pkg/jq/execution"
)

type ComponentReader interface {
	ExtractPodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodTemplateSpec, error)
	ExtractPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]corev1.PodSpec, error)
	ExtractPodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]metav1.ObjectMeta, error)
	ExtractFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]FragmentedPodSpec, error)
	ExtractScale(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]Scale, error)
	ExtractStatus(ctx context.Context, definition v1alpha1.ComponentDefinition) (*Status, error)
	ExtractInstanceIds(ctx context.Context, definition v1alpha1.ComponentDefinition) ([]string, error)
	GetObject() (map[string]interface{}, error)
}

type ComponentWriter interface {
	UpdatePodTemplateSpec(ctx context.Context, definition v1alpha1.ComponentDefinition, podTemplateSpecs []corev1.PodTemplateSpec) error
	UpdatePodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition, podSpecs []corev1.PodSpec) error
	UpdatePodMetadata(ctx context.Context, definition v1alpha1.ComponentDefinition, podMetadata []metav1.ObjectMeta) error
	UpdateFragmentedPodSpec(ctx context.Context, definition v1alpha1.ComponentDefinition, fragmentedPodSpecs []FragmentedPodSpec) error
}

//go:generate mockgen -source=component_factory.go -destination=accessor_mock.go -package=resource ComponentAccessor
type ComponentAccessor interface {
	ComponentReader
	ComponentWriter
}

type ComponentFactory struct {
	ri       *v1alpha1.ResourceInterface
	accessor ComponentAccessor

	componentDefinitionsByName map[string]v1alpha1.ComponentDefinition
}

// NewComponentFactory creates a new ResourceInterface-based component factory
func NewComponentFactory(ri *v1alpha1.ResourceInterface, accessor ComponentAccessor) *ComponentFactory {
	definitionsByName := make(map[string]v1alpha1.ComponentDefinition)

	// Create single slice with all components (root + children)
	allDefinitions := append(ri.Spec.StructureDefinition.ChildComponents, ri.Spec.StructureDefinition.RootComponent)
	for _, componentDefinition := range allDefinitions {
		definitionsByName[componentDefinition.Name] = componentDefinition
	}

	return &ComponentFactory{
		ri:                         ri,
		accessor:                   accessor,
		componentDefinitionsByName: definitionsByName,
	}
}

// NewComponentFactoryFromObject creates a new ResourceInterface-based component factory from a Kubernetes object
func NewComponentFactoryFromObject(ri *v1alpha1.ResourceInterface, object client.Object) *ComponentFactory {
	jqRunner := execution.NewDefault(object)
	accessor := NewAccessor(jqRunner)
	return NewComponentFactory(ri, accessor)
}

// GetComponent retrieves a component by name
func (f *ComponentFactory) GetComponent(name string) (*Component, error) {
	definition, exists := f.componentDefinitionsByName[name]
	if !exists {
		return nil, fmt.Errorf("component %s not found", name)
	}

	return &Component{
		name:       name,
		definition: definition,
		accessor:   f.accessor,
	}, nil
}

// GetRootComponent retrieves the root component
func (f *ComponentFactory) GetRootComponent() (*Component, error) {
	if f.ri == nil {
		return nil, fmt.Errorf("resource interface is nil")
	}

	return f.GetComponent(f.ri.Spec.StructureDefinition.RootComponent.Name)
}

// GetChildComponents retrieves all child components
func (f *ComponentFactory) GetChildComponents() ([]*Component, error) {
	if f.ri == nil {
		return nil, fmt.Errorf("resource interface is nil")
	}

	childComponents := make([]*Component, 0, len(f.ri.Spec.StructureDefinition.ChildComponents))
	for _, childDefinition := range f.ri.Spec.StructureDefinition.ChildComponents {
		component, err := f.GetComponent(childDefinition.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get child component %s: %w", childDefinition.Name, err)
		}
		childComponents = append(childComponents, component)
	}

	return childComponents, nil
}

func (f *ComponentFactory) GetResource() (client.Object, error) {
	object, err := f.accessor.GetObject()
	if err != nil {
		return nil, fmt.Errorf("failed to get updated data: %w", err)
	}
	u := &unstructured.Unstructured{Object: object}
	if err := validateKubernetesObject(u); err != nil {
		return nil, fmt.Errorf("invalid Kubernetes object: %w", err)
	}
	return u, nil
}

func validateKubernetesObject(u *unstructured.Unstructured) error {
	gvk := u.GroupVersionKind()
	if gvk.Group == "" && gvk.Version == "" { // Core groups might have empty Group, but need Version
		return fmt.Errorf("missing apiVersion")
	}
	if gvk.Kind == "" {
		return fmt.Errorf("missing kind")
	}
	if u.GetName() == "" && u.GetGenerateName() == "" {
		return fmt.Errorf("missing metadata.name or metadata.generateName")
	}
	return nil
}
