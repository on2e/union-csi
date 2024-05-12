package driver

import (
	"context"
	"fmt"
	"net"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	protosanitizer "github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	grpc "google.golang.org/grpc"
	klog "k8s.io/klog/v2"

	union "github.com/on2e/union-csi-driver/pkg/union"
)

var (
	// The name of the CSI driver.
	driverName = "union.csi.driver.union.io"

	// The version of the CSI driver.
	// Set at build time via `-ldflags`.
	driverVersion = "UNKNOWN"
)

func GetVersion() string {
	return driverVersion
}

type DriverMode string

const (
	ModeAll        DriverMode = "All"
	ModeController DriverMode = "Controller"
	ModeNode       DriverMode = "Node"
)

type Driver struct {
	csi.IdentityServer
	csi.ControllerServer
	csi.NodeServer

	srv     *grpc.Server
	options *driverOptions
	// NOTE: consider lock and running bool fields
}

type driverOptions struct {
	mode                  DriverMode
	csiEndpoint           string
	defaultLowerNamespace string
}

func NewDriver(unionHandler union.Interface, options ...DriverOption) (*Driver, error) {
	driverOptions := &driverOptions{
		mode:                  ModeAll,
		csiEndpoint:           DefaultCSIEndpoint,
		defaultLowerNamespace: DefaultLowerNamespace,
	}

	for _, o := range options {
		o(driverOptions)
	}

	// validate driver options here...

	driver := &Driver{
		options: driverOptions,
	}

	klog.InfoS("CSI driver", "name", driverName, "version", driverVersion, "mode", driverOptions.mode)

	driver.IdentityServer = newIdentityServer()

	switch mode := driverOptions.mode; mode {
	case ModeAll:
		driver.ControllerServer = newControllerServer(unionHandler, driverOptions)
		driver.NodeServer = newNodeServer()
	case ModeController:
		driver.ControllerServer = newControllerServer(unionHandler, driverOptions)
	case ModeNode:
		driver.NodeServer = newNodeServer()
	default:
		return nil, fmt.Errorf("unknown driver mode: %q", mode)
	}

	return driver, nil
}

func (d *Driver) Run(ctx context.Context) error {
	klog.Info("Starting driver ...")

	network, address, err := parseEndpoint(d.options.csiEndpoint)
	if err != nil {
		return err
	}

	listener, err := net.Listen(network, address)
	if err != nil {
		return err
	}

	d.srv = grpc.NewServer(grpc.UnaryInterceptor(logInterceptor))

	csi.RegisterIdentityServer(d.srv, d)

	switch mode := d.options.mode; mode {
	case ModeAll:
		csi.RegisterControllerServer(d.srv, d)
		csi.RegisterNodeServer(d.srv, d)
	case ModeController:
		csi.RegisterControllerServer(d.srv, d)
	case ModeNode:
		csi.RegisterNodeServer(d.srv, d)
	default:
		return fmt.Errorf("unknown driver mode: %q", mode)
	}

	klog.Infof("Listening for connections at %q", listener.Addr())

	return d.srv.Serve(listener)
}

func (d *Driver) Stop() {
	klog.Info("Stopping driver ...")
	d.srv.Stop()
}

func logInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	low := strings.LastIndex(info.FullMethod, "/") + 1
	method := info.FullMethod[low:]

	klog.InfoS(method, "req", protosanitizer.StripSecrets(req))
	resp, err = handler(ctx, req)
	if err != nil {
		klog.ErrorS(err, method)
	} else {
		klog.InfoS(method, "resp", protosanitizer.StripSecrets(resp))
	}

	return
}

// DriverOption is a functional option type for driverOptions
type DriverOption func(*driverOptions)

func WithMode(mode DriverMode) DriverOption {
	return func(o *driverOptions) {
		o.mode = mode
	}
}

func WithCSIEndpoint(endpoint string) DriverOption {
	return func(o *driverOptions) {
		o.csiEndpoint = endpoint
	}
}

func WithDefaultLowerNamespace(ns string) DriverOption {
	return func(o *driverOptions) {
		o.defaultLowerNamespace = ns
	}
}
