package driver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	klog "k8s.io/klog/v2"

	csivalidation "github.com/on2e/union-csi-driver/pkg/csi/validation"
	union "github.com/on2e/union-csi-driver/pkg/union"
)

type controllerServer struct {
	union     union.Interface
	validator csivalidation.ControllerValidator
	options   *driverOptions
}

var _ csi.ControllerServer = &controllerServer{}

func newControllerServer(union union.Interface, driverOptions *driverOptions) *controllerServer {
	validator, err := csivalidation.NewControllerValidator(controllerCapabilities, volumeCapabilities)
	if err != nil {
		panic(err)
	}
	return &controllerServer{
		union:     union,
		validator: validator,
		options:   driverOptions,
	}
}

func (s *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if err := s.validator.CreateVolumeRequestValidate(req); err != nil {
		return nil, err
	}

	// Check for each volume capability, if each access mode and access type (block or mount) can be satisfied.
	for _, volumeCap := range req.GetVolumeCapabilities() {
		switch volumeCap.GetAccessType().(type) {
		case *csi.VolumeCapability_Block:
			return nil, status.Error(codes.InvalidArgument, "Volume capability with access type of block not supported. Support only mount volumes")
		case *csi.VolumeCapability_Mount:
			//mountVolume := accessType.Mount
			// Further process mount fields here if need be.
		}
	}

	options := &union.CreateLowerOptions{
		LowerNamespace: s.options.defaultLowerNamespace,
	}

	if err := parseParameters(req.GetParameters(), options); err != nil {
		return nil, err
	}

	capacityBytes, err := getCapacityBytes(req.GetCapacityRange())
	if err != nil {
		return nil, err
	}
	options.CapacityBytes = capacityBytes

	for _, volumeCap := range req.GetVolumeCapabilities() {
		options.CSIAccessModes = append(options.CSIAccessModes, volumeCap.GetAccessMode().GetMode())
	}

	volumeName := req.GetName()

	klog.InfoS("CreateVolume: creating", "Name", volumeName)
	volume, err := s.union.CreateLower(ctx, volumeName, options)
	if err != nil {
		code := codes.Internal
		msg := fmt.Sprintf("Failed to create volume %s: %v", volumeName, err)
		switch {
		case errors.Is(err, union.ErrIdempotencyIncompatible):
			code = codes.AlreadyExists
		}
		return nil, status.Error(code, msg)
	}
	klog.InfoS("CreateVolume: created", "Name", volumeName)

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volume.VolumeId,
			CapacityBytes: volume.CapacityBytes,
		},
	}, nil
}

func parseParameters(params map[string]string, options *union.CreateLowerOptions) error {
	for k, v := range params {
		switch strings.ToLower(k) {
		case LowerNamespaceParamKey:
			options.LowerNamespace = v
		case LowerStorageClassNameParamKey:
			// TODO: move this validation to VolumeSplit validation
			if v == "" {
				return status.Errorf(codes.InvalidArgument, "%s value cannot be empty (\"\") when specified in parameters", k)
			}
			if options.LowerStorageClassName == nil {
				options.LowerStorageClassName = new(string)
			}
			*options.LowerStorageClassName = v
		case PVCNameParamKey, PVCNamespaceParamKey, PVNameParamKey:
			// NOOP ATM
		default:
			return status.Errorf(codes.InvalidArgument, "unknown parameters key: %q", k)
		}
	}
	return nil
}

func getCapacityBytes(capacityRange *csi.CapacityRange) (int64, error) {
	if capacityRange == nil {
		return DefaultCapacityBytes, nil
	}
	// * A PVC's Spec.Resources.Limits is ignored by external-provisioner side-car container.
	// * LimitBytes in CreateVolumeRequest.CapacityRange is not set by external-provisioner.
	// * Returned CreateVolumeResponse.CapacityBytes is not checked to be lower than LimitBytes.
	// Just retrieve RequiredBytes and don't care about LimitBytes.
	return capacityRange.GetRequiredBytes(), nil
}

