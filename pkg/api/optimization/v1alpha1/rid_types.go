package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceInterpretationDefinition
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName={rid}
// +kubebuilder:printcolumn:name="Framework",type="string",JSONPath=".spec.structureDefinition.rootComponent.kind.kind",description="Target framework kind"
// +kubebuilder:printcolumn:name="Root Component",type="string",JSONPath=".spec.structureDefinition.rootComponent.name",description="Root component name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ResourceInterpretationDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceInterpretationDefinitionSpec   `json:"spec,omitempty"`
	Status ResourceInterpretationDefinitionStatus `json:"status,omitempty"`
}

type ResourceInterpretationDefinitionSpec struct {
	// StructureDefinition defines the compute hierarchy and component relationships
	// +kubebuilder:validation:Required
	StructureDefinition StructureDefinition `json:"structureDefinition"`

	// Instructions contains optimization-specific instructions for the workload
	// +kubebuilder:validation:Optional
	Instructions OptimizationInstructions `json:"optimizationInstructions"`
}

// StructureDefinition defines the hierarchical structure of components in the workload.
type StructureDefinition struct {
	// RootComponent defines the top-level component of the workload hierarchy
	// +kubebuilder:validation:Required
	RootComponent ComponentDefinition `json:"rootComponent"`

	// ChildComponents defines the child components in the hierarchy
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	ChildComponents []ComponentDefinition `json:"childComponents,omitempty"`

	// AdditionalChildKinds lists Kubernetes kinds that are created/managed by this workload
	// but are not explicitly modeled as components (e.g., Deployments, Services).
	// Required for RBAC purposes, etc.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=kind
	AdditionalChildKinds []GroupVersionKind `json:"additionalChildKinds,omitempty"`
}

// OptimizationInstructions contains various optimization strategies that can be applied to the workload.
type OptimizationInstructions struct {
	// +kubebuilder:validation:Optional
	GangScheduling *GangSchedulingInstruction `json:"gangScheduling,omitempty"`
}

type ResourceInterpretationDefinitionStatus struct {
	// +optional
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// ResourceInterpretationDefinitionList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ResourceInterpretationDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceInterpretationDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceInterpretationDefinition{}, &ResourceInterpretationDefinitionList{})
}
