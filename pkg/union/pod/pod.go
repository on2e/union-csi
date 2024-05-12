package pod

import (
	"fmt"
	"path/filepath"
	"strconv"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// The mergerfs-wrapped image to use
	image = "docker.io/on2e/gogomergerfs:demo-mergerfs2.37.1"
	// The image entrypoint
	commandName = "gogomergerfs"
	// The image command as a string to be formatted with flag values and fed to a shell
	commandString = "gogomergerfs mergerfs --branches=%s --target=%s --block"
)

// Builder contains information to build attach pods
type Builder struct {
	// provided
	podName      string
	podNamespace string
	claimNames   []string
	hostPath     string
	// derived
	containerPath string
}

func NewBuilder(
	podName string,
	podNamespace string,
	claimNames []string,
	hostPath string,
	volumeId string) *Builder {
	return &Builder{
		podName:       podName,
		podNamespace:  podNamespace,
		claimNames:    claimNames,
		hostPath:      hostPath,
		containerPath: filepath.Join("/volume", volumeId),
	}
}

// Build builds the attach pod
func (b *Builder) Build() *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.podName,
			Namespace: b.podNamespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  commandName,
					Image: image,
					// PullAlways for now, we are on development and we expect different,
					// experimental versions of the image with the same tag
					ImagePullPolicy: v1.PullAlways,
				},
			},
		},
	}

	container := &pod.Spec.Containers[0]

	// Run container in privileged mode
	privileged := true
	container.SecurityContext = &v1.SecurityContext{Privileged: &privileged}

	// Add volumes
	b.addPVCVolumesAndVolumeMounts(&pod.Spec.Volumes, &container.VolumeMounts)
	b.addHostPathVolumeAndVolumeMount(&pod.Spec.Volumes, &container.VolumeMounts)

	// The paths to merge together
	// NOTE: let mergerfs handle globbing: https://github.com/trapexit/mergerfs/tree/2.37.1#globbing
	branches := filepath.Join(b.containerPath, "branches/\\*")
	// The directory to mount the union of the branches
	target := filepath.Join(b.containerPath, "merged")

	container.Command = []string{"/bin/sh"}
	container.Args = []string{
		"-c",
		fmt.Sprintf(commandString, branches, target),
	}

	return pod
}

// addPVCVolumesAndVolumeMounts adds persistentVolumeClaimVolumeSource volumes in pod volumes
// using Builder.claimNames and matches them to container volumeMounts
func (b *Builder) addPVCVolumesAndVolumeMounts(volumes *[]v1.Volume, volumeMounts *[]v1.VolumeMount) {
	for i, claimName := range b.claimNames {
		volumeName := "branch" + strconv.Itoa(i)
		// NOTE: do not put claimName at the start of each branch dir
		// mergerfs removes the common prefix of the branches in the device name of the union mount,
		// e.g. branches: branch1:branch2 -> device name: 1:2
		mountPath := filepath.Join(b.containerPath, "/branches", volumeName+"-"+claimName)

		*volumes = append(*volumes, v1.Volume{
			Name: volumeName,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: claimName,
				},
			},
		})

		*volumeMounts = append(*volumeMounts, v1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		})
	}
}

// addHostPathVolumeAndVolumeMount adds a hostPathVolumeSource volume in pod volumes
// using Builder.hostPath and matches it to container volumeMounts
func (b *Builder) addHostPathVolumeAndVolumeMount(volumes *[]v1.Volume, volumeMounts *[]v1.VolumeMount) {
	volumeName := "target"
	mountPath := filepath.Join(b.containerPath, "/merged")

	dirOrCreate := v1.HostPathDirectoryOrCreate
	*volumes = append(*volumes, v1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: b.hostPath,
				Type: &dirOrCreate,
			},
		},
	})

	// Bidirectional so the union mount can be exposed back on the host
	// and from there to consumer containers via bind-mounts
	bidir := v1.MountPropagationBidirectional
	*volumeMounts = append(*volumeMounts, v1.VolumeMount{
		Name:             volumeName,
		MountPath:        mountPath,
		MountPropagation: &bidir,
	})
}
