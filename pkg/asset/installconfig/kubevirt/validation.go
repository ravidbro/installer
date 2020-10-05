package kubevirt

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/kubevirt"
	"github.com/openshift/installer/pkg/types/kubevirt/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate executes kubevirt specific validation
func Validate(ic *types.InstallConfig, clientBuilderFunc ClientBuilderFuncType) error {
	kubevirtPlatformPath := field.NewPath("platform", "kubevirt")

	if ic.Platform.Kubevirt == nil {
		return errors.New(field.Required(
			kubevirtPlatformPath,
			"validation requires a Engine platform configuration").Error())
	}

	return validatePlatform(ic.Platform.Kubevirt, ic.MachineNetwork, clientBuilderFunc, kubevirtPlatformPath).ToAggregate()
}

func validatePlatform(kubevirtPlatform *kubevirt.Platform, machineNetworkEntryList []types.MachineNetworkEntry, clientBuilderFunc ClientBuilderFuncType, fldPath *field.Path) field.ErrorList {
	allErrs := validation.ValidatePlatform(kubevirtPlatform, fldPath)
	ctx := context.Background()

	client, resultErrs := validateInfraClusterReachable(ctx, clientBuilderFunc, fldPath)
	allErrs = append(allErrs, resultErrs...)
	if client != nil {
		nsErr := validateNamespaceExistsInInfraCluster(ctx, kubevirtPlatform.Namespace, client, fldPath)
		allErrs = append(allErrs, nsErr...)
		allErrs = append(allErrs, validateStorageClassExistsInInfraCluster(ctx, kubevirtPlatform.StorageClass, client, fldPath)...)
		if len(nsErr) == 0 {
			allErrs = append(allErrs, validateNetworkAttachmentDefinitionExistsInInfraCluster(ctx, kubevirtPlatform.NetworkName, kubevirtPlatform.Namespace, client, fldPath)...)
		}
	}
	allErrs = append(allErrs, validateIPsInMachineNetworkEntryList(machineNetworkEntryList, kubevirtPlatform.APIVIP, kubevirtPlatform.IngressVIP, fldPath)...)

	return allErrs
}

func validateInfraClusterReachable(ctx context.Context, clientBuilderFunc ClientBuilderFuncType, fieldPath *field.Path) (Client, field.ErrorList) {
	allErrs := field.ErrorList{}
	client, err := clientBuilderFunc()
	if err != nil {
		detailedErr := fmt.Errorf("failed to create InfraCluster client with error: %v", err)
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("InfraClusterReachable"), "InfraCluster", detailedErr.Error()))

		return nil, allErrs
	}

	if _, err := client.ListNamespace(ctx); err != nil {
		detailedErr := fmt.Errorf("failed to access to InfraCluster with error: %v", err)
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("InfraClusterReachable"), "InfraCluster", detailedErr.Error()))

		return nil, allErrs
	}

	return client, allErrs
}

func validateNamespaceExistsInInfraCluster(ctx context.Context, name string, client Client, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, err := client.GetNamespace(ctx, name); err != nil {
		detailedErr := fmt.Errorf("failed to get namespace %s from InfraCluster, with error: %v", name, err)
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("NamespaceExistsInInfraCluster"), name, detailedErr.Error()))
	}

	return allErrs
}

func validateStorageClassExistsInInfraCluster(ctx context.Context, name string, client Client, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, err := client.GetStorageClass(ctx, name); err != nil {
		detailedErr := fmt.Errorf("failed to get storageClass %s from InfraCluster, with error: %v", name, err)
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("StorageClassExistsInInfraCluster"), name, detailedErr.Error()))
	}

	return allErrs
}

func validateNetworkAttachmentDefinitionExistsInInfraCluster(ctx context.Context, name string, namespace string, client Client, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, err := client.GetNetworkAttachmentDefinition(ctx, name, namespace); err != nil {
		detailedErr := fmt.Errorf("failed to get network-attachment-definition %s from InfraCluster, with error: %v", name, err)
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("NetworkAttachmentDefinitionExistsInInfraCluster"), name, detailedErr.Error()))
	}

	return allErrs
}

func validateIPsInMachineNetworkEntryList(machineNetworkEntryList []types.MachineNetworkEntry, apiVIP string, ingressVIP string, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if err := assertIPInMachineNetworkEntryList(machineNetworkEntryList, apiVIP); err != nil {
		detailedErr := fmt.Errorf("validation of apiVIP %s in cider %s failed, with error: %v", apiVIP, machineNetworkEntryList, err)
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("IPsInCIDR"), apiVIP, detailedErr.Error()))
	}

	if err := assertIPInMachineNetworkEntryList(machineNetworkEntryList, ingressVIP); err != nil {
		detailedErr := fmt.Errorf("validation of ingressVIP %s in cider %s failed, with error: %v", ingressVIP, machineNetworkEntryList, err)
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("IPsInCIDR"), ingressVIP, detailedErr.Error()))
	}

	return allErrs
}

func assertIPInMachineNetworkEntryList(machineNetworkEntryList []types.MachineNetworkEntry, ip string) error {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return fmt.Errorf("ip %s is not valid IP address", ip)
	}
	for _, machineNetworkEntry := range machineNetworkEntryList {
		if machineNetworkEntry.CIDR.Contains(ipAddr) {
			return nil
		}
		return fmt.Errorf("ip %s not in machineNetworkEntryList %s", ip, machineNetworkEntryList)
	}
	return nil
}
