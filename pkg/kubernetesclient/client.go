package kubernetesclient

import (
	dyclient "k8s.io/client-go/dynamic"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
)

type KubernetesClient struct {
	DynamicClient dyclient.Interface
	discoveryClient *discovery.DiscoveryClient
}

// NewInClusterKubernetesClient initializes a dynamic client using in-cluster config
func NewInClusterKubernetesClient() (*KubernetesClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		 return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	dynClient, err := dyclient.NewForConfig(config)
	if err != nil {
		 return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)

	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}
	return &KubernetesClient{DynamicClient: dynClient, discoveryClient: discoveryClient}, nil
}

func (c *KubernetesClient) IsResourceNamespaced(apiVersion, kind string) (bool, error) {
	apiResourceLists, err := c.discoveryClient.ServerResourcesForGroupVersion(apiVersion)

	if err != nil {
		return false, fmt.Errorf("error getting API resource list: %v", err)
	}

	for _, res := range apiResourceLists.APIResources {

		if res.Kind == kind {
			if res.Namespaced {
				return true, nil
			} else {
				return false, nil
			}
		}
	}

	return false, fmt.Errorf("resource kind %s not found in API version %s", kind, apiVersion)
}

// GVRFromAPIVersionKind returns the GroupVersionResource for the given apiVersion and kind using discoveryClient.
func (c *KubernetesClient) GVRFromAPIVersionKind(apiVersion, kind string) (schema.GroupVersionResource, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	resourceList, err := c.discoveryClient.ServerResourcesForGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	for _, res := range resourceList.APIResources {
		if res.Kind == kind && !strings.Contains(res.Name, "/") {
			return schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: res.Name,
			}, nil
		}
	}
	return schema.GroupVersionResource{}, fmt.Errorf("resource for kind %s not found in %s", kind, apiVersion)
}
