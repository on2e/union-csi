package validation

import (
	"fmt"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	sets "k8s.io/apimachinery/pkg/util/sets"
)

type IdentityValidator interface {
	GetPluginCapabilities() []*csi.PluginCapability
	GetPluginCapabilityTypes() []csi.PluginCapability_Service_Type
	HasPluginCapabilityType(csi.PluginCapability_Service_Type) bool
}

// Exclude csi.PluginCapability_Service_UNKNOWN.
var validPluginCapTypes = sets.New[csi.PluginCapability_Service_Type](
	csi.PluginCapability_Service_CONTROLLER_SERVICE,
	csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
	csi.PluginCapability_Service_GROUP_CONTROLLER_SERVICE,
)

type identityValidator struct {
	pluginCapTypesSet sets.Set[csi.PluginCapability_Service_Type]
	pluginCaps        []*csi.PluginCapability
}

var _ IdentityValidator = &identityValidator{}

func NewIdentityValidator(pluginCapTypes []csi.PluginCapability_Service_Type) (*identityValidator, error) {
	v := &identityValidator{}

	for _, k := range pluginCapTypes {
		if k == csi.PluginCapability_Service_UNKNOWN {
			return nil, fmt.Errorf("plugin capability is %s", csi.PluginCapability_Service_UNKNOWN)
		}
		if !validPluginCapTypes.Has(k) {
			return nil, fmt.Errorf("unsupported plugin capability: %v. Supported values: %v", k, sets.List[csi.PluginCapability_Service_Type](validPluginCapTypes))
		}
	}

	v.pluginCapTypesSet = sets.New[csi.PluginCapability_Service_Type](pluginCapTypes...)

	for k := range v.pluginCapTypesSet {
		cap := &csi.PluginCapability{
			Type: &csi.PluginCapability_Service_{
				Service: &csi.PluginCapability_Service{
					Type: k,
				},
			},
		}
		v.pluginCaps = append(v.pluginCaps, cap)
	}

	return v, nil
}

func (v *identityValidator) GetPluginCapabilities() []*csi.PluginCapability {
	return v.pluginCaps
}

func (v *identityValidator) GetPluginCapabilityTypes() []csi.PluginCapability_Service_Type {
	return sets.List[csi.PluginCapability_Service_Type](v.pluginCapTypesSet)
}

func (v *identityValidator) HasPluginCapabilityType(k csi.PluginCapability_Service_Type) bool {
	return v.pluginCapTypesSet.Has(k)
}
