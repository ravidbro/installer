package defaults

import (
	"github.com/openshift/installer/pkg/types/kubevirt"
)

// SetPlatformDefaults sets the defaults for the platform.
func SetPlatformDefaults(p *kubevirt.Platform) {
	// No default values to set - all values are mandatory or empty value is the default
	// For example StorageClass and PersistentVolumeAccessMode are optional, but empty string is valid default value
}
