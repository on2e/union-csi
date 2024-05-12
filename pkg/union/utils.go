package union

import (
	"fmt"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	v1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
)

func getQuantity(value int64) *resource.Quantity {
	return resource.NewQuantity(value, resource.BinarySI)
}

func getAccessModes(modes []csi.VolumeCapability_AccessMode_Mode) ([]v1.PersistentVolumeAccessMode, error) {
	// NOTE: can filter modes for duplicate elements.
	accessModes := make([]v1.PersistentVolumeAccessMode, 0)
	for _, mode := range modes {
		accessMode, err := csiToK8sAccessMode(mode)
		if err != nil {
			return []v1.PersistentVolumeAccessMode{}, err
		}
		accessModes = append(accessModes, accessMode)
	}
	return accessModes, nil
}

func csiToK8sAccessMode(mode csi.VolumeCapability_AccessMode_Mode) (v1.PersistentVolumeAccessMode, error) {
	var accessMode v1.PersistentVolumeAccessMode
	switch mode {
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:
		return v1.ReadWriteOnce, nil
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
		return v1.ReadOnlyMany, nil
	case csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER:
		return accessMode, fmt.Errorf("access mode %v is currently not supported", mode)
	case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
		return v1.ReadWriteMany, nil
	default:
		return accessMode, fmt.Errorf("unsupported or unknown access mode: %v", mode)
	}
}

func claimToClaimKey(claim *v1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s/%s", claim.Namespace, claim.Name)
}
