package kubevirt

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/openshift/installer/pkg/asset/installconfig/kubevirt/mock"
	"github.com/openshift/installer/pkg/ipnet"
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/kubevirt"
)

var (
	validNamespace        = "valid-namespace"
	validStorageClass     = "valid-storage-class"
	validNetworkName      = "valid-network-name"
	validAPIVIP           = "192.168.123.15"
	validIngressVIP       = "192.168.123.20"
	validAccessMode       = "valid-access-mode"
	validMachineCIDR      = "192.168.123.0/24"
	invalidKubeconfigPath = "invalid-kubeconfig-path"
	invalidNamespace      = "invalid-namespace"
	invalidStorageClass   = "invalid-storage-class"
	invalidNetworkName    = "invalid-network-name"
	invalidAPIVIP         = "invalid-api-vip"
	invalidIngressVIP     = "invalid-ingress-vip"
	invalidAccessMode     = "invalid-access-mode"
	invalidMachineCIDR    = "10.0.0.0/16"
)

func validInstallConfig() *types.InstallConfig {
	return &types.InstallConfig{
		Networking: &types.Networking{
			MachineNetwork: []types.MachineNetworkEntry{
				{CIDR: *ipnet.MustParseCIDR(validMachineCIDR)},
			},
		},
		Platform: types.Platform{
			Kubevirt: &kubevirt.Platform{
				Namespace:                  validNamespace,
				StorageClass:               validStorageClass,
				NetworkName:                validNetworkName,
				APIVIP:                     validAPIVIP,
				IngressVIP:                 validIngressVIP,
				PersistentVolumeAccessMode: validAccessMode,
			},
		},
	}
}

