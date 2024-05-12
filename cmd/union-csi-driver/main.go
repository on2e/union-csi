package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"
	informers "k8s.io/client-go/informers"
	kubernetes "k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	klog "k8s.io/klog/v2"

	driver "github.com/on2e/union-csi-driver/pkg/csi/driver"
	unionclientset "github.com/on2e/union-csi-driver/pkg/k8s/client/clientset"
	union "github.com/on2e/union-csi-driver/pkg/union"
)

func main() {
	var config *rest.Config
	var err error

	fs := flag.NewFlagSet("union-csi-driver", flag.ExitOnError)

	options := GetOptions(fs)

	kubeconfig := options.Kubeconfig

	if kubeconfig != "" {
		klog.Info("Building config from kubeconfig ...")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		klog.Info("Building config from in-cluster ...")
		config, err = rest.InClusterConfig()
		// Try also: 1: $KUBECONFIG and 2: $HOME/.kube/config
	}
	if err != nil {
		klog.Fatalf("Failed to create config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create kubernetes client: %v", err)
	}

	unionClient, err := unionclientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create union client: %v", err)
	}

	factory := informers.NewSharedInformerFactory(kubeClient, 15*time.Minute)

	uunion := union.New(
		kubeClient,
		unionClient,
		factory.Core().V1().PersistentVolumeClaims(),
		factory.Core().V1().Nodes(),
		factory.Core().V1().Pods(),
	)

	driver, err := driver.NewDriver(
		uunion,
		driver.WithMode(options.Mode),
		driver.WithCSIEndpoint(options.CSIEndpoint),
		driver.WithDefaultLowerNamespace(options.DefaultLowerNamespace),
	)
	if err != nil {
		klog.Fatalf("Failed to create driver: %v", err)
	}

	ctx := context.Background()

	factory.Start(ctx.Done())
	for k, synced := range factory.WaitForCacheSync(ctx.Done()) {
		if !synced {
			klog.Fatalf("Failed to sync caches: %v", k)
		}
	}
	go uunion.Run(ctx)

	if err := driver.Run(ctx); err != nil {
		klog.Fatalf("Failed to run driver: %v", err)
	}
}

type Options struct {
	Mode                  driver.DriverMode
	CSIEndpoint           string
	DefaultLowerNamespace string
	Kubeconfig            string
}

func GetOptions(fs *flag.FlagSet) *Options {
	var mode *string
	var version *bool

	options := Options{}

	func() {
		fs.StringVar(
			&options.CSIEndpoint,
			"endpoint",
			driver.DefaultCSIEndpoint,
			"gRPC endpoint of CSI driver",
		)
		fs.StringVar(
			&options.DefaultLowerNamespace,
			"lower-namespace",
			driver.DefaultLowerNamespace,
			"Namespace of lower PersistentVolumeClaims when lowerNamespace is unspecified in StorageClass parameters",
		)
		//"StorageClass of lower PersistentVolumeClaims when lowerStorageClass is unspecified in StorageClass parameters. If this and lowerStorageClass are both unspecified then any lower PVCs created will have no storageClassName set (default StorageClass)",
		fs.StringVar(
			&options.Kubeconfig,
			"kubeconfig",
			"",
			"Absolute path to a kubeconfig file. Useful when running out-of-cluster",
		)

		mode = fs.String(
			"mode",
			"all",
			"Set CSI driver mode. \"controller\" runs the Controller service, \"node\" runs the Node service and \"all\" runs both",
		)
		version = fs.BoolP(
			"version",
			"v",
			false,
			"Print version and exit",
		)
	}()

	/*if err := fs.Parse(os.Args[1:]); err != nil {
		klog.Fatalf("Failed to parse command-line arguments: %v", err)
	}*/
	_ = fs.Parse(os.Args[1:])

	if *version {
		fmt.Println(filepath.Base(os.Args[0]), driver.GetVersion())
		os.Exit(0)
	}

	switch *mode {
	case "all":
		options.Mode = driver.ModeAll
	case "controller":
		options.Mode = driver.ModeController
	case "node":
		options.Mode = driver.ModeNode
	default:
		fmt.Printf("unknown driver mode: %q. Must be one of %v\n", *mode, []string{"all", "controller", "node"})
		os.Exit(1)
	}

	return &options
}
