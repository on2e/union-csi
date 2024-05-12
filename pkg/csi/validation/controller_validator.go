package validation

import (
	"fmt"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	sets "k8s.io/apimachinery/pkg/util/sets"
)

type ControllerServerRequestValidator interface {
	CreateVolumeRequestValidate(*csi.CreateVolumeRequest) error
	DeleteVolumeRequestValidate(*csi.DeleteVolumeRequest) error
	ControllerPublishVolumeRequestValidate(*csi.ControllerPublishVolumeRequest) error
	ControllerUnpublishVolumeRequestValidate(*csi.ControllerUnpublishVolumeRequest) error
}

type ControllerValidator interface {
	ControllerServerRequestValidator

	GetControllerCapabilities() []*csi.ControllerServiceCapability
	GetControllerCapabilityTypes() []csi.ControllerServiceCapability_RPC_Type
	GetVolumeCapabilityModes() []csi.VolumeCapability_AccessMode_Mode
	HasControllerCapabilityType(csi.ControllerServiceCapability_RPC_Type) bool
	HasVolumeCapabilityMode(csi.VolumeCapability_AccessMode_Mode) bool
}

// Exclude csi.ControllerServiceCapability_RPC_UNKNOWN.
var validControllerCapTypes = sets.New[csi.ControllerServiceCapability_RPC_Type](
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
	csi.ControllerServiceCapability_RPC_GET_CAPACITY,
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
	csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
	csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
	csi.ControllerServiceCapability_RPC_PUBLISH_READONLY,
	csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	csi.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES,
	csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
	csi.ControllerServiceCapability_RPC_GET_VOLUME,
	csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
)

type controllerValidator struct {
	controllerCapTypesSet sets.Set[csi.ControllerServiceCapability_RPC_Type]
	controllerCaps        []*csi.ControllerServiceCapability

	volumeValidator VolumeValidator
}

var _ ControllerValidator = &controllerValidator{}

func NewControllerValidator(
	controllerCapTypes []csi.ControllerServiceCapability_RPC_Type,
	volumeCapModes []csi.VolumeCapability_AccessMode_Mode) (*controllerValidator, error) {
	v := &controllerValidator{}

	for _, k := range controllerCapTypes {
		if k == csi.ControllerServiceCapability_RPC_UNKNOWN {
			return nil, fmt.Errorf("controller capability is %s", csi.ControllerServiceCapability_RPC_UNKNOWN)
		}
		if !validControllerCapTypes.Has(k) {
			return nil, fmt.Errorf("unsupported controller capability: %v. Supported values: %v", k, sets.List[csi.ControllerServiceCapability_RPC_Type](validControllerCapTypes))
		}
	}

	volumeValidator, err := NewVolumeValidator(volumeCapModes)
	if err != nil {
		return nil, err
	}

	v.volumeValidator = volumeValidator

	v.controllerCapTypesSet = sets.New[csi.ControllerServiceCapability_RPC_Type](controllerCapTypes...)

	for k := range v.controllerCapTypesSet {
		cap := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: k,
				},
			},
		}
		v.controllerCaps = append(v.controllerCaps, cap)
	}

	return v, nil
}

func (v *controllerValidator) CreateVolumeRequestValidate(req *csi.CreateVolumeRequest) error {
	if errs := ValidateCreateVolumeRequest(req); len(errs) > 0 {
		return status.Errorf(codes.InvalidArgument, errs.ToAggregate().Error())
	}

	for _, volumeCap := range req.VolumeCapabilities {
		if mode := volumeCap.AccessMode.Mode; !v.volumeValidator.HasVolumeCapabilityMode(mode) {
			return status.Errorf(codes.InvalidArgument, "Plugin does not support access mode: %v. Supported access modes: %v", mode, v.volumeValidator.GetVolumeCapabilityModes())
		}
	}

	return nil
}

func (v *controllerValidator) DeleteVolumeRequestValidate(req *csi.DeleteVolumeRequest) error {
	if errs := ValidateDeleteVolumeRequest(req); len(errs) > 0 {
		return status.Errorf(codes.InvalidArgument, errs.ToAggregate().Error())
	}
	return nil
}

func (v *controllerValidator) ControllerPublishVolumeRequestValidate(req *csi.ControllerPublishVolumeRequest) error {
	if errs := ValidateControllerPublishVolumeRequest(req); len(errs) > 0 {
		return status.Errorf(codes.InvalidArgument, errs.ToAggregate().Error())
	}

	if mode := req.VolumeCapability.AccessMode.Mode; !v.volumeValidator.HasVolumeCapabilityMode(mode) {
		return status.Errorf(codes.InvalidArgument, "Plugin does not support access mode: %v. Supported access modes: %v", mode, v.volumeValidator.GetVolumeCapabilityModes())
	}

	if !v.controllerCapTypesSet.Has(csi.ControllerServiceCapability_RPC_PUBLISH_READONLY) && req.Readonly {
		//err := field.Invalid(field.NewPath("readonly"), req.Readonly, fmt.Sprintf("must be false for plugins that do not support controller capability %s", csi.ControllerServiceCapability_RPC_PUBLISH_READONLY))
		//return status.Errorf(codes.InvalidArgument, err.Error())
		return status.Errorf(codes.InvalidArgument, "readonly is true but plugin does not support controller capability %s", csi.ControllerServiceCapability_RPC_PUBLISH_READONLY)
	}

	return nil
}

func (v *controllerValidator) ControllerUnpublishVolumeRequestValidate(req *csi.ControllerUnpublishVolumeRequest) error {
	if errs := ValidateControllerUnpublishVolumeRequest(req); len(errs) > 0 {
		return status.Errorf(codes.InvalidArgument, errs.ToAggregate().Error())
	}
	return nil
}

func (v *controllerValidator) GetControllerCapabilities() []*csi.ControllerServiceCapability {
	return v.controllerCaps
}

func (v *controllerValidator) GetControllerCapabilityTypes() []csi.ControllerServiceCapability_RPC_Type {
	return sets.List[csi.ControllerServiceCapability_RPC_Type](v.controllerCapTypesSet)
}

func (v *controllerValidator) GetVolumeCapabilityModes() []csi.VolumeCapability_AccessMode_Mode {
	return v.volumeValidator.GetVolumeCapabilityModes()
}

func (v *controllerValidator) HasControllerCapabilityType(k csi.ControllerServiceCapability_RPC_Type) bool {
	return v.controllerCapTypesSet.Has(k)
}

func (v *controllerValidator) HasVolumeCapabilityMode(k csi.VolumeCapability_AccessMode_Mode) bool {
	return v.volumeValidator.HasVolumeCapabilityMode(k)
}
