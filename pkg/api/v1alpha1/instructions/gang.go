package instructions

type GangScheduling struct {
	PodGroups []PodGroupDefinition `json:"podGroups"`
}

type PodGroupDefinition struct {
	Members   []GroupingSelector `json:"members"`             // the members of the pod group by components
	DependsOn []string           `json:"dependsOn,omitempty"` // group names that the current group depends on for scheduling purposes
}
