package structure

import "k8s.io/apimachinery/pkg/runtime/schema"

type ComponentDefinition struct {
	Name             string                   `json:"name"`
	Kind             *schema.GroupVersionKind `json:"kind,omitempty"`
	OwnerName        *string                  `json:"ownerName,omitempty"`       // name of the owning component. nil if root?
	SpecPath         string                   `json:"specPath"`                  // JQ path to the component spec (e.g., '.spec.services[]' or '.spec.template')
	MetadataPath     *string                  `json:"metadataPath"`              // JQ path to the component metadata. optional - defaults to ".metadata"
	References       []ReferenceDefinition    `json:"references,omitempty"`      // list of components this component references
	IsReference      bool                     `json:"isReference,omitempty"`     // true = this component is an external reference
	ScaleDefinition  *ScaleDefinition         `json:"scaleDefinition,omitempty"` // path to the scale/size struct
	StatusDefinition *StatusDefinition        `json:"statusDefinition,omitempty"`
	DependsOn        []string                 `json:"dependsOn,omitempty"` // component names that must be ready before this component starts
}

type ReferenceDefinition struct {
	ComponentName    string  `json:"componentName"`       // Name of the referenced component
	ComponentKeyPath string  `json:"componentKeyPath"`    // JQ path to where main resource stores this component's identifier
	Namespace        *string `json:"namespace,omitempty"` // optional - if the referenced component is in another namespace
}

type ScaleDefinition struct {
	ReplicasPath    *string `json:"replicasPath"`    // JQ path to replica count (e.g., '.replicas')
	MinReplicasPath *string `json:"minReplicasPath"` // JQ path to minimum replicas (e.g., '.minReplicas')
	MaxReplicasPath *string `json:"maxReplicasPath"` // JQ path to maximum replicas (e.g., '.maxReplicas')
}
