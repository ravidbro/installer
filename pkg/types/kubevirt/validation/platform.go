package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types/kubevirt"
	"github.com/openshift/installer/pkg/validate"
)

// ValidatePlatform checks that the specified platform is valid.
func ValidatePlatform(p *kubevirt.Platform, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if p.Namespace == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Infra Cluster Namespace"), p.Namespace, "Infra Cluster Namespace can't be empty"))
	}

	if p.NetworkName == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("NetworkName"), p.NetworkName, "NetworkName can't be empty"))
	}

	if err := validate.IP(p.APIVIP); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("APIVIP"), p.APIVIP, err.Error()))
	}

	if err := validate.IP(p.IngressVIP); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("IngressVIP"), p.IngressVIP, err.Error()))
	}

	return allErrs
}
