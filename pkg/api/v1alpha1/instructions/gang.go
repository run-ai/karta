package instructions

type GangScheduling struct {
	PodGroups []PodGroupDefinition `json:"podGroups"`
}

type PodGroupDefinition struct {
	Name      string              `json:"name"`                // the name of the pod group
	Members   []ComponentSelector `json:"members"`             // the members of the pod group by components
	DependsOn []string            `json:"dependsOn,omitempty"` // group names that the current group depends on for scheduling purposes
}
