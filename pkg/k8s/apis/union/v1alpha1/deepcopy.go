package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (in *VolumeSplit) DeepCopyInto(out *VolumeSplit) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

func (in *VolumeSplit) DeepCopy() *VolumeSplit {
	if in == nil {
		return nil
	}
	out := new(VolumeSplit)
	in.DeepCopyInto(out)
	return out
}

func (in *VolumeSplit) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *VolumeSplitSpec) DeepCopyInto(out *VolumeSplitSpec) {
	*out = *in
	if in.AccessModes != nil {
		in, out := &in.AccessModes, &out.AccessModes
		*out = make([]v1.PersistentVolumeAccessMode, len(*in))
		copy(*out, *in)
	}
	if in.StorageClassName != nil {
		in, out := &in.StorageClassName, &out.StorageClassName
		*out = new(string)
		**out = **in
	}
	if in.Splits != nil {
		in, out := &in.Splits, &out.Splits
		*out = make([]PersistentVolumeClaimSplit, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.CapacityTotal.DeepCopyInto(&out.CapacityTotal)
}

func (in *VolumeSplitSpec) DeepCopy() *VolumeSplitSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeSplitSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *PersistentVolumeClaimSplit) DeepCopyInto(out *PersistentVolumeClaimSplit) {
	*out = *in
	in.Resources.DeepCopyInto(&out.Resources)
}

func (in *PersistentVolumeClaimSplit) DeepCopy() *PersistentVolumeClaimSplit {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeClaimSplit)
	in.DeepCopyInto(out)
	return out
}

func (in *VolumeSplitList) DeepCopyInto(out *VolumeSplitList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeSplit, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *VolumeSplitList) DeepCopy() *VolumeSplitList {
	if in == nil {
		return nil
	}
	out := new(VolumeSplitList)
	in.DeepCopyInto(out)
	return out
}

func (in *VolumeSplitList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
