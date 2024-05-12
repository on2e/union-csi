package pod

import (
	v1 "k8s.io/api/core/v1"
)

// Factory produces attach pods
type Factory struct{}

func NewFactory() *Factory {
	return &Factory{}
}

// Create creates a new attach pod
func (f *Factory) Create(
	podName string,
	podNamespace string,
	claimNames []string,
	hostPath string,
	volumeId string) *v1.Pod {
	return NewBuilder(podName, podNamespace, claimNames, hostPath, volumeId).Build()
}
