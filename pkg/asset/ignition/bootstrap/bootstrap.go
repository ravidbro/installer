package bootstrap

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"path"

	"github.com/containers/image/pkg/sysregistriesv2"
	igntypes "github.com/coreos/ignition/v2/config/v3_1/types"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vincent-petithory/dataurl"

	"github.com/openshift/installer/data"
	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/ignition"
	"github.com/openshift/installer/pkg/asset/ignition/bootstrap/baremetal"
	"github.com/openshift/installer/pkg/asset/ignition/bootstrap/vsphere"
	"github.com/openshift/installer/pkg/asset/installconfig"
	"github.com/openshift/installer/pkg/asset/kubeconfig"
	"github.com/openshift/installer/pkg/asset/machines"
	"github.com/openshift/installer/pkg/asset/manifests"
	"github.com/openshift/installer/pkg/asset/releaseimage"
	"github.com/openshift/installer/pkg/asset/rhcos"
	"github.com/openshift/installer/pkg/asset/tls"
	"github.com/openshift/installer/pkg/types"
	baremetaltypes "github.com/openshift/installer/pkg/types/baremetal"
	vspheretypes "github.com/openshift/installer/pkg/types/vsphere"
)

const (
	rootDir              = "/opt/openshift"
	bootstrapIgnFilename = "bootstrap.ign"
)

// bootstrapTemplateData is the data to use to replace values in bootstrap
// template files.
type bootstrapTemplateData struct {
	AdditionalTrustBundle string
	FIPS                  bool
	EtcdCluster           string
	PullSecret            string
	ReleaseImage          string
	ClusterProfile        string
	Proxy                 *configv1.ProxyStatus
	Registries            []sysregistriesv2.Registry
	BootImage             string
	PlatformData          platformTemplateData
	BootstrapInPlace      *types.BootstrapInPlace
}

// platformTemplateData is the data to use to replace values in bootstrap
// template files that are specific to one platform.
type platformTemplateData struct {
	BareMetal *baremetal.TemplateData
	VSphere   *vsphere.TemplateData
}

// Bootstrap is an asset that generates the ignition config for bootstrap nodes.
type Bootstrap struct {
	Config           *igntypes.Config
	File             *asset.File
	bootstrapInPlace bool
}

var _ asset.WritableAsset = (*Bootstrap)(nil)

// Dependencies returns the assets on which the Bootstrap asset depends.
func (a *Bootstrap) Dependencies() []asset.Asset {
	return []asset.Asset{
		&baremetal.IronicCreds{},
		&installconfig.InstallConfig{},
		&kubeconfig.AdminInternalClient{},
		&kubeconfig.Kubelet{},
		&kubeconfig.LoopbackClient{},
		&manifests.Manifests{},
		&manifests.Proxy{},
		&tls.AdminKubeConfigCABundle{},
		&tls.AggregatorCA{},
		&tls.AggregatorCABundle{},
		&tls.AggregatorClientCertKey{},
		&tls.AggregatorSignerCertKey{},
		&tls.APIServerProxyCertKey{},
		&tls.BootstrapSSHKeyPair{},
		&tls.BoundSASigningKey{},
		&tls.CloudProviderCABundle{},
		&tls.EtcdCABundle{},
		&tls.EtcdMetricCABundle{},
		&tls.EtcdMetricSignerCertKey{},
		&tls.EtcdMetricSignerClientCertKey{},
		&tls.EtcdSignerCertKey{},
		&tls.EtcdSignerClientCertKey{},
		&tls.JournalCertKey{},
		&tls.KubeAPIServerLBCABundle{},
		&tls.KubeAPIServerExternalLBServerCertKey{},
		&tls.KubeAPIServerInternalLBServerCertKey{},
		&tls.KubeAPIServerLBSignerCertKey{},
		&tls.KubeAPIServerLocalhostCABundle{},
		&tls.KubeAPIServerLocalhostServerCertKey{},
		&tls.KubeAPIServerLocalhostSignerCertKey{},
		&tls.KubeAPIServerServiceNetworkCABundle{},
		&tls.KubeAPIServerServiceNetworkServerCertKey{},
		&tls.KubeAPIServerServiceNetworkSignerCertKey{},
		&tls.KubeAPIServerCompleteCABundle{},
		&tls.KubeAPIServerCompleteClientCABundle{},
		&tls.KubeAPIServerToKubeletCABundle{},
		&tls.KubeAPIServerToKubeletClientCertKey{},
		&tls.KubeAPIServerToKubeletSignerCertKey{},
		&tls.KubeControlPlaneCABundle{},
		&tls.KubeControlPlaneKubeControllerManagerClientCertKey{},
		&tls.KubeControlPlaneKubeSchedulerClientCertKey{},
		&tls.KubeControlPlaneSignerCertKey{},
		&tls.KubeletBootstrapCABundle{},
		&tls.KubeletClientCABundle{},
		&tls.KubeletClientCertKey{},
		&tls.KubeletCSRSignerCertKey{},
		&tls.KubeletServingCABundle{},
		&tls.MCSCertKey{},
		&tls.RootCA{},
		&tls.ServiceAccountKeyPair{},
		&releaseimage.Image{},
		new(rhcos.Image),
	}
}

