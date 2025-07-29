package instructions

type TopologyAwareness struct {
	TopologyGroups []TopologyGroupDefinition `json:"topologyGroups"`
}

type TopologyGroupDefinition struct {
	GroupName          string              `json:"groupName"`                    // the name of the topology group
	TopologyName       *string             `json:"topologyName,omitempty"`       // the name of the topology object
	PreferredPlacement *string             `json:"preferredPlacement,omitempty"` // e.g "rack", "zone", "node"
	RequiredPlacement  *string             `json:"requiredPlacement,omitempty"`  // e.g "rack", "zone", "node"
	Members            []ComponentSelector `json:"members"`                      // the members of the pod group by components
}
