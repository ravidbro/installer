package kubevirt

import (
	"gopkg.in/AlecAivazis/survey.v1"

	"github.com/openshift/installer/pkg/types/kubevirt"
)

// Platform collects kubevirt-specific configuration.
func Platform() (*kubevirt.Platform, error) {
	var (
		namespace, apiVIP, ingressVIP, networkName, storageClass string
		err                                                      error
	)

	if namespace, err = selectNamespace(); err != nil {
		return nil, err
	}

	if apiVIP, err = selectAPIVIP(); err != nil {
		return nil, err
	}

	if ingressVIP, err = selectIngressVIP(); err != nil {
		return nil, err
	}

	if networkName, err = selectNetworkName(); err != nil {
		return nil, err
	}

	if storageClass, err = selectStorageClass(); err != nil {
		return nil, err
	}

	return &kubevirt.Platform{
		Namespace:    namespace,
		StorageClass: storageClass,
		APIVIP:       apiVIP,
		IngressVIP:   ingressVIP,
		NetworkName:  networkName,
	}, nil
}

func selectNamespace() (string, error) {
	var selectedNamespace string

	err := survey.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{
				Message: "Namespace",
				Help:    "The namespace, in the undercluster, where all the resources of the overcluster would be created.",
			},
		},
	}, &selectedNamespace)

	return selectedNamespace, err
}

func selectAPIVIP() (string, error) {
	var selectedAPIVIP string

	defaultValue := ""

	err := survey.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{
				Message: "API VIP",
				Help:    "An IP which will be served by bootstrap and then pivoted masters, using keepalived.",
				Default: defaultValue,
			},
		},
	}, &selectedAPIVIP)

	return selectedAPIVIP, err
}

func selectIngressVIP() (string, error) {
	var selectedIngressVIP string

	defaultValue := ""

	err := survey.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{
				Message: "Ingress VIP",
				Help:    "An external IP which routes to the default ingress controller.",
				Default: defaultValue,
			},
		},
	}, &selectedIngressVIP)

	return selectedIngressVIP, err
}

func selectNetworkName() (string, error) {
	var selectedNetworkName string

	defaultValue := ""

	err := survey.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{
				Message: "Network Name",
				Help:    "The target network of all the network interfaces of the nodes.",
				Default: defaultValue,
			},
		},
	}, &selectedNetworkName)

	return selectedNetworkName, err
}

func selectStorageClass() (string, error) {
	var selectedStorageClass string

	defaultValue := ""

	err := survey.Ask([]*survey.Question{
		{
			Prompt: &survey.Input{
				Message: "Storage Class",
				Help:    "The name of the storage class used in the infra ocp cluster.",
				Default: defaultValue,
			},
		},
	}, &selectedStorageClass)

	return selectedStorageClass, err
}
