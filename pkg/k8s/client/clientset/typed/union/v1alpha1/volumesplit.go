package v1alpha1

import (
	"context"

	v1alpha1 "github.com/on2e/union-csi-driver/pkg/k8s/apis/union/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	scheme "k8s.io/client-go/kubernetes/scheme"
	rest "k8s.io/client-go/rest"
)

type VolumeSplitsGetter interface {
	VolumeSplits() VolumeSplitInterface
}

type VolumeSplitInterface interface {
	Create(ctx context.Context, volumeSplit *v1alpha1.VolumeSplit, opts metav1.CreateOptions) (*v1alpha1.VolumeSplit, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha1.VolumeSplit, error)
}

type volumeSplits struct {
	client rest.Interface
}

func newVolumeSplits(c *UnionV1alpha1Client) *volumeSplits {
	return &volumeSplits{
		client: c.RESTClient(),
	}
}

func (c *volumeSplits) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1alpha1.VolumeSplit, err error) {
	result = &v1alpha1.VolumeSplit{}
	err = c.client.Get().
		Resource("volumesplits").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

func (c *volumeSplits) Create(ctx context.Context, volumeSplit *v1alpha1.VolumeSplit, opts metav1.CreateOptions) (result *v1alpha1.VolumeSplit, err error) {
	result = &v1alpha1.VolumeSplit{}
	err = c.client.Post().
		Resource("volumesplits").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(volumeSplit).
		Do(ctx).
		Into(result)
	return
}

func (c *volumeSplits) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("volumesplits").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}
