package driver

import (
	"context"

	csi "github.com/container-storage-interface/spec/lib/go/csi"

	csivalidation "github.com/on2e/union-csi-driver/pkg/csi/validation"
)

type identityServer struct {
	validator csivalidation.IdentityValidator
}

var _ csi.IdentityServer = &identityServer{}

func newIdentityServer() *identityServer {
	validator, err := csivalidation.NewIdentityValidator(pluginCapabilities)
	if err != nil {
		panic(err)
	}
	return &identityServer{validator: validator}
}

func (s *identityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          driverName,
		VendorVersion: driverVersion,
	}, nil
}

func (s *identityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{Capabilities: s.validator.GetPluginCapabilities()}, nil
}

func (s *identityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	// Report ready at any time, might need to distinguish between states in the future.
	return &csi.ProbeResponse{}, nil
}
