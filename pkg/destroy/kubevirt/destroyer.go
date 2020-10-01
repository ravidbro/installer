package kubevirt

import (
	"github.com/sirupsen/logrus"

	ickubevirt "github.com/openshift/installer/pkg/asset/installconfig/kubevirt"
	"github.com/openshift/installer/pkg/destroy/providers"
	"github.com/openshift/installer/pkg/types"
)

// ClusterUninstaller holds the various options for the cluster we want to delete.
type ClusterUninstaller struct {
	Metadata types.ClusterMetadata
	Logger   logrus.FieldLogger
}

// Run is the entrypoint to start the uninstall process.
func (uninstaller *ClusterUninstaller) Run() error {
	namespace := uninstaller.Metadata.Kubevirt.Namespace
	labels := uninstaller.Metadata.Kubevirt.Labels

	kubevirtClient, err := ickubevirt.NewClient()
	if err != nil {
		return err
	}
	if err := uninstaller.deleteAllVMs(namespace, labels, kubevirtClient); err != nil {
		return err
	}
	if err := uninstaller.deleteAllDVs(namespace, labels, kubevirtClient); err != nil {
		return err
	}
	if err := uninstaller.deleteAllSecrets(namespace, labels, kubevirtClient); err != nil {
		return err
	}
	return nil
}

func (uninstaller *ClusterUninstaller) deleteAllVMs(namespace string, labels map[string]string, kubevirtClient ickubevirt.Client) error {
	list, err := kubevirtClient.ListVirtualMachineNames(namespace, labels)
	if err != nil {
		return err
	}
	uninstaller.Logger.Infof("List tenant cluster's VMs (in namespace %s) return: %s", namespace, list)
	for _, vmName := range list {
		uninstaller.Logger.Infof("Delete VM %s", vmName)
		if err := kubevirtClient.DeleteVirtualMachine(namespace, vmName, true); err != nil {
			// TODO Do we want to continue to other resources?
			return err
		}
	}
	return nil
}

func (uninstaller *ClusterUninstaller) deleteAllDVs(namespace string, labels map[string]string, kubevirtClient ickubevirt.Client) error {
	list, err := kubevirtClient.ListDataVolumeNames(namespace, labels)
	if err != nil {
		return err
	}
	uninstaller.Logger.Infof("List tenant cluster's DVs (in namespace %s) return: %s", namespace, list)
	for _, dvName := range list {
		uninstaller.Logger.Infof("Delete DV %s", dvName)
		if err := kubevirtClient.DeleteDataVolume(namespace, dvName, true); err != nil {
			// TODO Do we want to continue to other resources?
			return err
		}
	}
	return nil
}

func (uninstaller *ClusterUninstaller) deleteAllSecrets(namespace string, labels map[string]string, kubevirtClient ickubevirt.Client) error {
	list, err := kubevirtClient.ListSecretNames(namespace, labels)
	if err != nil {
		return err
	}
	uninstaller.Logger.Infof("List tenant cluster's secrets (in namespace %s) return: %s", namespace, list)
	for _, secretName := range list {
		uninstaller.Logger.Infof("Delete secret %s", secretName)
		if err := kubevirtClient.DeleteSecret(namespace, secretName, true); err != nil {
			// TODO Do we want to continue to other resources?
			return err
		}
	}
	return nil
}

// New returns oVirt Uninstaller from ClusterMetadata.
func New(logger logrus.FieldLogger, metadata *types.ClusterMetadata) (providers.Destroyer, error) {
	return &ClusterUninstaller{
		Metadata: *metadata,
		Logger:   logger,
	}, nil
}
