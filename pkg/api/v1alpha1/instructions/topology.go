package instructions

type TopologyAwareness struct {
	TopologyGroups []TopologyGroupDefinition `json:"topologyGroups"`
}

type TopologyGroupDefinition struct {
	TopologyName       *string           `json:"topologyName,omitempty"`       // the name of the topology object
	PreferredPlacement *string           `json:"preferredPlacement,omitempty"` // e.g "rack", "zone", "node"
	RequiredPlacement  *string           `json:"requiredPlacement,omitempty"`  // e.g "rack", "zone", "node"
	Members            []GeneralSelector `json:"members"`                      // the members of the pod group by components
}
