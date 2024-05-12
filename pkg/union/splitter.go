package union

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klog "k8s.io/klog/v2"

	v1alpha1 "github.com/on2e/union-csi-driver/pkg/k8s/apis/union/v1alpha1"
	unionclientset "github.com/on2e/union-csi-driver/pkg/k8s/client/clientset"
)

type Splitter interface {
	CreateSplit(context.Context, string, *v1alpha1.VolumeSplitSpec) (*v1alpha1.VolumeSplit, error)
	DeleteSplit(context.Context, string) error
	GetSplit(context.Context, string) (*v1alpha1.VolumeSplit, error)
}

type splitter struct {
	unionClient unionclientset.Interface
	// Implement a VolumeSplit informer and use it here.

	claimNamePrefix string
}

func NewSplitter(unionClient unionclientset.Interface, options ...SplitterOption) *splitter {
	s := &splitter{
		unionClient: unionClient,
	}

	for _, o := range options {
		o(s)
	}

	return s
}

// If VolumeSplit with splitName does not already exist, CreateSplit creates it.
// If VolumeSplit with same splitName already exists and has the same parameters,
// CreateSplit gets the existing VolumeSplit.
// If VolumeSplit with same splitName but different parameters already exists,
// ErrIdempotentParameterMismatch error is returned.
func (s *splitter) CreateSplit(ctx context.Context, volumeId string, splitSpec *v1alpha1.VolumeSplitSpec) (split *v1alpha1.VolumeSplit, err error) {
	var notFound bool

	splitName := s.makeSplitName(volumeId)

	split, err = s.GetSplit(ctx, splitName)
	if err != nil {
		// First handle Get errors other than IsNotFound that indicate a problem
		// that will mess the incoming Create so we can exit early.
		notFound = apierrors.IsNotFound(err)
	}

	if notFound {
		split, err = s.doCreateSplit(ctx, volumeId, splitName, splitSpec)
		if err == nil {
			return split, nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
		// First, Get returned IsNotFound and then Create returned IsAlreadyExists ...
		// Make one last attempt to get the split.
		if split, err = s.GetSplit(ctx, splitName); err != nil {
			// This short timeframe mismatch is interesting so log about it.
			klog.Infof("Failed to get VolumeSplit %q for volume %q after already-exists indication: %v", splitName, volumeId, err)
			// Maybe special error?
			return nil, err
		}
	} else {
		klog.Infof("VolumeSplit %q for volume %q already exists", splitName, volumeId)
	}

	if !isSplitSpecCompatible(&split.Spec, splitSpec) {
		// fmt.Errorf("%w: %v", ErrIdempotencyIncompatible, err)
		return nil, ErrIdempotencyIncompatible
	}

	return split, nil
}

func (s *splitter) doCreateSplit(ctx context.Context, volumeId, splitName string, splitSpec *v1alpha1.VolumeSplitSpec) (split *v1alpha1.VolumeSplit, err error) {
	defer func() {
		if err == nil {
			klog.Infof("Created VolumeSplit %q for volume %q", splitName, volumeId)
		} else if apierrors.IsAlreadyExists(err) {
			klog.Infof("VolumeSplit %q for volume %q already exists", splitName, volumeId)
		} else {
			klog.Infof("Error creating VolumeSplit %q: %v", splitName, err)
		}
	}()

	split = &v1alpha1.VolumeSplit{
		ObjectMeta: metav1.ObjectMeta{Name: splitName},
		Spec:       *splitSpec,
	}

	capacityQty := split.Spec.CapacityTotal[v1.ResourceStorage]

	for i, c := range s.splitQuantitySuperDuperDummy(&capacityQty) {
		claimName := s.makeClaimName(split.Spec.VolumeName, i)
		claimSplit := v1alpha1.PersistentVolumeClaimSplit{
			ClaimName: claimName,
			Resources: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceStorage: *c}},
		}
		split.Spec.Splits = append(split.Spec.Splits, claimSplit)
	}

	_, err = s.unionClient.UnionV1alpha1().VolumeSplits().Create(ctx, split, metav1.CreateOptions{})
	return
}

