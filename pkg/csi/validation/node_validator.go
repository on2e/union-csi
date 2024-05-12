package validation

import (
	"fmt"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	sets "k8s.io/apimachinery/pkg/util/sets"
)

type NodeServerRequestValidator interface {
	NodePublishVolumeRequestValidate(req *csi.NodePublishVolumeRequest) error
	NodeUnpublishVolumeRequestValidate(req *csi.NodeUnpublishVolumeRequest) error
}

type NodeValidator interface {
	NodeServerRequestValidator

	GetNodeCapabilities() []*csi.NodeServiceCapability
	GetNodeCapabilityTypes() []csi.NodeServiceCapability_RPC_Type
	GetVolumeCapabilityModes() []csi.VolumeCapability_AccessMode_Mode
	HasNodeCapabilityType(csi.NodeServiceCapability_RPC_Type) bool
	HasVolumeCapabilityMode(csi.VolumeCapability_AccessMode_Mode) bool
}

// Exclude csi.NodeServiceCapability_RPC_UNKNOWN.
var validNodeCapTypes = sets.New[csi.NodeServiceCapability_RPC_Type](
	csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
	csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
	csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
	csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
	csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP,
)

type nodeValidator struct {
	nodeCapTypesSet sets.Set[csi.NodeServiceCapability_RPC_Type]
	nodeCaps        []*csi.NodeServiceCapability

	volumeValidator VolumeValidator
}

var _ NodeValidator = &nodeValidator{}

func NewNodeValidator(
	nodeCapTypes []csi.NodeServiceCapability_RPC_Type,
	volumeCapModes []csi.VolumeCapability_AccessMode_Mode) (*nodeValidator, error) {
	v := &nodeValidator{}

	for _, k := range nodeCapTypes {
		if k == csi.NodeServiceCapability_RPC_UNKNOWN {
			return nil, fmt.Errorf("node capability is %s", csi.NodeServiceCapability_RPC_UNKNOWN)
		}
		if !validNodeCapTypes.Has(k) {
			return nil, fmt.Errorf("unsupported node capability: %v. Supported values: %v", k, sets.List[csi.NodeServiceCapability_RPC_Type](validNodeCapTypes))
		}
	}

	volumeValidator, err := NewVolumeValidator(volumeCapModes)
	if err != nil {
		return nil, err
	}

	v.volumeValidator = volumeValidator

	v.nodeCapTypesSet = sets.New[csi.NodeServiceCapability_RPC_Type](nodeCapTypes...)

	for k := range v.nodeCapTypesSet {
		cap := &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: k,
				},
			},
		}
		v.nodeCaps = append(v.nodeCaps, cap)
	}

	return v, nil
}

func (v *nodeValidator) NodePublishVolumeRequestValidate(req *csi.NodePublishVolumeRequest) error {
	if errs := ValidateNodePublishVolumeRequest(req); len(errs) > 0 {
		return status.Errorf(codes.InvalidArgument, errs.ToAggregate().Error())
	}

	if mode := req.VolumeCapability.AccessMode.Mode; !v.volumeValidator.HasVolumeCapabilityMode(mode) {
		return status.Errorf(codes.InvalidArgument, "Plugin does not support access mode: %v. Supported access modes: %v", mode, v.volumeValidator.GetVolumeCapabilityModes())

	}

	if v.nodeCapTypesSet.Has(csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME) && len(req.StagingTargetPath) == 0 {
		//err := field.Invalid(field.NewPath("stagingTargetPath"), req.StagingTargetPath, fmt.Sprintf("must be set for plugins that support node capability %s", csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME))
		//return status.Errorf(codes.InvalidArgument, err.Error())
		return status.Errorf(codes.InvalidArgument, "stagingTargetPath is empty but plugin supports node capability %v", csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)

	}

	return nil
}

func (v *nodeValidator) NodeUnpublishVolumeRequestValidate(req *csi.NodeUnpublishVolumeRequest) error {
	if errs := ValidateNodeUnpublishVolumeRequest(req); len(errs) > 0 {
		return status.Errorf(codes.InvalidArgument, errs.ToAggregate().Error())
	}
	return nil
}

func (v *nodeValidator) GetNodeCapabilities() []*csi.NodeServiceCapability {
	return v.nodeCaps
}

func (v *nodeValidator) GetNodeCapabilityTypes() []csi.NodeServiceCapability_RPC_Type {
	return sets.List[csi.NodeServiceCapability_RPC_Type](v.nodeCapTypesSet)
}

func (v *nodeValidator) GetVolumeCapabilityModes() []csi.VolumeCapability_AccessMode_Mode {
	return v.volumeValidator.GetVolumeCapabilityModes()
}

func (v *nodeValidator) HasNodeCapabilityType(k csi.NodeServiceCapability_RPC_Type) bool {
	return v.nodeCapTypesSet.Has(k)
}

func (v *nodeValidator) HasVolumeCapabilityMode(k csi.VolumeCapability_AccessMode_Mode) bool {
	return v.volumeValidator.HasVolumeCapabilityMode(k)
}