func (s *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if err := s.validator.DeleteVolumeRequestValidate(req); err != nil {
		return nil, err
	}

	volumeId := req.GetVolumeId()

	klog.InfoS("DeleteVolume: deleting", "VolumeId", volumeId)
	if err := s.union.DeleteLower(ctx, volumeId); err != nil {
		code := codes.Internal
		msg := fmt.Sprintf("Failed to delete volume %s: %v", volumeId, err)
		switch {
		case errors.Is(err, union.ErrVolumeNotFound):
			klog.InfoS("DeleteVolume: volume not found, return OK", "VolumeId", volumeId)
			return &csi.DeleteVolumeResponse{}, nil
			//case errors.Is(err, union.ErrVolumeInUse):
			//	code = codes.FailedPrecondition
		}
		return nil, status.Error(code, msg)
	}
	klog.InfoS("DeleteVolume: deleted", "VolumeId", volumeId)

	return &csi.DeleteVolumeResponse{}, nil
}

func (s *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if err := s.validator.ControllerPublishVolumeRequestValidate(req); err != nil {
		return nil, err
	}

	volumeId := req.GetVolumeId()
	nodeId := req.GetNodeId()

	klog.InfoS("ControllerPublishVolume: attaching", "VolumeId", volumeId, "NodeId", nodeId)
	attachment, err := s.union.AttachLower(ctx, volumeId, nodeId)
	if err != nil {
		code := codes.Internal
		msg := fmt.Sprintf("Failed to attach volume %s at node %s: %v", volumeId, nodeId, err)
		switch {
		case errors.Is(err, union.ErrVolumeInUse):
			code = codes.FailedPrecondition
			msg = fmt.Sprintf("Volume %q cannot be attached at node %q, already attached at node %q", volumeId, nodeId, attachment.NodeId)
		//case errors.Is(err, union.ErrIdempotencyIncompatible):
		//	code = codes.AlreadyExists
		case errors.Is(err, union.ErrVolumeNotFound), errors.Is(err, union.ErrNodeNotFound):
			code = codes.NotFound
		}
		return nil, status.Error(code, msg)
	}
	klog.InfoS("ControllerPublishVolume: attached", "VolumeId", volumeId, "NodeId", nodeId)

	publishContext := map[string]string{PathPublishContextKey: attachment.HostPath}

	return &csi.ControllerPublishVolumeResponse{PublishContext: publishContext}, nil
}

func (s *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	// NOTE: NodeId is not required to be set in ControllerUnpublishVolumeRequest.
	// Since we currently not support MULTI_NODE_... i think NodeId will be set at all times
	// and also we cannot do without when not in MULTI_NODE_... .
	// Require to be set to find out and consider to check if it is not set when MULTI_NODE_... is not supported in validator pkg.
	if err := s.validator.ControllerUnpublishVolumeRequestValidate(req); err != nil {
		return nil, err
	}

	volumeId := req.GetVolumeId()
	nodeId := req.GetNodeId()
	if len(nodeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "nodeId is missing")
	}

	klog.InfoS("ControllerUnpublishVolume: detaching", "VolumeId", volumeId, "NodeId", nodeId)
	if err := s.union.DetachLower(ctx, volumeId, nodeId); err != nil {
		code := codes.Internal
		msg := fmt.Sprintf("Failed to detach volume %s at node %s: %v", volumeId, nodeId, err)
		switch {
		case errors.Is(err, union.ErrAttachmentNotFound):
			klog.InfoS("ControllerUnpublishVolume: attachment not found, return OK", "VolumeId", volumeId, "NodeId", nodeId)
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		case errors.Is(err, union.ErrVolumeNotFound), errors.Is(err, union.ErrNodeNotFound):
			code = codes.NotFound
		}
		return nil, status.Error(code, msg)
	}
	klog.InfoS("ControllerUnpublishVolume: detached", "VolumeId", volumeId, "NodeId", nodeId)

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// Unimplemented.
func (s *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented ValidateVolumeCapabilities method")
}

// Unimplemented.
func (s *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented ListVolumes method")
}

// Unimplemented.
func (s *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented GetCapacity method")
}

func (s *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: s.validator.GetControllerCapabilities()}, nil
}

// Unimplemented.
func (s *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented CreateSnapshot method")
}

// Unimplemented.
func (s *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented DeleteSnapshot method")
}

// Unimplemented.
func (s *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented ListSnapshots method")
}

// Unimplemented.
func (s *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented ControllerExpandVolume method")
}

// Unimplemented.
func (s *controllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented ControllerGetVolume method")
}
