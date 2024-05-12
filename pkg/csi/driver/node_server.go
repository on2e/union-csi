package driver

import (
	"context"
	"fmt"
	"os"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	klog "k8s.io/klog/v2"

	csivalidation "github.com/on2e/union-csi-driver/pkg/csi/validation"
	mount "github.com/on2e/union-csi-driver/pkg/mount"
)

type nodeServer struct {
	nodeId    string
	mounter   mount.Mounter
	validator csivalidation.NodeValidator
}

var _ csi.NodeServer = &nodeServer{}

func newNodeServer() *nodeServer {
	nodeId := os.Getenv("NODE_NAME")
	if nodeId == "" {
		err := fmt.Errorf("unset or empty NODE_NAME environment variable")
		panic(err)
	}
	validator, err := csivalidation.NewNodeValidator(nodeCapabilities, volumeCapabilities)
	if err != nil {
		panic(err)
	}
	return &nodeServer{
		nodeId:    nodeId,
		mounter:   mount.NewMounter(),
		validator: validator,
	}
}

// Unimplemented.
func (s *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented NodeStageVolume method")
}

// Unimplemented.
func (s *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented NodeUnstageVolume method")
}

func (s *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if err := s.validator.NodePublishVolumeRequestValidate(req); err != nil {
		return nil, err
	}

	volumeId := req.GetVolumeId()
	target := req.GetTargetPath()

	if len(req.GetPublishContext()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "publishContext is missing")
	}

	source, ok := req.GetPublishContext()[PathPublishContextKey]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Missing publishContext key: %q", PathPublishContextKey)
	}

	var mountVolume *csi.VolumeCapability_MountVolume
	var options *mount.PublishOptions

	switch accessType := req.GetVolumeCapability().GetAccessType().(type) {
	case *csi.VolumeCapability_Block:
		return nil, status.Error(codes.InvalidArgument, "Volume capability with access type of block not supported. Support only mount volumes")
	case *csi.VolumeCapability_Mount:
		mountVolume = accessType.Mount
		options = &mount.PublishOptions{
			FsType:       mountVolume.GetFsType(),
			ReadOnly:     req.GetReadonly(),
			MountOptions: mountVolume.GetMountFlags(),
		}
	}

	klog.InfoS("NodePublishVolume: mounting", "VolumeId", volumeId, "TargetPath", target)
	if err := s.mounter.Publish(source, target, options); err != nil {
		code := codes.Internal
		msg := fmt.Sprintf("Failed to mount volume %s at path %s: %v", volumeId, target, err)
		switch {
		}
		return nil, status.Error(code, msg)
	}
	klog.InfoS("NodePublishVolume: mounted", "VolumeId", volumeId, "TargetPath", target)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := s.validator.NodeUnpublishVolumeRequestValidate(req); err != nil {
		return nil, err
	}

	volumeId := req.GetVolumeId()
	target := req.GetTargetPath()

	klog.InfoS("NodeUnpublishVolume: unmounting", "VolumeId", volumeId, "TargetPath", target)
	if err := s.mounter.Unpublish(target); err != nil {
		code := codes.Internal
		msg := fmt.Sprintf("Failed to unmount volume %s at path %s: %v", volumeId, target, err)
		switch {
		}
		return nil, status.Error(code, msg)
	}
	klog.InfoS("NodeUnpublishVolume: unmounted", "VolumeId", volumeId, "TargetPath", target)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

