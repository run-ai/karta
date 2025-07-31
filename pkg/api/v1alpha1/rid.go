package v1alpha1

import (
	"github.com/run-ai/runai/kai-bolt/pkg/api/v1alpha1/instructions"
	"github.com/run-ai/runai/kai-bolt/pkg/api/v1alpha1/structure"
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
	TopOwnerKind        schema.GroupVersionKind         `json:"topOwnerKind"`
	StructureDefinition []structure.ComponentDefinition `json:"structureDefinition"`
	Instructions        OptimizationInstructions        `json:"optimizationInstructions"`
}

type StructureDefinition struct {
	Components           []structure.ComponentDefinition `json:"components"`
	AdditionalChildKinds []schema.GroupVersionKind       `json:"additionalChildKinds,omitempty"` // make sure contains all the unspecified child components' kinds
}

type OptimizationInstructions struct {
	GangScheduling    *instructions.GangScheduling    `json:"gangScheduling,omitempty"`
	GPUInterconnect   *instructions.GPUInterconnect   `json:"gpuInterconnect,omitempty"`
	TopologyAwareness *instructions.TopologyAwareness `json:"topologyAwareness,omitempty"`
}

type ResourceInterpretationDefinitionStatus struct {
}