// Generate generates the ignition config for the Bootstrap asset.
func (a *Bootstrap) Generate(dependencies asset.Parents) error {
	installConfig := &installconfig.InstallConfig{}
	proxy := &manifests.Proxy{}
	releaseImage := &releaseimage.Image{}
	rhcosImage := new(rhcos.Image)
	bootstrapSSHKeyPair := &tls.BootstrapSSHKeyPair{}
	ironicCreds := &baremetal.IronicCreds{}
	dependencies.Get(installConfig, proxy, releaseImage, rhcosImage, bootstrapSSHKeyPair, ironicCreds)

	templateData, err := a.getTemplateData(installConfig.Config, releaseImage.PullSpec, installConfig.Config.ImageContentSources, proxy.Config, rhcosImage, ironicCreds)

	if err != nil {
		return errors.Wrap(err, "failed to get bootstrap templates")
	}

	a.Config = &igntypes.Config{
		Ignition: igntypes.Ignition{
			Version: igntypes.MaxVersion.String(),
		},
	}

	err = ignition.AddStorageFiles(a.Config, "/", "bootstrap/files", templateData)
	if err != nil {
		return err
	}

	enabled := map[string]struct{}{
		"progress.service":                {},
		"kubelet.service":                 {},
		"chown-gatewayd-key.service":      {},
		"systemd-journal-gatewayd.socket": {},
		"approve-csr.service":             {},
		// baremetal & openstack platform services
		"keepalived.service":        {},
		"coredns.service":           {},
		"ironic.service":            {},
		"master-bmh-update.service": {},
	}

	err = ignition.AddSystemdUnits(a.Config, "bootstrap/systemd/units", templateData, enabled)
	if err != nil {
		return err
	}

	// Check for optional platform specific files/units
	platform := installConfig.Config.Platform.Name()
	platformFilePath := fmt.Sprintf("bootstrap/%s/files", platform)
	directory, err := data.Assets.Open(platformFilePath)
	if err == nil {
		directory.Close()
		err = ignition.AddStorageFiles(a.Config, "/", platformFilePath, templateData)
		if err != nil {
			return err
		}
	}

	platformUnitPath := fmt.Sprintf("bootstrap/%s/systemd/units", platform)
	directory, err = data.Assets.Open(platformUnitPath)
	if err == nil {
		directory.Close()
		err = ignition.AddSystemdUnits(a.Config, platformUnitPath, templateData, enabled)
		if err != nil {
			return err
		}
	}

	for _, asset := range []asset.WritableAsset{
		&manifests.Manifests{},
		&manifests.Openshift{},
		&machines.Master{},
		&machines.Worker{},
	} {
		dependencies.Get(asset)

		// Replace files that already exist in the slice with ones added later, otherwise append them
		for _, file := range ignition.FilesFromAsset(rootDir, "root", 0644, asset) {
			a.Config.Storage.Files = ignition.ReplaceOrAppend(a.Config.Storage.Files, file)
		}
	}
	ignition.AddParentFiles(a.Config, dependencies, "/", "root", 0600, []asset.WritableAsset{
		&kubeconfig.AdminInternalClient{},
		&kubeconfig.Kubelet{},
		&kubeconfig.LoopbackClient{},
		&tls.AdminKubeConfigCABundle{},
		&tls.AggregatorCA{},
		&tls.AggregatorCABundle{},
		&tls.AggregatorClientCertKey{},
		&tls.AggregatorSignerCertKey{},
		&tls.APIServerProxyCertKey{},
		&tls.EtcdCABundle{},
		&tls.EtcdMetricCABundle{},
		&tls.EtcdMetricSignerCertKey{},
		&tls.EtcdMetricSignerClientCertKey{},
		&tls.EtcdSignerCertKey{},
		&tls.EtcdSignerClientCertKey{},
		&tls.KubeAPIServerLBCABundle{},
		&tls.KubeAPIServerExternalLBServerCertKey{},
		&tls.KubeAPIServerInternalLBServerCertKey{},
		&tls.KubeAPIServerLBSignerCertKey{},
		&tls.KubeAPIServerLocalhostCABundle{},
		&tls.KubeAPIServerLocalhostServerCertKey{},
		&tls.KubeAPIServerLocalhostSignerCertKey{},
		&tls.KubeAPIServerServiceNetworkCABundle{},
		&tls.KubeAPIServerServiceNetworkServerCertKey{},
		&tls.KubeAPIServerServiceNetworkSignerCertKey{},
		&tls.KubeAPIServerCompleteCABundle{},
		&tls.KubeAPIServerCompleteClientCABundle{},
		&tls.KubeAPIServerToKubeletCABundle{},
		&tls.KubeAPIServerToKubeletClientCertKey{},
		&tls.KubeAPIServerToKubeletSignerCertKey{},
		&tls.KubeControlPlaneCABundle{},
		&tls.KubeControlPlaneKubeControllerManagerClientCertKey{},
		&tls.KubeControlPlaneKubeSchedulerClientCertKey{},
		&tls.KubeControlPlaneSignerCertKey{},
		&tls.KubeletBootstrapCABundle{},
		&tls.KubeletClientCABundle{},
		&tls.KubeletClientCertKey{},
		&tls.KubeletCSRSignerCertKey{},
		&tls.KubeletServingCABundle{},
		&tls.MCSCertKey{},
		&tls.ServiceAccountKeyPair{},
		&tls.JournalCertKey{},
	})
	rootCA := &tls.RootCA{}
	dependencies.Get(rootCA)
	a.Config.Storage.Files = ignition.ReplaceOrAppend(a.Config.Storage.Files, ignition.FileFromBytes(filepath.Join(rootDir, rootCA.CertFile().Filename), "root", 0644, rootCA.Cert()))

	a.Config.Passwd.Users = append(
		a.Config.Passwd.Users,
		igntypes.PasswdUser{Name: "core", SSHAuthorizedKeys: []igntypes.SSHAuthorizedKey{
			igntypes.SSHAuthorizedKey(installConfig.Config.SSHKey),
			igntypes.SSHAuthorizedKey(string(bootstrapSSHKeyPair.Public())),
		}},
	)

	data, err := ignition.Marshal(a.Config)
	if err != nil {
		return errors.Wrap(err, "failed to Marshal Ignition config")
	}
	fileName := bootstrapIgnFilename
	a.File = &asset.File{
		Filename: fileName,
		Data:     data,
	}

	return nil
}

