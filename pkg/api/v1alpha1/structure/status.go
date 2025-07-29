package structure

type WorkloadStatus string

const (
	RunningStatus   WorkloadStatus = "Running"
	CompletedStatus WorkloadStatus = "Completed"
	FailedStatus    WorkloadStatus = "Failed"
	UnknownStatus   WorkloadStatus = "Unknown" // default for when the user did not provide a status definition and we couldn't infer it naively
	// StoppedStatus WorkloadStatus = "Stopped"
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
	ConditionsPath  string `json:"conditionsPath"`
	TypeFieldName   string `json:"typeFieldName"`
	StatusFieldName string `json:"statusFieldName"`
}

type StatusMappings struct {
	Running   *StatusMatcher `json:"running,omitempty"`
	Completed *StatusMatcher `json:"completed,omitempty"`
	Failed    *StatusMatcher `json:"failed,omitempty"`
}

type StatusMatcher struct {
	ByPhase      []string            `json:"byPhase,omitempty"`
	ByConditions []ExpectedCondition `json:"byConditions,omitempty"`
	// Implicit logic: if both provided, ALL must match (AND)
}

type ExpectedCondition struct {
	Type   string `json:"type"`   // e.g. "Ready"
	Status string `json:"status"` // e.g. "True"
}