// TODO: return err instead of bool to provide informative ErrIdempotencyIncompatible error messages.
func isSplitSpecCompatible(oldSpec, newSpec *v1alpha1.VolumeSplitSpec) bool {
	// Check for compatibility only for non-nil specs.
	if oldSpec == nil || newSpec == nil {
		return false
	}

	// Hmm...
	//if oldSpec.VolumeName != newSpec.VolumeName {
	//	return false
	//}
	if oldSpec.Namespace != newSpec.Namespace {
		return false
	}
	if oldSpec.StorageClassName == nil && newSpec.StorageClassName == nil {
		// NOOP
	} else if oldSpec.StorageClassName != nil && newSpec.StorageClassName != nil {
		if *oldSpec.StorageClassName != *newSpec.StorageClassName {
			return false
		}
	} else {
		return false
	}

	newSize := newSpec.CapacityTotal[v1.ResourceStorage]
	oldSize := oldSpec.CapacityTotal[v1.ResourceStorage]
	if newSize.Cmp(oldSize) > 0 {
		return false
	}

	// Check if every access mode in new spec is present in existing spec.
	isAccessModesCompatible := func(oldModes, newModes []v1.PersistentVolumeAccessMode) bool {
		m := map[v1.PersistentVolumeAccessMode]bool{}
		for _, mode := range oldModes {
			m[mode] = true
		}
		for _, mode := range newModes {
			if !m[mode] {
				return false
			}
		}
		return true
	}

	return isAccessModesCompatible(oldSpec.AccessModes, newSpec.AccessModes)
}

func (s *splitter) splitQuantitySuperDuperDummy(q *resource.Quantity) []*resource.Quantity {
	// TODO: make splitting lossless when quantity is odd
	v, f := q.Value()/2, q.Format
	return []*resource.Quantity{resource.NewQuantity(v, f), resource.NewQuantity(v, f)}
}

func (s *splitter) DeleteSplit(ctx context.Context, volumeId string) (err error) {
	splitName := s.makeSplitName(volumeId)

	defer func() {
		if err == nil {
			klog.Infof("Deleted VolumeSplit %q for volume %q", splitName, volumeId)
		} else if apierrors.IsNotFound(err) {
			klog.Infof("VolumeSplit %q for volume %q does not exist", splitName, volumeId)
			err = fmt.Errorf("%w: %w", ErrVolumeNotFound, err)
		} else {
			klog.Infof("Error deleting VolumeSplit %q: %v", splitName, err)
		}
	}()

	err = s.unionClient.UnionV1alpha1().VolumeSplits().Delete(ctx, splitName, metav1.DeleteOptions{})
	return
}

func (s *splitter) GetSplit(ctx context.Context, volumeId string) (split *v1alpha1.VolumeSplit, err error) {
	defer func() {
		if err != nil && apierrors.IsNotFound(err) {
			err = fmt.Errorf("%w: %w", ErrVolumeNotFound, err)
		}
	}()

	splitName := s.makeSplitName(volumeId)
	// Try informer cache first when implemented.
	split, err = s.unionClient.UnionV1alpha1().VolumeSplits().Get(ctx, splitName, metav1.GetOptions{})
	return
}

func (s *splitter) makeSplitName(volumeId string) string {
	return volumeId + "-split"
}

func (s *splitter) makeClaimName(name string, index int) string {
	if s.claimNamePrefix != "" {
		return fmt.Sprintf("%s-%s-lower%d", s.claimNamePrefix, name, index)
	}
	return fmt.Sprintf("%s-lower%d", name, index)
}

type SplitterOption func(s *splitter)

func WithClaimNamePrefix(prefix string) SplitterOption {
	return func(s *splitter) {
		s.claimNamePrefix = prefix
	}
}