// Name returns the human-friendly name of the asset.
func (a *Bootstrap) Name() string {
	return "Bootstrap Ignition Config"
}

// Files returns the files generated by the asset.
func (a *Bootstrap) Files() []*asset.File {
	if a.File != nil {
		return []*asset.File{a.File}
	}
	return []*asset.File{}
}

// getTemplateData returns the data to use to execute bootstrap templates.
func (a *Bootstrap) getTemplateData(installConfig *types.InstallConfig, releaseImage string, imageSources []types.ImageContentSource, proxy *configv1.Proxy, rhcosImage *rhcos.Image, ironicCreds *baremetal.IronicCreds) (*bootstrapTemplateData, error) {
	etcdEndpoints := make([]string, *installConfig.ControlPlane.Replicas)

	for i := range etcdEndpoints {
		etcdEndpoints[i] = fmt.Sprintf("https://etcd-%d.%s:2379", i, installConfig.ClusterDomain())
	}

	registries := []sysregistriesv2.Registry{}
	for _, group := range mergedMirrorSets(imageSources) {
		if len(group.Mirrors) == 0 {
			continue
		}

		registry := sysregistriesv2.Registry{}
		registry.Endpoint.Location = group.Source
		registry.MirrorByDigestOnly = true
		for _, mirror := range group.Mirrors {
			registry.Mirrors = append(registry.Mirrors, sysregistriesv2.Endpoint{Location: mirror})
		}
		registries = append(registries, registry)
	}

	// Generate platform-specific bootstrap data
	var platformData platformTemplateData

	switch installConfig.Platform.Name() {
	case baremetaltypes.Name:
		platformData.BareMetal = baremetal.GetTemplateData(installConfig.Platform.BareMetal, installConfig.MachineNetwork, ironicCreds.Username, ironicCreds.Password)
	case vspheretypes.Name:
		platformData.VSphere = vsphere.GetTemplateData(installConfig.Platform.VSphere)
	}

	// Set cluster profile
	clusterProfile := ""
	if cp := os.Getenv("OPENSHIFT_INSTALL_EXPERIMENTAL_CLUSTER_PROFILE"); cp != "" {
		logrus.Warnf("Found override for Cluster Profile: %q", cp)
		clusterProfile = cp
	}
	var bootstrapInPlaceConfig *types.BootstrapInPlace
	if a.bootstrapInPlace {
		bootstrapInPlaceConfig = installConfig.BootstrapInPlace
	}
	return &bootstrapTemplateData{
		AdditionalTrustBundle: installConfig.AdditionalTrustBundle,
		FIPS:                  installConfig.FIPS,
		PullSecret:            installConfig.PullSecret,
		ReleaseImage:          releaseImage,
		EtcdCluster:           strings.Join(etcdEndpoints, ","),
		Proxy:                 &proxy.Status,
		Registries:            registries,
		BootImage:             string(*rhcosImage),
		PlatformData:          platformData,
		ClusterProfile:        clusterProfile,
		BootstrapInPlace:      bootstrapInPlaceConfig,
	}, nil
}

