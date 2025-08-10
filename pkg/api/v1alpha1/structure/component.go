package structure

import "k8s.io/apimachinery/pkg/runtime/schema"

type ComponentDefinition struct {
	Name                string                   `json:"name"`
	Kind                *schema.GroupVersionKind `json:"kind,omitempty"`
	OwnerName           *string                  `json:"ownerName,omitempty"`    // name of the owning component. nil if root?
	SpecPath            *string                  `json:"specPath,omitempty"`     // JQ path to the component spec (e.g., '.spec.services[]' or '.spec.template') - defaults to '.spec', aka redundant for root components
	MetadataPath        *string                  `json:"metadataPath,omitempty"` // JQ path to the component metadata. optional - defaults to '.metadata'
	ChildSpecDefinition *ChildSpecDefinition     `json:"childSpecDefinition,omitempty"`
	ScaleDefinition     *ScaleDefinition         `json:"scaleDefinition,omitempty"` // path to the scale/size struct
	StatusDefinition    *StatusDefinition        `json:"statusDefinition,omitempty"`
	References          []ReferenceDefinition    `json:"references,omitempty"` // list of components this component references
}

type ChildSpecDefinition struct {
	// only one of the following should be provided
	PodTemplateSpecPath     *string                  `json:"podTemplateSpecPath,omitempty"`
	PodSpecPath             *string                  `json:"podSpecPath,omitempty"`
	FragmentedPodDefinition *FragmentedPodDefinition `json:"fragmentedPodDefinition,omitempty"` // for cases where the parent component is not holding a podSpec or podTemplateSpec objects
}

type FragmentedPodDefinition struct {
	SchedulerNamePath  *string `json:"schedulerNamePath"`
	LabelsPath         *string `json:"labelsPath"`
	AnnotationsPath    *string `json:"annotationsPath"`
	ResourcesPath      *string `json:"resourcesPath"`
	ResourceClaimsPath *string `json:"resourceClaimsPath"`
	PodAffinityPath    *string `json:"podAffinityPath"`
	NodeAffinityPath   *string `json:"nodeAffinityPath"`
	ContainersPath     *string `json:"containersPath"`
}

type ReferenceDefinition struct {
	ComponentName    string  `json:"componentName"`       // Name of the referenced component
	ComponentKeyPath string  `json:"componentKeyPath"`    // JQ path to where main resource stores the referenced component's identifier
	Namespace        *string `json:"namespace,omitempty"` // optional - if the referenced component is in another namespace
}

type ScaleDefinition struct {
	ReplicasPath    *string `json:"replicasPath"`    // JQ path to replica count (e.g., '.replicas')
	MinReplicasPath *string `json:"minReplicasPath"` // JQ path to minimum replicas (e.g., '.minReplicas')
	MaxReplicasPath *string `json:"maxReplicasPath"` // JQ path to maximum replicas (e.g., '.maxReplicas')
}
