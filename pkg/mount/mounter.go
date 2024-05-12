package mount

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	klog "k8s.io/klog/v2"
	mountutils "k8s.io/mount-utils"
)

type Mounter interface {
	Publish(string, string, *PublishOptions) error
	Unpublish(string) error
}

type PublishOptions struct {
	FsType       string
	ReadOnly     bool
	MountOptions []string
}

type mounter struct {
	mountutils.Interface
}

var _ Mounter = &mounter{}

func NewMounter() *mounter {
	return &mounter{mountutils.New("")}
}

func (m *mounter) Publish(source, target string, options *PublishOptions) error {
	// First check that source path exists and is a healthy mountpoint.
	// Whould be nice to also get device name and rest mountpoint references.
	// k8s.io/mount-utils GetMountRefs does all that except returning the device name.
	// However, it gives us no way to distinguish between source not existing, source being a corrupted mountpoint,
	// and source existing, being healthy and just having no rest references since it returns []string, nil in every case described.
	// Lead with our own handling of not PathExists and IsCorruptedMnt and follow with GetMountRefs.
	// GetMountRefs also returns error if an entry for source does not exist on /proc/self/mountinfo.
	// If device name if useful/necessary, we can write a modified GetMountRefs in the future.
	exists, err := mountutils.PathExists(source)
	if err != nil {
		if mountutils.IsCorruptedMnt(err) {
			return fmt.Errorf("source %s is corrupted: %v", source, err)
		}
		return fmt.Errorf("error checking if source %s exists: %v", source, err)
	}
	if !exists {
		return fmt.Errorf("source %s does not exist", source)
	}

	refs, err := m.GetMountRefs(source)
	if err != nil {
		// Could just be that source is not mounted, a more suitable error should be produced then.
		return fmt.Errorf("error checking if source %s is mounted: %v", source, err)
	}

	// See how many mount references there are for source.
	// Is this a good way to determine if source is about to mount/has been mounted on more than 1 target
	// and make a case about multi-consumer volume capability support?
	switch l := len(refs); {
	case l == 0: // source is the only mountpoint of device.
	case l == 1 && refs[0] == target: // device is mounted at target.
	default:
		return fmt.Errorf("found more than one mount reference to source %s that is not target %s: %v", source, target, refs)
	}

	// Check if target is mounted by the same device as source.
	var isMnt bool
	for _, ref := range refs {
		if ref == target {
			isMnt = true
			break
		}
	}

	// Target is mounted by the same device as source.
	if isMnt {
		// We should somehow check that the device is indeed backed by the volume of volumeId
		// and is idempotent-compatible with requested VolumeCapability and O fields.
		// Assume it is for now and return nil error.
		//
		// Also also, source is mounted by device, target is mounted by device, source is healthy => target is healthy, right?
		klog.InfoS("Target %s already mounted by source %s", target, source)
		return nil
	}

	// Check if target is mounted by a filesystem other than device of source.
	isMnt, err = m.IsMountPoint(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		if mountutils.IsCorruptedMnt(err) {
			// If target is corrupted, should we attempt to unmount, remove, make and mount again?
			return fmt.Errorf("target %s is corrupted, will not attempt to mount: %v", target, err)
		}
		return fmt.Errorf("error checking if target %s is mounted: %v", target, err)
	}

	// Target is mounted by a diffrent device than source.
	if isMnt {
		return fmt.Errorf("target %s is already mounted by a different device, will not attempt to mount", target)
	}

	// Create target dir.
	// NOTE: can first check if parent dir exists as SHALL do by CO
	//parentDir := filepath.Dir(target)
	//parentExists, err := mountutils.PathExists(parentDir)
	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %v", target, err)
	}

	// Validate mount fields.
	// From https://kubernetes.io/docs/concepts/storage/storage-classes/#mount-options: "Mount options are not validated on either the class or PV. If a mount option is invalid, the PV mount fails."
	// Leave mountOptions validation for the mount utility.
	// Seems mount has no problem with duplicate options, so don't bother filtering.
	mountOptions := append([]string{"bind"}, options.MountOptions...)
	if options.ReadOnly {
		mountOptions = append(mountOptions, "ro")
	}

	// Proceed to mount source at target.
	if err := m.Mount(source, target, "", mountOptions); err != nil {
		// TODO: attempt to cleanup
		return err
	}
	klog.Infof("Mounted source %s at target %s", source, target)

	return nil
}

func (m *mounter) Unpublish(target string) error {
	if err := mountutils.CleanupMountPoint(target, m, true); err != nil {
		// See: https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v1.21.0/pkg/driver/mount_linux.go#L146C1-L150C68
		if strings.Contains(fmt.Sprint(err), "not mounted") {
			klog.Infof("Ignoring 'not mounted' error for target %s", target)
		} else {
			return err
		}
	}
	klog.Infof("Cleaned up target %s", target)
	return nil
}

/*func pathExistsAndHealthy(path string) error {
	exists, err := mountutils.PathExists(path)
	if err != nil {
		if mountutils.IsCorruptedMnt(err) {
			return fmt.Errorf("%s is corrupted: %v", path, err)
		}
		return fmt.Errorf("error checking if %s exists: %v", path, err)
	}
	if !exists {
		return fmt.Errorf("%s does not exist", path)
	}
	return nil
}*/

// **********

// GetDeviceNameAndMountsFromMount reads /proc/mounts and returns the device mounted at mountPath,
// any rest mountpoint paths that device is mounted on and an error.
// Modified from k8s.io/mount-utils/mount.getMountRefsByDev
func GetDeviceNameAndMountsFromMount(mounter mountutils.Mounter, mountPath string) (device string, refs []string, err error) {
	mountPath, err = filepath.EvalSymlinks(mountPath)
	if err != nil {
		return
	}

	mps, err := mounter.List()
	if err != nil {
		return
	}

	// Find the device mounted to mountPath.
	// If multiple devices mounted on the same mountPath, only the first one is returned.
	for i := range mps {
		if mps[i].Path == mountPath {
			device = mps[i].Device
			break
		}
	}

	// Find all references to the device.
	// Do not include the mountPath reference.
	for i := range mps {
		if mps[i].Device == device || mps[i].Device == mountPath {
			if mps[i].Path != mountPath {
				refs = append(refs, mps[i].Path)
			}
		}
	}

	return device, refs, nil
}
