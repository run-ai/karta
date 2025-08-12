package instructions

// GroupingSelector defines how to select and filter components for grouping instructions
type GroupingSelector struct {
	ComponentSelectorBase
	GroupByKeyPaths []string `json:"groupByKeyPaths,omitempty"` // JQ path of values to group by (e.g owner name, replica key, etc.). optional - if nil, can find owning component via owner ref traversal
}

// GeneralSelector defines how to select and filter components for instructions
type GeneralSelector struct {
	ComponentSelectorBase
	ComponentKeyPath *string `json:"componentKeyPath,omitempty"` // JQ path to the unique component key in pod (e.g., '.metadata.labels["training.kubeflow.org/job-name"]'). optional - if nil, can find owning component via owner ref traversal
}

type ComponentSelectorBase struct {
	ComponentName string   `json:"componentName"`     // References a component defined in the RID's structureDefinition
	Filters       []string `json:"filters,omitempty"` // Optional, List of JQ filter expressions to select specific components (e.g., '.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0 > 0'). Expressions are ANDed
}