/*func (s *nodeServer) nodePublishVolumeForMount(source, target string, mount *csi.VolumeCapability_MountVolume) error {
	// Get device ref-counts for source mountpoint (provided by PublishContext from ControllerPublishVolume).
	// * If refs is 0, it means we are missing the mount from ControllerPublishVolume and cannot proceed.
	// * If refs is 1, it means that we only have the source mountpoint and can proceed.
	// * If refs is 2, it means that (probably) we have the initial mount from ControllerPublishVolume plus one NodePublishVolume mount:
	//   * In this case we can check if the NodePublishVolume mountpoint is the same as target and if it is, we can check for VolumeCapability and Readonly
	//   fields idempotency compatibility between the published volume at target and the requested volume at target (how we do that?).
	//   * If the NodePublishVolume mountpoint is NOT the same as target we can check if we support SINGLE_NODE_MULTI_WRITER or MULTI_NODE_...
	//   and decide if we should proceed with more than 1 NodePublishVolume mounts on different targets. This is also the same approach for refs > 2.
	var (
		device string
		refs   []string
		err    error
	)

	device, refs, err = mounter.GetDeviceNameAndMountsFromMount(s.mounter, source)
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to get device mounted at %s: %v", source, err)
	}

	if device == "" {
		klog.Infof("%s is not mounted by a device", source)
		// Is this a "Volume does not exist: 5 NOT_FOUND" error?
		// What is "Volume does not exist" for the nodeServer:
		// Is it that VolumeSplit exists or/and device at source path exists?
		return status.Errorf(codes.Internal, "%s is not a mountpoint, cannot mount at target %s", source, target)
	}

	// Can check here if device has any relation with volumeId

	//klog.InfoS("NodePublishVolume", "Device", device, "Source", source, "Mountpoints", refs, "Target", target)

	switch l := len(refs); {
	case l == 0: // source is the only mountpoint of device.
	case l == 1 && refs[0] == target: // device is mounted at target.
	default:
		// T1!=T2: (in l > 1 case, T1 is "undefined" -> l == 1: s!=target, l > 1: [...]!=target)
		// if *MULTI* { OK }
		// else       { FAILED_PRECONDITION }
		if !s.supportsMulti() {
			// FAILED_PRECONDITION
			return status.Errorf(
				codes.FailedPrecondition,
				"Device %s mounted at %s found also mounted at 1 or more mountpoints that are not target %s"+
					" but multi-consumer volume capability access mode is not supported", device, source, target,
			)
		}
	}

	// Check if target is mounted by device.
	var isMnt bool
	for _, mp := range refs {
		if mp == target {
			isMnt = true
			break
		}
	}

	// The device is also mounted at target.
	if isMnt {
		// We should somehow check that the device is indeed backed by the volume of volumeId
		// and is idempotent-compatible with requested VolumeCapability and Readonly fields.
		// Assume it is for now and return nil error.
		// T1=T2:
		// if P1=P2 { OK (idempotent) }
		// else     { ALREADY_EXISTS }
		klog.InfoS("NodePublishVolume: target already mounted, return OK", "Device", device, "Source", source, "Target", target)
		return nil
	}

	// Check if target is mounted by a filesystem other than device.
	isMnt, err = s.mounter.IsMountPoint(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		if !mountutils.IsCorruptedMnt(err) {
			return status.Errorf(codes.Internal, "Failed to determine if target %s is a mountpoint: %v", target, err)
		}
		klog.Infof("Target %s is likely a corrupted mountpoint, attempting to clean up and mount", target)
		if err := mountutils.CleanupMountPoint(target, s.mounter, true); err != nil {
			if strings.Contains(fmt.Sprint(err), "not mounted") {
				klog.Infof("Ignoring 'not mounted' error for target %s", target)
			} else {
				return status.Errorf(codes.Internal, "Failed to clean up target %s: %v", target, err)
			}
		}
		klog.Infof("Successfully cleaned up target %s", target)
	}

	if isMnt {
		return status.Errorf(codes.Internal, "Target %s is already mounted by a different device, will not attempt to mount", target)
	}

	// Create target dir.
	if err := os.MkdirAll(target, 0o755); err != nil {
		return status.Errorf(codes.Internal, "Failed to create target directory %s", target)
	}

	// Validate mount fields.
	mountOptions := []string{"bind"}

	// Proceed to mount source at target.
	klog.Infof("Mounting source %s at target %s", source, target)
	if err := s.mounter.Mount(source, target, "fuse.mergerfs", mountOptions); err != nil {
		return status.Errorf(codes.Internal, "Failed to mount source %s at target %s: %v", source, target, err)
	}

	return nil
}

func (s *nodeServer) supportsMulti() bool {
	return false
}*/

// Unimplemented.
func (s *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented NodeGetVolumeStats method")
}

// Unimplemented.
func (s *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Unimplemented NodeExpandVolume method")
}

func (s *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{Capabilities: s.validator.GetNodeCapabilities()}, nil
}

func (s *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId:            s.nodeId,
		MaxVolumesPerNode: 0,
	}, nil
}
