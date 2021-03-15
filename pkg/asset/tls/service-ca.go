package tls

import (
	"crypto/x509"
	"crypto/x509/pkix"

	"github.com/openshift/installer/pkg/asset"
)

// ServiceCASignerCertKey is a key/cert pair for service-ca.
type ServiceCASignerCertKey struct {
	SelfSignedCertKey
}

var _ asset.WritableAsset = (*ServiceCASignerCertKey)(nil)

// Dependencies returns the dependency of the root-ca, which is empty.
func (c *ServiceCASignerCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Generate generates the root-ca key and cert pair.
func (c *ServiceCASignerCertKey) Generate(parents asset.Parents) error {
	cfg := &CertCfg{
		Subject:   pkix.Name{CommonName: "service-ca-signer", OrganizationalUnit: []string{"openshift"}},
		KeyUsages: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		Validity:  ValidityTenYears,
		IsCA:      true,
	}

	return c.SelfSignedCertKey.Generate(cfg, "service-ca-signer")
}

// Name returns the human-friendly name of the asset.
func (c *ServiceCASignerCertKey) Name() string {
	return "Certificate (service-ca=signer)"
}

// ServiceCABundle is the asset the generates the service-ca-bundle,
// which contains all the individual client CAs.
type ServiceCABundle struct {
	CertBundle
}

var _ asset.Asset = (*ServiceCABundle)(nil)

// Dependencies returns the dependency of the cert bundle.
func (a *ServiceCABundle) Dependencies() []asset.Asset {
	return []asset.Asset{
		&ServiceCASignerCertKey{},
	}
}

// Generate generates the cert bundle based on its dependencies.
func (a *ServiceCABundle) Generate(deps asset.Parents) error {
	var certs []CertInterface
	for _, asset := range a.Dependencies() {
		deps.Get(asset)
		certs = append(certs, asset.(CertInterface))
	}
	return a.CertBundle.Generate("service-ca-bundle", certs...)
}

// Name returns the human-friendly name of the asset.
func (a *ServiceCABundle) Name() string {
	return "Certificate (service-ca-bundle)"
}
