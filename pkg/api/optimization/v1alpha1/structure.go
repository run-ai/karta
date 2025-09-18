package v1alpha1

// GroupVersionKind represents a Kubernetes API object's group, version, and kind.
type GroupVersionKind struct {
	// Group is the API group of the resource
	Group string `json:"group"`

	// Version is the API version of the resource
	Version string `json:"version"`

	// Kind is the API kind of the resource
	Kind string `json:"kind"`
}

// ComponentDefinition defines a single component in the workload hierarchy.
// Components represent logical units of computation that can be optimized independently.
type ComponentDefinition struct {
	// Name is the unique identifier for this component within the RI
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Kind specifies the Kubernetes GroupVersionKind for this component
	// +kubebuilder:validation:Optional
	Kind *GroupVersionKind `json:"kind,omitempty"`

	// OwnerRef references the parent component in the hierarchy by name (nil for root component)
	// +kubebuilder:validation:Optional
	OwnerRef *string `json:"ownerRef,omitempty"`

	// SpecDefinition defines how to extract pod specifications from this component
	// +kubebuilder:validation:Optional
	SpecDefinition *SpecDefinition `json:"specDefinition,omitempty"`

	// ScaleDefinition defines how to extract scaling information from this component
	// +kubebuilder:validation:Optional
	ScaleDefinition *ScaleDefinition `json:"scaleDefinition,omitempty"`

	// StatusDefinition defines how to interpret the status of this component
	// should be added only for the root component
	// +kubebuilder:validation:Optional
	StatusDefinition *StatusDefinition `json:"statusDefinition,omitempty"`

	// InstanceIdPath is the JQ path to the instance ID, for components that hold multiple pod definitions (in array or map)
	// +kubebuilder:validation:Optional
	InstanceIdPath *string `json:"instanceIdPath,omitempty"`

	// PodSelector defines how to identify pods belonging to this component
	// +kubebuilder:validation:Optional
	PodSelector *PodSelector `json:"podSelector,omitempty"`
}

// SpecDefinition defines how to extract pod specifications from a component.
// Only one of the three options should be provided (PodTemplateSpec, FragmentedPodSpec, PodSpec + Metadata).
type SpecDefinition struct {
	// PodTemplateSpecPath is the JQ path to a complete PodTemplateSpec object
	// +kubebuilder:validation:Optional
	PodTemplateSpecPath *string `json:"podTemplateSpecPath,omitempty"`

	// PodSpecPath is the JQ path to a complete PodSpec object
	// +kubebuilder:validation:Optional
	PodSpecPath *string `json:"podSpecPath,omitempty"`

	// MetadataPath is the JQ path to the component metadata
	// May be used only with PodSpecPath, in cases where pod spec and metadata are separated
	// +kubebuilder:validation:Optional
	MetadataPath *string `json:"metadataPath,omitempty"`

	// FragmentedPodSpecDefinition defines how to extract individual pod spec fields
	// when they are scattered across different paths in the component
	// +kubebuilder:validation:Optional
	FragmentedPodSpecDefinition *FragmentedPodSpecDefinition `json:"fragmentedPodSpecDefinition,omitempty"`
}

// FragmentedPodSpecDefinition defines JQ paths to individual pod spec fields
// when they are scattered across different locations in the component YAML.
type FragmentedPodSpecDefinition struct {
	// SchedulerNamePath is the JQ path to the scheduler name
	// +kubebuilder:validation:Optional
	SchedulerNamePath *string `json:"schedulerNamePath,omitempty"`

	// LabelsPath is the JQ path to pod labels
	// +kubebuilder:validation:Optional
	LabelsPath *string `json:"labelsPath,omitempty"`

	// AnnotationsPath is the JQ path to pod annotations
	// +kubebuilder:validation:Optional
	AnnotationsPath *string `json:"annotationsPath,omitempty"`

	// ResourcesPath is the JQ path to resource requirements
	// +kubebuilder:validation:Optional
	ResourcesPath *string `json:"resourcesPath,omitempty"`

	// ResourceClaimsPath is the JQ path to DRA resource claims
	// +kubebuilder:validation:Optional
	ResourceClaimsPath *string `json:"resourceClaimsPath,omitempty"`

	// PodAffinityPath is the JQ path to pod affinity rules
	// +kubebuilder:validation:Optional
	PodAffinityPath *string `json:"podAffinityPath,omitempty"`

	// NodeAffinityPath is the JQ path to node affinity rules
	// +kubebuilder:validation:Optional
	NodeAffinityPath *string `json:"nodeAffinityPath,omitempty"`

	// ContainersPath is the JQ path to containers specifications
	// +kubebuilder:validation:Optional
	ContainersPath *string `json:"containersPath,omitempty"`

	// ContainesPath is the JQ path to a single container specifications
	// Used when the component has only one container
	// +kubebuilder:validation:Optional
	ContainerPath *string `json:"containerPath,omitempty"`

	// PriorityClassNamePath is the JQ path to the priority class name
	// +kubebuilder:validation:Optional
	PriorityClassNamePath *string `json:"priorityClassNamePath,omitempty"`

	// ImagePath is the JQ path to the container image
	// +kubebuilder:validation:Optional
	ImagePath *string `json:"imagePath,omitempty"`
}

