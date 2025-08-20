package v1alpha1

import "k8s.io/apimachinery/pkg/runtime/schema"

type ComponentDefinition struct {
	Name             string                   `json:"name"`
	Kind             *schema.GroupVersionKind `json:"kind,omitempty"`
	OwnerName        *string                  `json:"ownerName,omitempty"`    // name of the owning component. nil if root?
	MetadataPath     *string                  `json:"metadataPath,omitempty"` // JQ path to the component metadata. optional - defaults to '.metadata'
	SpecDefinition   *SpecDefinition          `json:"specDefinition,omitempty"`
	ScaleDefinition  *ScaleDefinition         `json:"scaleDefinition,omitempty"` // path to the scale/size struct
	StatusDefinition *StatusDefinition        `json:"statusDefinition,omitempty"`
	PodSelector      *PodSelector             `json:"podSelector,omitempty"` // A key-value pair that if exists on a pod, indicates it's component type
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

type ScaleDefinition struct {
	ReplicasPath    *string `json:"replicasPath"`    // JQ path to replica count (e.g., '.replicas')
	MinReplicasPath *string `json:"minReplicasPath"` // JQ path to minimum replicas (e.g., '.minReplicas')
	MaxReplicasPath *string `json:"maxReplicasPath"` // JQ path to maximum replicas (e.g., '.maxReplicas')
}

type PodSelector struct {
	KeyPath string  `json:"keyPath"`         // JQ path to the key
	Value   *string `json:"value,omitempty"` // optional - if the key exists, the pod is this component type
}

type ResourceStatus string

const (
	InitializingStatus ResourceStatus = "Initializing" // "sink" for all pre running statuses
	RunningStatus      ResourceStatus = "Running"
	CompletedStatus    ResourceStatus = "Completed"
	FailedStatus       ResourceStatus = "Failed"
	UndefinedStatus    ResourceStatus = "Undefined" // default for when the user did not provide a status definition, and we couldn't infer it naively
)

type StatusDefinition struct {
	PhaseDefinition      *PhaseDefinition      `json:"phaseDefinition,omitempty"`
	ConditionsDefinition *ConditionsDefinition `json:"conditionsDefinition,omitempty"`
	StatusMappings       StatusMappings        `json:"statusMappings"`
}

type PhaseDefinition struct {
	Path string `json:"path"`
}

type ConditionsDefinition struct {
	Path             string `json:"path"`
	TypeFieldName    string `json:"typeFieldName"`
	StatusFieldName  string `json:"statusFieldName"`
	MessageFieldName string `json:"messageFieldName"`
}

type StatusMappings struct {
	Initializing []StatusMatcher `json:"initializing,omitempty"`
	Running      []StatusMatcher `json:"running,omitempty"`
	Completed    []StatusMatcher `json:"completed,omitempty"`
	Failed       []StatusMatcher `json:"failed,omitempty"`
}

type StatusMatcher struct {
	ByPhase      string              `json:"byPhase,omitempty"`
	ByConditions []ExpectedCondition `json:"byConditions,omitempty"` // ANDed
	// Implicit logic: if both provided, ALL must match (AND)
}

type ExpectedCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}
