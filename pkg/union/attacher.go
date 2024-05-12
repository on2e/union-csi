package union

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	wait "k8s.io/apimachinery/pkg/util/wait"
	kubernetes "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	klog "k8s.io/klog/v2"

	pod "github.com/on2e/union-csi-driver/pkg/union/pod"
)

// attacher implements the Attacher interface.
// Attach/Detach methods create/delete attach pods.
// Attach pods are pods that use the PersistentVolumeClaims of an "upper" volume to trigger
// the attachment and mounting of the corresponding "lower" volumes on the target node,
// mount them to a mergerfs-based container, merge them together and serve the union mount
// back on the host via a HostPath volume.
// Note that this union mount, now accessible on the node for subsequent bind-mounts on consumer
// containers, is the storage asset behind a Union PersistentVolume.
type attacher struct {
	kubeClient kubernetes.Interface
	podLister  corelisters.PodLister
	podFactory *pod.Factory
}

var _ Attacher = &attacher{}

func NewAttacher(kubeClient kubernetes.Interface, podLister corelisters.PodLister) *attacher {
	return &attacher{
		kubeClient: kubeClient,
		podLister:  podLister,
		podFactory: pod.NewFactory(),
	}
}

func (a *attacher) Attach(ctx context.Context, volume *Volume, nodeId string) (*VolumeAttachment, error) {
	podName := makeAttachPodName(volume.VolumeId)
	podKey := volume.Namespace + "/" + podName

	pod, err := a.podLister.Pods(volume.Namespace).Get(podName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("error getting pod %q: %v", podKey, err)
	}

	hostPath := makeHostPath(volume.VolumeId)

	if pod == nil {
		// Create a new attach pod
		pod = a.podFactory.Create(podName, volume.Namespace, volume.ClaimNames, hostPath, volume.VolumeId)
		// TODO: find the right place and mechanism to apply the nodeSelector
		pod.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeId}

		_, err = a.kubeClient.CoreV1().Pods(volume.Namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err == nil {
			klog.Infof("Created attach pod %q for volume %q at node %q", podKey, volume.VolumeId, nodeId)
		} else if apierrors.IsAlreadyExists(err) {
			klog.Infof("Attach pod %q for volume %q already exists", podKey, volume.VolumeId)
		} else {
			return nil, fmt.Errorf("error creating pod %q: %v", podKey, err)
		}
	} else {
		klog.Infof("Attach pod %q for volume %q already exists", podKey, volume.VolumeId)
	}

	klog.Infof("Start waiting for attachment of volume %q at node %q", volume.VolumeId, nodeId)
	attachment, err := a.waitForAttach(ctx, volume.VolumeId, nodeId, podName, volume.Namespace)
	if attachment != nil {
		// NOTE: this should be better detected and set in waitForAttach
		attachment.HostPath = hostPath
	}
	return attachment, err
}

func (a *attacher) waitForAttach(ctx context.Context, volumeId, expectedNodeId, podName, podNamespace string) (*VolumeAttachment, error) {
	var pod *v1.Pod
	var err error

	podKey := podNamespace + "/" + podName
	// Semi-random values
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.5,
		Steps:    12,
	}

	waitForAttachFunc := func(ctx context.Context) (bool, error) {
		pod, err = a.podLister.Pods(podNamespace).Get(podName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("error getting pod %q: %v", podKey, err)
			}
			// Pod may have not made it in local cache yet, return nil error to continue waiting
			return false, nil
		}
		// NOTE: can also embed pod status reason and message fields on "pod not running yet" errors/logs

		// If pod is terminating, stop waiting for it with an error
		if isPodTerminating(pod) {
			klog.Infof("Attach pod %q for volume %q is terminating, stop waiting for attachment", podKey, volumeId)
			return false, fmt.Errorf("attach pod %q for volume %q is terminating", podKey, volumeId)
		}

		// If pod is not running on a node yet, return nil error to continue waiting
		if !isPodRunning(pod) {
			klog.Infof("Attach pod %q for volume %q is not running on a node yet, continue waiting for attachment ...", podKey, volumeId)
			return false, nil
		}

		// If pod is running on a different node, stop waiting for it with ErrVolumeInUse error.
		// Currently, assume volume is attached on that other node.
		if pod.Spec.NodeName != expectedNodeId {
			klog.Infof("Attach pod %q for volume %q is running on node %q, expected %q, stop waiting for attachment", podKey, volumeId, pod.Spec.NodeName, expectedNodeId)
			return true, ErrVolumeInUse
		}

		return true, nil
	}

	// Consider using google.golang.org/grpc/codes.DeadlineExceeded
	waitErr := wait.ExponentialBackoffWithContext(ctx, backoff, waitForAttachFunc)
	if waitErr != nil && !errors.Is(waitErr, ErrVolumeInUse) {
		return nil, waitErr
	}

	return &VolumeAttachment{VolumeId: volumeId, NodeId: pod.Spec.NodeName}, waitErr
}