// ScaleDefinition defines how to extract scaling information from a component.
type ScaleDefinition struct {
	// ReplicasPath is the JQ path to the current replica count
	// +kubebuilder:validation:Optional
	ReplicasPath *string `json:"replicasPath,omitempty"`

	// MinReplicasPath is the JQ path to the minimum replica count
	// +kubebuilder:validation:Optional
	MinReplicasPath *string `json:"minReplicasPath,omitempty"`

	// MaxReplicasPath is the JQ path to the maximum replica count
	// +kubebuilder:validation:Optional
	MaxReplicasPath *string `json:"maxReplicasPath,omitempty"`
}

// PodSelector defines how to identify pods belonging to a specific component.
type PodSelector struct {
	// ComponentTypeSelector identifies whether the pod matches a specific component type
	// +kubebuilder:validation:Optional
	ComponentTypeSelector *ComponentTypeSelector `json:"componentTypeSelector,omitempty"`

	// ComponentInstanceSelector identifies the component instance the pod matches, in case the component has multiple instances
	// +kubebuilder:validation:Optional
	ComponentInstanceSelector *ComponentInstanceSelector `json:"componentInstanceSelector,omitempty"`
}

type ComponentTypeSelector struct {
	// KeyPath is the JQ path to the identifying key/label on the pod
	// JQ paths are evaluated against individual pod objects, not the root resource spec
	// +kubebuilder:validation:Required
	KeyPath string `json:"keyPath"`

	// Value is the expected value for the key (optional - if nil, only key existence is checked)
	// +kubebuilder:validation:Optional
	Value *string `json:"value,omitempty"`
}

type ComponentInstanceSelector struct {
	// IdPath is the JQ path to the component instance identifier on the pod
	// JQ paths are evaluated against individual pod objects, not the root resource spec
	// +kubebuilder:validation:Required
	IdPath string `json:"idPath"`
}

// ResourceStatus represents the high-level status of a component.
// +kubebuilder:validation:Enum=Initializing;Running;Completed;Failed;Undefined
type ResourceStatus string

const (
	// InitializingStatus indicates the component has been created or starting up or preparing to run (pre Running status)
	InitializingStatus ResourceStatus = "Initializing"

	// RunningStatus indicates the component is actively running
	RunningStatus ResourceStatus = "Running"

	// CompletedStatus indicates the component has finished successfully
	CompletedStatus ResourceStatus = "Completed"

	// FailedStatus indicates the component has failed
	FailedStatus ResourceStatus = "Failed"

	// UndefinedStatus is used when status was not defined or cannot be determined
	UndefinedStatus ResourceStatus = "Undefined"
)

// StatusDefinition defines how to interpret the status of a component.
type StatusDefinition struct {
	// PhaseDefinition defines how to extract a simple phase/state string
	// +kubebuilder:validation:Optional
	PhaseDefinition *PhaseDefinition `json:"phaseDefinition,omitempty"`

	// ConditionsDefinition defines how to extract Kubernetes-style conditions
	// +kubebuilder:validation:Optional
	ConditionsDefinition *ConditionsDefinition `json:"conditionsDefinition,omitempty"`

	// StatusMappings define how to map extracted status to ResourceStatus values
	// +kubebuilder:validation:Required
	StatusMappings StatusMappings `json:"statusMappings"`
}

// PhaseDefinition defines how to extract a simple phase/state string from the component.
type PhaseDefinition struct {
	// Path is the JQ path to the phase/state field
	// +kubebuilder:validation:Required
	Path string `json:"path"`
}

// ConditionsDefinition defines how to extract Kubernetes-style conditions from the component.
type ConditionsDefinition struct {
	// Path is the JQ path to the conditions array
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// TypeFieldName is the field name for the condition type
	// +kubebuilder:validation:Required
	TypeFieldName string `json:"typeFieldName"`

	// StatusFieldName is the field name for the condition status
	// +kubebuilder:validation:Required
	StatusFieldName string `json:"statusFieldName"`

	// MessageFieldName is the field name for the condition text message
	// +kubebuilder:validation:Optional
	MessageFieldName *string `json:"messageFieldName"`
}

// StatusMappings define how to map extracted status information to ResourceStatus values.
type StatusMappings struct {
	// Initializing defines matchers for the Initializing status
	// +kubebuilder:validation:Optional
	// +listType=atomic
	Initializing []StatusMatcher `json:"initializing,omitempty"`

	// Running defines matchers for the Running status
	// +kubebuilder:validation:Optional
	// +listType=atomic
	Running []StatusMatcher `json:"running,omitempty"`

	// Completed defines matchers for the Completed status
	// +kubebuilder:validation:Optional
	// +listType=atomic
	Completed []StatusMatcher `json:"completed,omitempty"`

	// Failed defines matchers for the Failed status
	// +kubebuilder:validation:Optional
	// +listType=atomic
	Failed []StatusMatcher `json:"failed,omitempty"`
}

// StatusMatcher defines criteria for matching a specific status.
// If both ByPhase and ByConditions are provided, ALL must match (AND logic).
type StatusMatcher struct {
	// ByPhase matches against a specific phase value
	// +kubebuilder:validation:Optional
	ByPhase string `json:"byPhase,omitempty"`

	// ByConditions matches against specific condition combinations (ANDed together)
	// +kubebuilder:validation:Optional
	// +listType=atomic
	ByConditions []ExpectedCondition `json:"byConditions,omitempty"`
}

// ExpectedCondition defines a condition type and status that must be present.
type ExpectedCondition struct {
	// Type is the condition type to match
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// Status is the expected condition status
	// +kubebuilder:validation:Required
	Status string `json:"status"`
}
