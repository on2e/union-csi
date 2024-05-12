package validation

import (
	"fmt"
	"path"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	field "k8s.io/apimachinery/pkg/util/validation/field"
)

// Controller service request validation.

func ValidateCreateVolumeRequest(req *csi.CreateVolumeRequest) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(req.Name) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("name"), ""))
	}

	allErrs = append(allErrs, validateCapacityRange(req.CapacityRange, field.NewPath("capacityRange"))...)
	allErrs = append(allErrs, validateVolumeCapabilities(req.VolumeCapabilities, field.NewPath("volumeCapabilities"))...)
	allErrs = append(allErrs, validateVolumeContentSource(req.VolumeContentSource, field.NewPath("volumeContentSource"))...)
	allErrs = append(allErrs, validateAccessibilityRequirements(req.AccessibilityRequirements, field.NewPath("accessibilityRequirements"))...)

	return allErrs
}

func validateCapacityRange(capacityRange *csi.CapacityRange, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if capacityRange == nil { // optional field
		return allErrs
	}

	reqBytes, limBytes := capacityRange.RequiredBytes, capacityRange.LimitBytes

	if reqBytes == 0 && limBytes == 0 {
		return append(allErrs, field.Required(fldPath.Child("requiredBytes"), "must specify either requiredBytes or limitBytes or both when capacityRange is specified"))
	}

	allErrs = append(allErrs, apimachineryvalidation.ValidateNonnegativeField(reqBytes, fldPath.Child("requiredBytes"))...)
	allErrs = append(allErrs, apimachineryvalidation.ValidateNonnegativeField(limBytes, fldPath.Child("limitBytes"))...)

	if reqBytes > limBytes && limBytes > 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("requiredBytes"), "requiredBytes must not be greater than limitBytes when limitBytes is specified"))
	}

	return allErrs
}

func validateVolumeCapabilities(volumeCaps []*csi.VolumeCapability, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(volumeCaps) == 0 {
		return append(allErrs, field.Required(fldPath, ""))
	}

	for i, volumeCap := range volumeCaps {
		allErrs = append(allErrs, validateVolumeCapability(volumeCap, fldPath.Index(i))...)
	}

	return allErrs
}

func validateVolumeCapability(volumeCap *csi.VolumeCapability, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if volumeCap == nil {
		return append(allErrs, field.Required(fldPath, ""))
	}

	switch typePath := fldPath.Child("accessType"); t := volumeCap.AccessType.(type) {
	case nil:
		// nil interface: (T=nil, V=*)
		allErrs = append(allErrs, field.Required(typePath, ""))
	case *csi.VolumeCapability_Block:
		// Interface with typed nil value: (T=*csi.VolumeCapability_Block, V=nil)
		if t == nil {
			return append(allErrs, field.Required(typePath, ""))
		}
		if t.Block == nil {
			allErrs = append(allErrs, field.Required(typePath.Child("block"), ""))
		}
	case *csi.VolumeCapability_Mount:
		// Interface with typed nil value: (T=*csi.VolumeCapability_Mount, V=nil)
		if t == nil {
			return append(allErrs, field.Required(typePath, ""))
		}
		if t.Mount == nil {
			allErrs = append(allErrs, field.Required(typePath.Child("mount"), ""))
		}
	default:
		// Invalid type implementing csi.isVolumeCapability_AccessType interface.
		// Error message will be:
		// 'createVolumeRequest.volumeCapabilities[<index>].accessType: Invalid value: <underlying_value>: invalid type for access type field'
		allErrs = append(allErrs, field.TypeInvalid(typePath, t, "invalid type for access type field"))
	}

	if modePath := fldPath.Child("accessMode"); volumeCap.AccessMode == nil {
		allErrs = append(allErrs, field.Required(modePath, ""))
	} else {
		allErrs = append(allErrs, validateMode(volumeCap.AccessMode.Mode, modePath.Child("mode"))...)
	}

	return allErrs
}

func validateMode(mode csi.VolumeCapability_AccessMode_Mode, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch mode {
	case csi.VolumeCapability_AccessMode_UNKNOWN:
		allErrs = append(allErrs, field.Required(fldPath, fmt.Sprintf("access mode is %s", csi.VolumeCapability_AccessMode_UNKNOWN)))
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY:
	case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
	case csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER:
	case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER:
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:
	default:
		validModes := []string{
			csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER.String(),
			csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY.String(),
			csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY.String(),
			csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER.String(),
			csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER.String(),
			csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER.String(),
			csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER.String(),
		}
		allErrs = append(allErrs, field.NotSupported(fldPath, mode, validModes))
	}

	return allErrs
}

