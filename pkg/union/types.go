package union

import (
	"context"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	v1alpha1 "github.com/on2e/union-csi-driver/pkg/k8s/apis/union/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

type Interface interface {
	CreateLower(ctx context.Context, volumeName string, options *CreateLowerOptions) (*Volume, error)
	DeleteLower(ctx context.Context, volumeId string) error
	AttachLower(ctx context.Context, volumeId, nodeId string) (*VolumeAttachment, error)
	DetachLower(ctx context.Context, volumeId, nodeId string) error
}

type CreateLowerOptions struct {
	CapacityBytes         int64
	LowerNamespace        string
	LowerStorageClassName *string
	CSIAccessModes        []csi.VolumeCapability_AccessMode_Mode
}

// TODO: integrate in AttachLower() args
type AttachLowerOptions struct {
	CSIAccessMode csi.VolumeCapability_AccessMode_Mode
	ReadOnly      bool
}

// Volume is the internal representation of an "upper" volume.
// It is a mirror of the v1alpha1.VolumeSplit API type to pass around functions for convenience.
type Volume struct {
	//
	VolumeId string
	//
	CapacityBytes int64
	//
	AccessModes []v1.PersistentVolumeAccessMode
	//
	Namespace string
	//
	ClaimNames []string
	//
	StorageClassName *string
}

type VolumeAttachment struct {
	VolumeId string
	NodeId   string
	HostPath string
}

// Attacher defines the interface that abstracts over the attach/detach operations
type Attacher interface {
	Attach(ctx context.Context, volume *Volume, nodeId string) (*VolumeAttachment, error)
	Detach(ctx context.Context, volume *Volume, nodeId string) error
}

// NewVolumeFromVolumeSplit creates a Volume from a v1alpha1.VolumeSplit
func NewVolumeFromVolumeSplit(split *v1alpha1.VolumeSplit) *Volume {
	size := split.Spec.CapacityTotal[v1.ResourceStorage]
	volume := &Volume{
		VolumeId:         split.Spec.VolumeName,
		CapacityBytes:    size.Value(),
		AccessModes:      split.Spec.AccessModes,
		Namespace:        split.Spec.Namespace,
		StorageClassName: split.Spec.StorageClassName,
	}
	for i := range split.Spec.Splits {
		volume.ClaimNames = append(volume.ClaimNames, split.Spec.Splits[i].ClaimName)
	}
	return volume
}
