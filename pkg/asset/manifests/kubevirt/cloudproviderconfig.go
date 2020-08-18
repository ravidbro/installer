package kubevirt

import (
	"bytes"
	"encoding/json"
)

// CloudProviderConfig is the kubevirt cloud provider config
type CloudProviderConfig struct {
	// The namespace in the infra cluster that the cluster resources are created in
	Namespace string
	InfraID   string
}

type config struct {
	// The namespace in the infra cluster that the cluster resources are created in
	Namespace string `json:"namespace" yaml:"namespace"`
	InfraID   string `json:"infraID" yaml:"infraID"`
}

// JSON generates the cloud provider json config for the kubevirt platform.
func (params CloudProviderConfig) JSON() (string, error) {
	config := config{
		Namespace: params.Namespace,
		InfraID:   params.InfraID,
	}
	buff := &bytes.Buffer{}
	encoder := json.NewEncoder(buff)
	encoder.SetIndent("", "\t")
	if err := encoder.Encode(config); err != nil {
		return "", err
	}
	return buff.String(), nil
}
