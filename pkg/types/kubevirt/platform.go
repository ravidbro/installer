package kubevirt

// Platform stores all the global configuration that all
// machinesets use.
type Platform struct {
	// The Namespace in the infra cluster, which the control plane (master vms)
	// and the compute (worker vms) are installed in
	Namespace string `json:"namespace"`

	// The Storage Class used in the infra cluster
	StorageClass string `json:"storageClass"`

	// NetworkName is the target network of all the network interfaces of the nodes.
	NetworkName string `json:"networkName"`

	// APIVIP is an IP which will be served by bootstrap and then pivoted masters, using keepalived
	APIVIP string `json:"apiVIP"`

	// IngressIP is an external IP which routes to the default ingress controller.
	IngressVIP string `json:"ingressVIP"`

	// PersistentVolumeAccessMode is the access mode should be use with the persistent volumes
	PersistentVolumeAccessMode string `json:"persistentVolumeAccessMode,omitempty"`

	// DefaultMachinePlatform is the default configuration used when
	// installing on Kubevirt for machine pools which do not define their own
	// platform configuration.
	// +optional
	DefaultMachinePlatform *MachinePool `json:"defaultMachinePlatform,omitempty"`
}
