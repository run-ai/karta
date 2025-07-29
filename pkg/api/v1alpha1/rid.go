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
	Kind                schema.GroupVersionKind         `json:"kind"`
	StructureDefinition []structure.ComponentDefinition `json:"structureDefinition"`
	Instructions        OptimizationInstructions        `json:"optimizationInstructions"`
	ChildKinds          []schema.GroupVersionKind       `json:"childKinds,omitempty"` // make sure contains all the referenced components' types
}

type OptimizationInstructions struct {
	GangScheduling    *instructions.GangScheduling    `json:"gangScheduling,omitempty"`
	MultiNodeNVLink   *instructions.MultiNodeNVLink   `json:"multiNodeNVLink,omitempty"`
	TopologyAwareness *instructions.TopologyAwareness `json:"topologyAwareness,omitempty"`
}

type ResourceInterpretationDefinitionStatus struct {
}
