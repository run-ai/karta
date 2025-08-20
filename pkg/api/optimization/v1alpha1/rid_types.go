package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceInterpretationDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceInterpretationDefinitionSpec   `json:"spec,omitempty"`
	Status ResourceInterpretationDefinitionStatus `json:"status,omitempty"`
}

type ResourceInterpretationDefinitionSpec struct {
	StructureDefinition StructureDefinition      `json:"structureDefinition"`
	Instructions        OptimizationInstructions `json:"optimizationInstructions"`
}

type StructureDefinition struct {
	RootComponent        ComponentDefinition       `json:"rootComponent"`
	ChildComponents      []ComponentDefinition     `json:"childComponents,omitempty"`
	AdditionalChildKinds []schema.GroupVersionKind `json:"additionalChildKinds,omitempty"` // make sure contains all the unspecified child components' kinds
}

type OptimizationInstructions struct {
	GangScheduling *GangSchedulingInstruction `json:"gangScheduling,omitempty"`
}

type ResourceInterpretationDefinitionStatus struct {
}
