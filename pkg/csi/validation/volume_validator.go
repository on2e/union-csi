package validation

import (
	"fmt"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	sets "k8s.io/apimachinery/pkg/util/sets"
)

type VolumeValidator interface {
	GetVolumeCapabilityModes() []csi.VolumeCapability_AccessMode_Mode
	HasVolumeCapabilityMode(csi.VolumeCapability_AccessMode_Mode) bool
}

// Exclude csi.VolumeCapability_AccessMode_UNKNOWN.
var validVolumeCapTypes = sets.New[csi.VolumeCapability_AccessMode_Mode](
	csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
	csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
	csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
	csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER,
	csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
	csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER,
	csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER,
)

type volumeValidator struct {
	volumeCapModesSet sets.Set[csi.VolumeCapability_AccessMode_Mode]
}

var _ VolumeValidator = &volumeValidator{}

func NewVolumeValidator(volumeCapModes []csi.VolumeCapability_AccessMode_Mode) (*volumeValidator, error) {
	v := &volumeValidator{}

	for _, k := range volumeCapModes {
		if k == csi.VolumeCapability_AccessMode_UNKNOWN {
			return nil, fmt.Errorf("volume capability is %s", csi.VolumeCapability_AccessMode_UNKNOWN)
		}
		if !validVolumeCapTypes.Has(k) {
			return nil, fmt.Errorf("unsupported volume capability: %v. Supported values: %v", k, sets.List[csi.VolumeCapability_AccessMode_Mode](validVolumeCapTypes))
		}
	}

	v.volumeCapModesSet = sets.New[csi.VolumeCapability_AccessMode_Mode](volumeCapModes...)

	return v, nil
}

func (v *volumeValidator) GetVolumeCapabilityModes() []csi.VolumeCapability_AccessMode_Mode {
	return sets.List[csi.VolumeCapability_AccessMode_Mode](v.volumeCapModesSet)
}

func (v *volumeValidator) HasVolumeCapabilityMode(k csi.VolumeCapability_AccessMode_Mode) bool {
	return v.volumeCapModesSet.Has(k)
}
