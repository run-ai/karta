package instructions

// GPUInterconnect represents optimization for workloads requiring high-bandwidth
// GPU interconnects (NVIDIA MultiNodeNVLink, AWS EFA, etc.) for multi-node GPU communication
type GPUInterconnect struct {
	AcceleratedComponents []ComponentSelector `json:"acceleratedComponents"`
}