func TestKubevirtInstallConfigValidation(t *testing.T) {
	cases := []struct {
		name             string
		edit             func(ic *types.InstallConfig)
		expectedError    bool
		expectedErrMsg   string
		clientBuilderErr error
		expectClient     func(kubevirtClient *mock.MockClient)
	}{
		{
			name:           "valid",
			edit:           nil,
			expectedError:  false,
			expectedErrMsg: "",
			expectClient: func(kubevirtClient *mock.MockClient) {
				kubevirtClient.EXPECT().ListNamespace(gomock.Any()).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNamespace(gomock.Any(), validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNetworkAttachmentDefinition(gomock.Any(), validNetworkName, validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetStorageClass(gomock.Any(), validStorageClass).Return(nil, nil).AnyTimes()
			},
		},
		{
			name:           "invalid empty platform",
			edit:           func(ic *types.InstallConfig) { ic.Platform.Kubevirt = nil },
			expectedError:  true,
			expectedErrMsg: "platform.kubevirt: Required value: validation requires a Engine platform configuration",
		},
		{
			name:             "invalid client builder error",
			expectedError:    true,
			expectedErrMsg:   fmt.Sprintf("platform.kubevirt.InfraClusterReachable: Invalid value: \"InfraCluster\": failed to create InfraCluster client with error: test"),
			clientBuilderErr: errors.New("test"),
		},
		{
			name:           "invalid cluster unreachable",
			edit:           nil,
			expectedError:  true,
			expectedErrMsg: fmt.Sprintf("platform.kubevirt.InfraClusterReachable: Invalid value: \"InfraCluster\": failed to access to InfraCluster with error: test"),
			expectClient: func(kubevirtClient *mock.MockClient) {
				kubevirtClient.EXPECT().ListNamespace(gomock.Any()).Return(nil, fmt.Errorf("test")).AnyTimes()
			},
		},
		{
			name:           "invalid namespace",
			edit:           func(ic *types.InstallConfig) { ic.Platform.Kubevirt.Namespace = invalidNamespace },
			expectedError:  true,
			expectedErrMsg: "platform.kubevirt.NamespaceExistsInInfraCluster: Invalid value: \"invalid-namespace\": failed to get namespace invalid-namespace from InfraCluster, with error: test",
			expectClient: func(kubevirtClient *mock.MockClient) {
				kubevirtClient.EXPECT().ListNamespace(gomock.Any()).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNamespace(gomock.Any(), invalidNamespace).Return(nil, fmt.Errorf("test")).AnyTimes()
				kubevirtClient.EXPECT().GetStorageClass(gomock.Any(), validStorageClass).Return(nil, nil).AnyTimes()
			},
		},
		{
			name:           "invalid storage class",
			edit:           func(ic *types.InstallConfig) { ic.Platform.Kubevirt.StorageClass = invalidStorageClass },
			expectedError:  true,
			expectedErrMsg: "platform.kubevirt.StorageClassExistsInInfraCluster: Invalid value: \"invalid-storage-class\": failed to get storageClass invalid-storage-class from InfraCluster, with error: test",
			expectClient: func(kubevirtClient *mock.MockClient) {
				kubevirtClient.EXPECT().ListNamespace(gomock.Any()).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNamespace(gomock.Any(), validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNetworkAttachmentDefinition(gomock.Any(), validNetworkName, validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetStorageClass(gomock.Any(), invalidStorageClass).Return(nil, fmt.Errorf("test")).AnyTimes()
			},
		},
		{
			name:           "invalid network name",
			edit:           func(ic *types.InstallConfig) { ic.Platform.Kubevirt.NetworkName = invalidNetworkName },
			expectedError:  true,
			expectedErrMsg: "platform.kubevirt.NetworkAttachmentDefinitionExistsInInfraCluster: Invalid value: \"invalid-network-name\": failed to get network-attachment-definition invalid-network-name from InfraCluster, with error: test",
			expectClient: func(kubevirtClient *mock.MockClient) {
				kubevirtClient.EXPECT().ListNamespace(gomock.Any()).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNamespace(gomock.Any(), validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNetworkAttachmentDefinition(gomock.Any(), invalidNetworkName, validNamespace).Return(nil, fmt.Errorf("test")).AnyTimes()
				kubevirtClient.EXPECT().GetStorageClass(gomock.Any(), validStorageClass).Return(nil, nil).AnyTimes()
			},
		},
		{
			name:           "invalid APIVIP",
			edit:           func(ic *types.InstallConfig) { ic.Platform.Kubevirt.APIVIP = invalidAPIVIP },
			expectedError:  true,
			expectedErrMsg: "platform.kubevirt.APIVIP: Invalid value: \"invalid-api-vip\": \"invalid-api-vip\" is not a valid IP",
			expectClient: func(kubevirtClient *mock.MockClient) {
				kubevirtClient.EXPECT().ListNamespace(gomock.Any()).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNamespace(gomock.Any(), validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNetworkAttachmentDefinition(gomock.Any(), validNetworkName, validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetStorageClass(gomock.Any(), validStorageClass).Return(nil, nil).AnyTimes()
			},
		},
		{
			name:           "invalid IngressVIP",
			edit:           func(ic *types.InstallConfig) { ic.Platform.Kubevirt.IngressVIP = invalidIngressVIP },
			expectedError:  true,
			expectedErrMsg: "platform.kubevirt.IngressVIP: Invalid value: \"invalid-ingress-vip\": \"invalid-ingress-vip\" is not a valid IP",
			expectClient: func(kubevirtClient *mock.MockClient) {
				kubevirtClient.EXPECT().ListNamespace(gomock.Any()).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNamespace(gomock.Any(), validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNetworkAttachmentDefinition(gomock.Any(), validNetworkName, validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetStorageClass(gomock.Any(), validStorageClass).Return(nil, nil).AnyTimes()
			},
		},
		{
			name: "invalid VIPs not in CIDR",
			edit: func(ic *types.InstallConfig) {
				ic.Networking.MachineNetwork[0].CIDR = *ipnet.MustParseCIDR(invalidMachineCIDR)
			},
			expectedError:  true,
			expectedErrMsg: "platform.kubevirt.IPsInCIDR",
			expectClient: func(kubevirtClient *mock.MockClient) {
				kubevirtClient.EXPECT().ListNamespace(gomock.Any()).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNamespace(gomock.Any(), validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetNetworkAttachmentDefinition(gomock.Any(), validNetworkName, validNamespace).Return(nil, nil).AnyTimes()
				kubevirtClient.EXPECT().GetStorageClass(gomock.Any(), validStorageClass).Return(nil, nil).AnyTimes()
			},
		},
	}
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installConfig := validInstallConfig()
			if tc.edit != nil {
				tc.edit(installConfig)
			}

			kubevirtClient := mock.NewMockClient(mockCtrl)
			if tc.expectClient != nil {
				tc.expectClient(kubevirtClient)
			}

			errs := Validate(installConfig, func() (Client, error) { return kubevirtClient, tc.clientBuilderErr })
			if tc.expectedError {
				assert.Regexp(t, tc.expectedErrMsg, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}
