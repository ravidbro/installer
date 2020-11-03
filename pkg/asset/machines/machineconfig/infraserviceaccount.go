package machineconfig

import (
	"encoding/base64"
	"fmt"
	"github.com/coreos/ignition/v2/config/util"
	"strings"

	igntypes "github.com/coreos/ignition/v2/config/v3_1/types"
	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/installer/pkg/asset/ignition"
)

const (
	mountPath             = "/var/mnt/serviceaccount"
	credsSecretName       = "kubevirt-credentials"
	credSecretScriptPath  = "/root/creds_secret.sh"
	credSecretServiceName = "infraCredsSecret"
)

var (
	mountServiceContent = fmt.Sprintf(
		"[Unit]\n"+
			"Before=local-fs.target\n"+
			"[Mount]\n"+
			"What=/dev/disk/by-id/virtio-SERVICEACCOUNT\n"+
			"Where=%s\n"+
			"[Install]\n"+
			"WantedBy=local-fs.target\n", mountPath)
	updateSecretScriptContent = fmt.Sprintf(
		"set -e\n"+
			"oc login https://api.hekio04-0.kubevirt.org:6443 --certificate-authority=%s/ca.crt --token=`cat %s/token`\n"+
			"sed -i \"s/certificate-authority: %s/ca.crt/certificate-authority-data: \\b`cat %s/ca.crt`\\b/g\" /root/.kube/config\n"+
			"oc --kubeconfig=/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig -n kube-system delete secret %s || true\n"+
			"oc --kubeconfig=/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig -n kube-system create secret generic %s --from-file=kubeconfig=/root/.kube/config\n"+
			"rm /root/.kube/config\n",
		mountPath, mountPath, strings.ReplaceAll(mountPath, "/", "\\/"), mountPath, credsSecretName, credsSecretName)
	updateSecretServiceContent = fmt.Sprintf(
		"[Service]\n"+
			"User=0\n"+
			"Type=oneshot\n"+
			"ExecStart=/bin/bash %s\n", credSecretScriptPath)
	updateSecretTimerContent = fmt.Sprintf(
		"[Timer]\n" +
			"OnUnitActiveSec=60s\n" +
			"OnBootSec=60s\n" +
			"[Install]\n" +
			"WantedBy=timers.target\n")
)

// ForInfraServiceAccount creates the MachineConfig to manage the serviceAccount secret for the infra cluster in the tenant cluster
// This is being used on KubeVirt platform
func ForInfraServiceAccount(role string) (*mcfgv1.MachineConfig, error) {
	ignConfig := igntypes.Config{
		Ignition: igntypes.Ignition{
			Version: igntypes.MaxVersion.String(),
		},
		Storage: igntypes.Storage{
			Files: []igntypes.File{{
				igntypes.Node{
					Path:      credSecretScriptPath,
					Overwrite: util.BoolToPtr(true),
				},
				igntypes.FileEmbedded1{
					Mode: util.IntToPtr(500),
					Contents: igntypes.Resource{
						Source: util.StrToPtr("data:text/plain;charset=utf-8;base64," + base64.URLEncoding.EncodeToString([]byte(updateSecretScriptContent))),
					},
				},
			}},
		},
		Systemd: igntypes.Systemd{
			Units: []igntypes.Unit{
				{
					Name:     fmt.Sprintf("%s.mount", strings.ReplaceAll(strings.TrimPrefix(mountPath, "/"), "/", "-")),
					Enabled:  util.BoolToPtr(true),
					Contents: &mountServiceContent,
				},
				{
					Name:     credSecretServiceName + ".service",
					Enabled:  util.BoolToPtr(true),
					Contents: &updateSecretServiceContent,
				},
				{
					Name:     credSecretServiceName + ".timer",
					Enabled:  util.BoolToPtr(true),
					Contents: &updateSecretTimerContent,
				},
			},
		},
	}

	rawExt, err := ignition.ConvertToRawExtension(ignConfig)
	if err != nil {
		return nil, err
	}

	return &mcfgv1.MachineConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: mcfgv1.SchemeGroupVersion.String(),
			Kind:       "MachineConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("99-%s-infra-service-account", role),
			Labels: map[string]string{
				"machineconfiguration.openshift.io/role": role,
			},
		},
		Spec: mcfgv1.MachineConfigSpec{
			Config: rawExt,
		},
	}, nil
}