func (a *attacher) Detach(ctx context.Context, volume *Volume, nodeId string) error {
	podName := makeAttachPodName(volume.VolumeId)
	podKey := volume.Namespace + "/" + podName

	pod, err := a.podLister.Pods(volume.Namespace).Get(podName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Currently, assume volume is not attached at node if attach pod does not exist
			klog.Infof("Attach pod %q for volume %q does not exist", podKey, volume.VolumeId)
			return fmt.Errorf("%w: %w", ErrAttachmentNotFound, err)
		} else {
			return fmt.Errorf("error getting pod %q: %v", podKey, err)
		}
	}

	if len(pod.Spec.NodeName) == 0 {
		klog.Infof("Attach pod %q for volume %q is not scheduled on a node yet, will not attempt to delete", podKey, volume.VolumeId)
		return ErrAttachmentNotFound
	}

	if pod.Spec.NodeName != nodeId {
		// Currently, assume volume is attached on that other node
		klog.Infof("Attach pod %q for volume %q found on node %q, expected %q, will not attempt to delete", podKey, volume.VolumeId, pod.Spec.NodeName, nodeId)
		return ErrAttachmentNotFound
	}

	if pod.DeletionTimestamp == nil {
		err := a.kubeClient.CoreV1().Pods(volume.Namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		if err == nil {
			klog.Infof("Deleted attach pod %q for volume %q at node %q", podKey, volume.VolumeId, nodeId)
		} else if apierrors.IsNotFound(err) {
			klog.Infof("Attach pod %q for volume %q does not exist", podKey, volume.VolumeId)
			return fmt.Errorf("%w: %w", ErrAttachmentNotFound, err)
		} else {
			return fmt.Errorf("error deleting pod %q: %v", podKey, err)
		}
	}

	klog.Infof("Start waiting for detachment of volume %q at node %q", volume.VolumeId, nodeId)
	return a.waitForDetach(ctx, volume.VolumeId, nodeId, podName, volume.Namespace)
}

func (a *attacher) waitForDetach(ctx context.Context, volumeId, expectedNodeId, podName, podNamespace string) error {
	podKey := podNamespace + "/" + podName
	// Semi-random values
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2.0,
		Steps:    12,
	}

	waitForDetachFunc := func(ctx context.Context) (bool, error) {
		pod, err := a.podLister.Pods(podNamespace).Get(podName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("error getting pod %q: %v", podKey, err)
			}
			// Pod is removed from API, deletion is complete, stop waiting
			return true, nil
		}

		// We already checked for that in Detach() ...
		if pod.Spec.NodeName != expectedNodeId {
			klog.Infof("Attach pod %q for volume %q found on node %q, expected %q, stop waiting for detachment", podKey, volumeId, pod.Spec.NodeName, expectedNodeId)
			return false, ErrAttachmentNotFound
		}

		// If pod is not removed from API server yet, return nil error to continue waiting
		klog.Infof("Attach pod %q for volume %q is not removed from API server yet, continue waiting for detachment ...", podKey, volumeId)
		return false, nil
	}

	return wait.ExponentialBackoffWithContext(ctx, backoff, waitForDetachFunc)
}

// makeAttachPodName returns attach-pod-<sha256(volumeId)>
func makeAttachPodName(volumeId string) string {
	result := sha256.Sum256([]byte(volumeId))
	return fmt.Sprintf("attach-pod-%x", result)
}

// makeHostPath
func makeHostPath(volumeId string) string {
	// TODO: make driver provide this dir
	return filepath.Join("/var/lib/union-csi-driver.union.io/volumes", volumeId, "merged")
}

// isPodTerminated checks if pod is terminating.
// Does not check if pod is nil.
func isPodTerminating(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodFailed || pod.Status.Phase == v1.PodSucceeded
}

// isPodRunning checks if pod has a non-empty NodeName and a Running phase.
// Does not check if pod is nil.
func isPodRunning(pod *v1.Pod) bool {
	return len(pod.Spec.NodeName) > 0 && pod.Status.Phase == v1.PodRunning
}
