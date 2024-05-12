package union

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	klog "k8s.io/klog/v2"

	v1alpha1 "github.com/on2e/union-csi-driver/pkg/k8s/apis/union/v1alpha1"
	unionclientset "github.com/on2e/union-csi-driver/pkg/k8s/client/clientset"
)

type union struct {
	kubeClient kubernetes.Interface

	claimLister corelisters.PersistentVolumeClaimLister
	nodeLister  corelisters.NodeLister

	splitter Splitter
	attacher Attacher
}

func New(
	kubeClient kubernetes.Interface,
	unionClient unionclientset.Interface,
	claimInformer coreinformers.PersistentVolumeClaimInformer,
	nodeInformer coreinformers.NodeInformer,
	podInformer coreinformers.PodInformer) *union {

	u := union{
		kubeClient:  kubeClient,
		claimLister: claimInformer.Lister(),
		nodeLister:  nodeInformer.Lister(),
		splitter:    NewSplitter(unionClient),
		attacher:    NewAttacher(kubeClient, podInformer.Lister()),
	}

	return &u
}

func (u *union) Run(ctx context.Context) {
	// NOOP
}

func (u *union) CreateLower(ctx context.Context, volumeName string, options *CreateLowerOptions) (*Volume, error) {
	accessModes, err := getAccessModes(options.CSIAccessModes)
	if err != nil {
		return nil, err
	}

	splitSpec := &v1alpha1.VolumeSplitSpec{
		VolumeName:       volumeName,
		CapacityTotal:    v1.ResourceList{v1.ResourceStorage: *getQuantity(options.CapacityBytes)},
		AccessModes:      accessModes,
		Namespace:        options.LowerNamespace,
		StorageClassName: options.LowerStorageClassName,
	}

	split, err := u.splitter.CreateSplit(ctx, volumeName, splitSpec)
	if err != nil {
		return nil, err
	}

	if err := u.createLowerFromSplit(ctx, split); err != nil {
		return nil, err
	}

	// NOTE: we can get lower storage class (informer or/and API) and decide on volumeBindingMode
	// wether we should wait here for lower PVs to get bound to created lower PVCs (Immediate) or return (WaitForFirstConsumer)
	//
	// If lower PVCs are on immediate binding mode, we do a waitForVolume here to poll on the binding state of the claims,
	// i.e. wait for all the lower PVCs to get bound to PVs. If this happens, then we can get and report the actual total capacity
	// by summing up the lowerClaim.Status.Capacity[ResourceStorage] of the claims (or implement a VolumeSplit controller
	// and have him do that by writting it on a VolumeSplit Status field).
	// If on delayed binding mode then report 0 capacity which means that the actual capacity is unknown right now.

	// waitForVolume

	volume := NewVolumeFromVolumeSplit(split)
	volume.CapacityBytes = 0

	return volume, nil
}

func (u *union) createLowerFromSplit(ctx context.Context, split *v1alpha1.VolumeSplit) error {
	/*if split == nil {
		return fmt.Errorf("volume split is nil")
	}*/

	splitName := split.GetName()

	if len(split.Spec.Splits) == 0 {
		return fmt.Errorf("unable to create lower claim(s) because splits is empty in volume split %s", splitName)
	}

	created, total := 0, len(split.Spec.Splits)

	for i := range split.Spec.Splits {
		// Consider adding concurrency here.
		claim, newlyCreated, err := u.createLowerClaimFromSplit(ctx, split, &split.Spec.Splits[i])
		if err != nil {
			return err
		}
		created++
		if newlyCreated {
			klog.Infof("Created lower claim %q (%d/%d)", claimToClaimKey(claim), created, total)
		}
	}

	return nil
}

func (u *union) createLowerClaimFromSplit(ctx context.Context, split *v1alpha1.VolumeSplit, claimSplit *v1alpha1.PersistentVolumeClaimSplit) (*v1.PersistentVolumeClaim, bool, error) {
	var notFound bool

	lowerClaim, err := u.getClaimLocal(split.Spec.Namespace, claimSplit.ClaimName)
	if err != nil {
		// First handle get errors other than IsNotFound that indicate a problem
		// that will mess the incoming create operation so we can exit early.
		notFound = apierrors.IsNotFound(err)
	}

	if notFound {
		lowerClaim = &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      claimSplit.ClaimName,
				Namespace: split.Spec.Namespace,
				//Labels:    map[string]string{},
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes:      split.Spec.AccessModes,
				Resources:        claimSplit.Resources,
				StorageClassName: split.Spec.StorageClassName,
			},
		}

		_, err := u.kubeClient.CoreV1().PersistentVolumeClaims(split.Spec.Namespace).Create(ctx, lowerClaim, metav1.CreateOptions{})
		if err != nil {
			return nil, false, fmt.Errorf("failed to create lower claim %q: %v", claimToClaimKey(lowerClaim), err)
		}
		return lowerClaim, true, nil
	} else {
		klog.Infof("lower claim %q for volume split %q already exists", claimToClaimKey(lowerClaim), split.GetName())
		if !isValidLowerClaimFromSplit(lowerClaim, split) {
			return nil, false, fmt.Errorf("invalid lower claim %q found for volume split %q", claimToClaimKey(lowerClaim), split.GetName())
		}
		return lowerClaim, false, nil
	}
}

