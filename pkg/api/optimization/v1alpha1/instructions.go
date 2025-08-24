package v1alpha1

type GangSchedulingInstruction struct {
	PodGroups []PodGroupDefinition `json:"podGroups"`
}

type PodGroupDefinition struct {
	Name    string             `json:"name"`    // the name of the pod group
	Members []GroupingSelector `json:"members"` // the members of the pod group by components
}

// GroupingSelector defines how to select and filter components for grouping instructions
type GroupingSelector struct {
	ComponentName   string   `json:"componentName"`             // References a component defined in the RID's structureDefinition
	GroupByKeyPaths []string `json:"groupByKeyPaths,omitempty"` // JQ path of values to group by (e.g owner name, replica key, etc.). optional - if nil, can find owning component via owner ref traversal
	Filters         []string `json:"filters,omitempty"`         // Optional, List of JQ filter expressions to select specific components (e.g., '(.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0) > 0'). Expressions are ANDed
}
