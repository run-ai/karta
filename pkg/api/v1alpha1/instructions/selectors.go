package instructions

// ComponentSelector defines how to select and filter components for instructions
type ComponentSelector struct {
	ComponentDefinitionName string   `json:"componentDefinitionName"` // References a component defined in the RID's structureDefinition
	ComponentKeyPath        *string  `json:"componentKeyPath"`        // JQ path to the unique component key in pod (e.g., '.metadata.labels["training.kubeflow.org/job-name"]'). optional - if nil, can find owning component via owner ref traversal
	Filter                  []string `json:"filter,omitempty"`        // Optional, List of JQ filter expressions to select specific components (e.g., '.spec.containers[0].resources.limits["nvidia.com/gpu"] // 0 > 0'). Expressions are ANDed
}
