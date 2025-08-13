package structure

import "k8s.io/apimachinery/pkg/runtime/schema"

type ComponentDefinition struct {
	Name             string                   `json:"name"`
	Kind             *schema.GroupVersionKind `json:"kind,omitempty"`
	OwnerName        *string                  `json:"ownerName,omitempty"`    // name of the owning component. nil if root?
	MetadataPath     *string                  `json:"metadataPath,omitempty"` // JQ path to the component metadata. optional - defaults to '.metadata'
	SpecDefinition   *SpecDefinition          `json:"specDefinition,omitempty"`
	ScaleDefinition  *ScaleDefinition         `json:"scaleDefinition,omitempty"` // path to the scale/size struct
	StatusDefinition *StatusDefinition        `json:"statusDefinition,omitempty"`
	References       []ReferenceDefinition    `json:"references,omitempty"` // list of components this component references
}

type SpecDefinition struct {
	// only one of the following should be provided
	PodTemplateSpecPath     *string                  `json:"podTemplateSpecPath,omitempty"`     // JQ path to the component pod template spec
	PodSpecPath             *string                  `json:"podSpecPath,omitempty"`             // JQ path to the component pod spec
	FragmentedPodDefinition *FragmentedPodDefinition `json:"fragmentedPodDefinition,omitempty"` // for cases where the parent component is not holding a podSpec or podTemplateSpec objects
}

type FragmentedPodDefinition struct {
	SchedulerNamePath     *string `json:"schedulerNamePath,omitempty"`
	LabelsPath            *string `json:"labelsPath,omitempty"`
	AnnotationsPath       *string `json:"annotationsPath,omitempty"`
	ResourcesPath         *string `json:"resourcesPath,omitempty"`
	ResourceClaimsPath    *string `json:"resourceClaimsPath,omitempty"`
	PodAffinityPath       *string `json:"podAffinityPath,omitempty"`
	NodeAffinityPath      *string `json:"nodeAffinityPath,omitempty"`
	ContainersPath        *string `json:"containersPath,omitempty"`
	PriorityClassNamePath *string `json:"priorityClassNamePath,omitempty"`
	ImagePath             *string `json:"imagePath,omitempty"`
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
