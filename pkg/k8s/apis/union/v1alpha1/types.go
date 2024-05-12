package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VolumeSplit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              VolumeSplitSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type VolumeSplitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []VolumeSplit `json:"items" protobuf:"bytes,2,rep,name=items"`
}

type VolumeSplitSpec struct {
	VolumeName       string                          `json:"volumeName,omitempty" protobuf:"bytes,1,name=volumeName"`
	CapacityTotal    v1.ResourceList                 `json:"capacityTotal,omitempty" protobuf:"bytes,5,name=capacityTotal"`
	AccessModes      []v1.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,4,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`
	Namespace        string                          `json:"namespace,omitempty" protobuf:"bytes,2,name=namespace"`
	StorageClassName *string                         `json:"storageClassName,omitempty" protobuf:"bytes,3,opt,name=storageClassName"`
	Splits           []PersistentVolumeClaimSplit    `json:"splits,omitempty" protobuf:"bytes,6,rep,name=splits"`
}

type PersistentVolumeClaimSplit struct {
	ClaimName string                  `json:"claimName" protobuf:"bytes,1,opt,name=claimName"`
	Resources v1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,name=resources"`
}