// Load returns the bootstrap ignition from disk.
func (a *Bootstrap) Load(f asset.FileFetcher) (found bool, err error) {
	file, err := f.FetchByName(bootstrapIgnFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	config := &igntypes.Config{}
	if err := json.Unmarshal(file.Data, config); err != nil {
		return false, errors.Wrapf(err, "failed to unmarshal %s", bootstrapIgnFilename)
	}

	a.File, a.Config = file, config
	warnIfCertificatesExpired(a.Config)
	return true, nil
}

// warnIfCertificatesExpired checks for expired certificates and warns if so
func warnIfCertificatesExpired(config *igntypes.Config) {
	expiredCerts := 0
	for _, file := range config.Storage.Files {
		if filepath.Ext(file.Path) == ".crt" && file.Contents.Source != nil {
			fileName := path.Base(file.Path)
			decoded, err := dataurl.DecodeString(*file.Contents.Source)
			if err != nil {
				logrus.Debugf("Unable to decode certificate %s: %s", fileName, err.Error())
				continue
			}
			data := decoded.Data
			for {
				block, rest := pem.Decode(data)
				if block == nil {
					break
				}

				cert, err := x509.ParseCertificate(block.Bytes)
				if err == nil {
					if time.Now().UTC().After(cert.NotAfter) {
						logrus.Warnf("Bootstrap Ignition-Config Certificate %s expired at %s.", path.Base(file.Path), cert.NotAfter.Format(time.RFC3339))
						expiredCerts++
					}
				} else {
					logrus.Debugf("Unable to parse certificate %s: %s", fileName, err.Error())
					break
				}

				data = rest
			}
		}
	}

	if expiredCerts > 0 {
		logrus.Warnf("Bootstrap Ignition-Config: %d certificates expired. Installation attempts with the created Ignition-Configs will possibly fail.", expiredCerts)
	}
}
