package driver

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi"
)

/*
Supported capabilities for the driver
*/

// Capabilities for the identity service
var pluginCapabilities = []csi.PluginCapability_Service_Type{
	csi.PluginCapability_Service_CONTROLLER_SERVICE,
}

// Capabilities for the controller service
var controllerCapabilities = []csi.ControllerServiceCapability_RPC_Type{
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
}

// Capabilities for the node service
var nodeCapabilities = []csi.NodeServiceCapability_RPC_Type{}

// Capabilities for the volumes (access modes)
var volumeCapabilities = []csi.VolumeCapability_AccessMode_Mode{
	csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
}
