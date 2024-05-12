package driver

// Constants for default driver option values
const (
	DefaultCSIEndpoint    = "unix:///tmp/csi.sock"
	DefaultLowerNamespace = "union"
)

const (
	DefaultCapacityBytes int64 = 1024
)

// Constants for parameter keys
const (
	LowerNamespaceParamKey        = "lowernamespace"
	LowerStorageClassNameParamKey = "lowerstorageclassname"
	PVCNameParamKey               = "csi.storage.k8s.io/pvc/name"
	PVCNamespaceParamKey          = "csi.storage.k8s.io/pvc/namespace"
	PVNameParamKey                = "csi.storage.k8s.io/pv/name"
)

// Contants for topology keys
/*const (
	NodeIdTopologyKey = "topology.union.io/node-id"
)*/

// Contants for PublishContext keys
const (
	PathPublishContextKey = "path"
)