// isValidLowerClaimFromSplit checks if claim fields match those from split specification.
func isValidLowerClaimFromSplit(claim *v1.PersistentVolumeClaim, split *v1alpha1.VolumeSplit) bool {
	// TODO: actually implement validation
	return true
}

func (u *union) DeleteLower(ctx context.Context, volumeId string) error {
	split, err := u.splitter.GetSplit(ctx, volumeId)
	if err != nil {
		return err
	}

	if err := u.deleteLowerFromSplit(ctx, split); err != nil {
		return err
	}

	if err := u.splitter.DeleteSplit(ctx, volumeId); err != nil {
		return err
	}

	return nil
}

func (u *union) deleteLowerFromSplit(ctx context.Context, split *v1alpha1.VolumeSplit) error {
	deleted, total := 0, len(split.Spec.Splits)

	for i := range split.Spec.Splits {
		claimSplit := &split.Spec.Splits[i]
		newlyDeleted, err := u.deleteLowerClaimFromSplit(ctx, split, claimSplit)
		if err != nil {
			return err
		}
		deleted++
		if newlyDeleted {
			klog.Infof("Deleted lower claim \"%s/%s\" (%d/%d)", split.Spec.Namespace, claimSplit.ClaimName, deleted, total)
		}
	}

	return nil
}

func (u *union) deleteLowerClaimFromSplit(ctx context.Context, split *v1alpha1.VolumeSplit, claimSplit *v1alpha1.PersistentVolumeClaimSplit) (bool, error) {
	if err := u.kubeClient.CoreV1().PersistentVolumeClaims(split.Spec.Namespace).Delete(ctx, claimSplit.ClaimName, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// * Check that volume with volumeId exists: ErrNotFound -> codes.NotFound
// * Check that node with nodeId exists: ErrNotFound -> codes.NotFound
// * Check that volume is not attached on different node: -> codes.FailedPrecondition
// * Idempotency: Check that volume is attached on node and is compatible with volume capability
func (u *union) AttachLower(ctx context.Context, volumeId, nodeId string) (*VolumeAttachment, error) {
	split, err := u.splitter.GetSplit(ctx, volumeId)
	if err != nil {
		// klog
		return nil, err
	}
	volume := NewVolumeFromVolumeSplit(split)

	if _, err := u.getNodeLocal(nodeId); err != nil {
		// klog
		if apierrors.IsNotFound(err) {
			err = fmt.Errorf("%w: %v", ErrNodeNotFound, err)
		}
		return nil, err
	}

	return u.attacher.Attach(ctx, volume, nodeId)
}

// 1. !attach-pod & volumeId   & nodeId: "volume not attached at node":   ErrAttachmentNotFound -> 0 OK
// 2. !attach-pod & (!volumeId | !nodeId): "volume not attached at node": ErrAttachmentNotFound -> 0 OK
// 3. attach-pod  & !volumeId  & *:                                       ErrVolumeNotFound -> 5 NOT_FOUND
// 4. attach-pod  & *          & !nodeId:                                 ErrNodeNotFound -> 5 NOT_FOUND
func (u *union) DetachLower(ctx context.Context, volumeId, nodeId string) error {
	// VolumeSplit bears the namespace to be used to search for the attach pod,
	// so there is no way now to check for "attachment" if "volume" is not found.
	// This messes up the order in which we want to run the above checks.
	split, err := u.splitter.GetSplit(ctx, volumeId)
	if err != nil {
		// klog
		return err
	}
	volume := NewVolumeFromVolumeSplit(split)

	// This is not in accordance with above checks. Settle with it for now.
	if _, err := u.getNodeLocal(nodeId); err != nil {
		// klog
		if apierrors.IsNotFound(err) {
			err = fmt.Errorf("%w: %v", ErrNodeNotFound, err)
		}
		return err
	}

	return u.attacher.Detach(ctx, volume, nodeId)
}

// getClaimLocal retrieves claim by namespace/name by looking in local cache.
func (u *union) getClaimLocal(namespace, name string) (claim *v1.PersistentVolumeClaim, err error) {
	claim, err = u.claimLister.PersistentVolumeClaims(namespace).Get(name)
	return
}

// getClaimEscalate retrieves claim by namespace/name by first looking in local cache
// and if not found there by getting it from the API server.
func (u *union) getClaimEscalate(ctx context.Context, namespace, name string) (claim *v1.PersistentVolumeClaim, err error) {
	if claim, err = u.claimLister.PersistentVolumeClaims(namespace).Get(name); claim == nil {
		claim, err = u.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	return
}

// getNodeLocal retrieves node by name by looking in local cache.
func (u *union) getNodeLocal(name string) (node *v1.Node, err error) {
	node, err = u.nodeLister.Get(name)
	return
}

// getNodeEscalate retrieves node by name by first looking in local cache
// and if not found there by getting it from the API server.
func (u *union) getNodeEscalate(ctx context.Context, name string) (node *v1.Node, err error) {
	if node, err = u.nodeLister.Get(name); node == nil {
		node, err = u.kubeClient.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	}
	return
}