func validateVolumeContentSource(vcSource *csi.VolumeContentSource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if vcSource == nil { // optional field
		return allErrs
	}

	switch typePath := fldPath.Child("type"); t := vcSource.Type.(type) {
	case nil:
		// nil interface: (T=nil, V=*)
		allErrs = append(allErrs, field.Required(typePath, ""))
	case *csi.VolumeContentSource_Snapshot:
		// Interface with typed nil value: (T=*csi.VolumeContentSource_Snapshot, V=nil)
		if t == nil {
			return append(allErrs, field.Required(typePath, ""))
		}
		if t.Snapshot == nil {
			return append(allErrs, field.Required(typePath.Child("snapshot"), ""))
		}
		if len(t.Snapshot.SnapshotId) == 0 {
			allErrs = append(allErrs, field.Required(typePath.Child("snapshot").Child("snapshotId"), ""))
		}
	case *csi.VolumeContentSource_Volume:
		// Interface with typed nil value: (T=*csi.VolumeContentSource_Volume, V=nil)
		if t == nil {
			return append(allErrs, field.Required(typePath, ""))
		}
		if t.Volume == nil {
			return append(allErrs, field.Required(typePath.Child("volume"), ""))
		}
		if len(t.Volume.VolumeId) == 0 {
			allErrs = append(allErrs, field.Required(typePath.Child("volume").Child("volumeId"), ""))
		}
	default:
		// Invalid type implementing csi.isVolumeContentSource_Type interface.
		// Error message will be:
		// 'createVolumeRequest.volumeContentSource[<index>].accessType: Invalid value: <underlying_value>: invalid type for type field'
		allErrs = append(allErrs, field.TypeInvalid(typePath, t, "invalid type for type field"))
	}

	return allErrs
}

func validateAccessibilityRequirements(topologyRequirement *csi.TopologyRequirement, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if topologyRequirement == nil { // optional field
		return allErrs
	}

	reqPath := fldPath.Child("requisite")
	if len(topologyRequirement.Requisite) == 0 && len(topologyRequirement.Preferred) == 0 {
		return append(allErrs, field.Required(reqPath, "must specify either requisite or preferred or both when topologyRequirement is specified"))
	}

	for i, topology := range topologyRequirement.Requisite {
		allErrs = append(allErrs, validateTopology(topology, reqPath.Index(i))...)
	}

	prefPath := fldPath.Child("preferred")
	for i, topology := range topologyRequirement.Preferred {
		allErrs = append(allErrs, validateTopology(topology, prefPath.Index(i))...)
	}

	return allErrs
}

func validateTopology(topology *csi.Topology, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if topology == nil {
		return append(allErrs, field.Required(fldPath, ""))
	}

	if segmPath := fldPath.Child("segments"); len(topology.Segments) == 0 {
		// "Each value (topological segment) MUST contain 1 or more strings."
		// This seems wrong/misphrased since a topological segment is defined as a string value of the topology map, so it's like saying:
		// "Each string MUST contain 1 or more strings."
		allErrs = append(allErrs, field.Required(segmPath, "topology must contain at least 1 topological segment"))
	} else {
		allErrs = append(allErrs, metav1validation.ValidateLabels(topology.Segments, segmPath)...)
	}

	return allErrs
}

func ValidateDeleteVolumeRequest(req *csi.DeleteVolumeRequest) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(req.VolumeId) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("volumeId"), ""))
	}

	return allErrs
}

func ValidateControllerPublishVolumeRequest(req *csi.ControllerPublishVolumeRequest) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(req.VolumeId) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("volumeId"), ""))
	}

	if len(req.NodeId) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("nodeId"), ""))
	}

	allErrs = append(allErrs, validateVolumeCapability(req.VolumeCapability, field.NewPath("volumeCapability"))...)

	return allErrs
}

func ValidateControllerUnpublishVolumeRequest(req *csi.ControllerUnpublishVolumeRequest) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(req.VolumeId) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("volumeId"), ""))
	}

	return allErrs
}

// Node service request validation.

func ValidateNodePublishVolumeRequest(req *csi.NodePublishVolumeRequest) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(req.VolumeId) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("volumeId"), ""))
	}

	if len(req.StagingTargetPath) > 0 { // optional field
		allErrs = append(allErrs, validateTargetPath(req.StagingTargetPath, field.NewPath("stagingTargetPath"))...)
	}

	allErrs = append(allErrs, validateTargetPath(req.TargetPath, field.NewPath("targetPath"))...)
	allErrs = append(allErrs, validateVolumeCapability(req.VolumeCapability, field.NewPath("volumeCapability"))...)

	return allErrs
}

func validateTargetPath(targetPath string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(targetPath) == 0 {
		return append(allErrs, field.Required(fldPath, ""))
	}

	if !path.IsAbs(targetPath) {
		allErrs = append(allErrs, field.Invalid(fldPath, targetPath, "must be an absolute path"))
	}

	// More checks? Not containing `..` maybe?
	return allErrs
}

func ValidateNodeUnpublishVolumeRequest(req *csi.NodeUnpublishVolumeRequest) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(req.VolumeId) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("volumeId"), ""))
	}

	allErrs = append(allErrs, validateTargetPath(req.TargetPath, field.NewPath("targetPath"))...)

	return allErrs
}

// TODO: add validation for rest gRPC requests
