package aio

import (
	"encoding/json"
	"fmt"
	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/openshift/installer/pkg/asset/kubeconfig"
	"github.com/openshift/installer/pkg/asset/manifests"
	"github.com/openshift/installer/pkg/asset/rhcos"
	"net"
	"os"
	"path/filepath"

	igntypes "github.com/coreos/ignition/v2/config/v3_1/types"
	"github.com/pkg/errors"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/ignition"
	"github.com/openshift/installer/pkg/asset/installconfig"
	"github.com/openshift/installer/pkg/asset/releaseimage"
	"github.com/openshift/installer/pkg/asset/tls"
	"github.com/openshift/installer/pkg/types"
)

const (
	aioIgnFilename = "aio.ign"
	rootDir        = "/opt/openshift"
)

// aioTemplateData is the data to use to replace values in all-in-one
// template files.
type aioTemplateData struct {
	ReleaseImage   string
	ClusterDomain  string
	EtcdCluster    string
	ClusterDNSIP   string
	PullSecret     string
	ClusterNetwork string
}

// AIO is an asset that generates the ignition config for an all-in-one node.
type AIO struct {
	Config *igntypes.Config
	File   *asset.File
}

var _ asset.WritableAsset = (*AIO)(nil)

// Dependencies returns the assets on which the AIO asset depends.
func (a *AIO) Dependencies() []asset.Asset {
	return []asset.Asset{
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

// Generate generates the ignition config for the all-in-one asset.
func (a *AIO) Generate(dependencies asset.Parents) error {
	installConfig := &installconfig.InstallConfig{}
	releaseImage := &releaseimage.Image{}
	dependencies.Get(installConfig, releaseImage)

	templateData, err := a.getTemplateData(installConfig.Config, releaseImage.PullSpec)

	if err != nil {
		return errors.Wrap(err, "failed to get aio templates")
	}

	a.Config = &igntypes.Config{
		Ignition: igntypes.Ignition{
			Version: igntypes.MaxVersion.String(),
		},
	}

	err = ignition.AddStorageFiles(a.Config, "/", "aio/files", templateData)
	if err != nil {
		return err
	}

	a.Config.Storage.Files = ignition.ReplaceOrAppend(a.Config.Storage.Files,
		ignition.FileFromURL("/usr/local/bin/kubelet", "root", 0755,
			"https://storage.googleapis.com/kubernetes-release/release/v1.20.4/bin/linux/amd64/kubelet"))

	a.Config.Storage.Files = ignition.ReplaceOrAppend(a.Config.Storage.Files,
		ignition.FileFromURL("/usr/local/bin/kubectl", "root", 0755,
			"https://storage.googleapis.com/kubernetes-release/release/v1.20.4/bin/linux/amd64/kubectl"))

	a.Config.Storage.Files = ignition.ReplaceOrAppend(a.Config.Storage.Files,
		ignition.FileFromURL("/usr/local/bin/kube-proxy", "root", 0755,
			"https://storage.googleapis.com/kubernetes-release/release/v1.20.4/bin/linux/amd64/kube-proxy"))

	enabled := map[string]struct{}{
		"kubelet.service":     {},
		"kube-proxy.service":  {},
		"aiokube.service":     {},
		"approve-csr.service": {},
	}

	err = ignition.AddSystemdUnits(a.Config, "aio/systemd/units", templateData, enabled)
	if err != nil {
		return err
	}
	a.addParentFiles(dependencies)

	a.Config.Passwd.Users = append(
		a.Config.Passwd.Users,
		igntypes.PasswdUser{Name: "core", SSHAuthorizedKeys: []igntypes.SSHAuthorizedKey{
			igntypes.SSHAuthorizedKey(installConfig.Config.SSHKey),
		}},
	)

	data, err := json.Marshal(a.Config)
	if err != nil {
		return errors.Wrap(err, "failed to Marshal Ignition config")
	}
	a.File = &asset.File{
		Filename: aioIgnFilename,
		Data:     data,
	}

	return nil
}

// Name returns the human-friendly name of the asset.
func (a *AIO) Name() string {
	return "All-in-one Ignition Config"
}

// Files returns the files generated by the asset.
func (a *AIO) Files() []*asset.File {
	if a.File != nil {
		return []*asset.File{a.File}
	}
	return []*asset.File{}
}

func (a *AIO) addParentFiles(dependencies asset.Parents) {
	// TODO: clean this up
	// These files are all added with mode 0644, i.e. readable
	// by all processes on the system.
	ignition.AddParentFiles(a.Config, dependencies, rootDir, "root", 0644, []asset.WritableAsset{
		&manifests.Manifests{},
	})

	// These files are all added with mode 0600; use for secret keys and the like.
	ignition.AddParentFiles(a.Config, dependencies, rootDir, "root", 0600, []asset.WritableAsset{
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
}

// getTemplateData returns the data to use to execute all-in-one templates.
func (a *AIO) getTemplateData(installConfig *types.InstallConfig, releaseImage string) (*aioTemplateData, error) {
	if *installConfig.ControlPlane.Replicas != 1 {
		return nil, fmt.Errorf("All-in-one configurations must use a single control plane replica")
	}

	for _, worker := range installConfig.Compute {
		if worker.Replicas != nil && *worker.Replicas != 0 {
			return nil, fmt.Errorf("All-in-one configurations do not support compute replicas")
		}
	}
	dnsIp, err := clusterDNSIP(installConfig.Networking.ServiceNetwork[0].String())
	if err != nil {
		return nil, err
	}
	return &aioTemplateData{
		ReleaseImage:  releaseImage,
		ClusterDomain: installConfig.ClusterDomain(),
		EtcdCluster:   fmt.Sprintf("https://etcd-0.%s:2379", installConfig.ClusterDomain()),
		ClusterDNSIP:  dnsIp,
		PullSecret: installConfig.PullSecret,
		ClusterNetwork: installConfig.Networking.ClusterNetwork[0].CIDR.String(),

	}, nil
}

// Load returns the all-in-one ignition from disk.
func (a *AIO) Load(f asset.FileFetcher) (found bool, err error) {
	file, err := f.FetchByName(aioIgnFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	config := &igntypes.Config{}
	if err := json.Unmarshal(file.Data, config); err != nil {
		return false, errors.Wrapf(err, "failed to unmarshal %s", aioIgnFilename)
	}

	a.File, a.Config = file, config
	return true, nil
}

func clusterDNSIP(iprange string) (string, error) {
	_, network, err := net.ParseCIDR(iprange)
	if err != nil {
		return "", err
	}
	ip, err := cidr.Host(network, 10)
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}
