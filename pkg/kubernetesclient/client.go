package kubernetesclient

import (
	"context"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	dyclient "k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesClient struct {
	DynamicClient dyclient.Interface
	discoveryClient *discovery.DiscoveryClient
}

// NewInClusterKubernetesClient initializes a dynamic client using in-cluster config
func NewInClusterKubernetesClient() (*KubernetesClient, error) {
       var config *rest.Config
       var err error
       kubeconfig := os.Getenv("KUBECONFIG")
       if kubeconfig != "" {
	       config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	       if err != nil {
		       return nil, fmt.Errorf("failed to build config from KUBECONFIG: %w", err)
	       }
       } else {
	       config, err = rest.InClusterConfig()
	       if err != nil {
		       return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	       }
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

func (c *KubernetesClient) GetTopmostControllerOwner(res *unstructured.Unstructured) (*unstructured.Unstructured, error) {

	owners := res.GetOwnerReferences()

	for _, ownerRef := range owners {

		if ownerRef.Controller != nil && *ownerRef.Controller {
			gvr, isNamespaced, err := c.gvrFromAPIVersionKind(ownerRef.APIVersion, ownerRef.Kind)

			if err != nil {
				return nil, fmt.Errorf("error getting GVR from apiVersion/kind: %v", err)
			}

			var ownerRes *unstructured.Unstructured

			if isNamespaced {
				// If the owner is namespaced, we need to get it from the same namespace
				ownerRes, err = c.DynamicClient.Resource(gvr).Namespace(res.GetNamespace()).Get(context.TODO(), ownerRef.Name, metav1.GetOptions{})
			} else {
				ownerRes, err = c.DynamicClient.Resource(gvr).Get(context.TODO(), ownerRef.Name, metav1.GetOptions{})
			}

			if err != nil {
				return nil, fmt.Errorf("error getting owner resource %s/%s: %v", ownerRef.Kind, ownerRef.Name, err)
			}

			// Recursively get the topmost owner
			return c.GetTopmostControllerOwner(ownerRes)
		}
	}

	return res, nil
}

// GVRFromAPIVersionKind returns the GroupVersionResource for the given apiVersion and kind using discoveryClient.
// It also returns a boolean indicating if the resource is namespaced.
func (c *KubernetesClient) gvrFromAPIVersionKind(apiVersion, kind string) (schema.GroupVersionResource, bool, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}
	resourceList, err := c.discoveryClient.ServerResourcesForGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}
	for _, res := range resourceList.APIResources {
		if res.Kind == kind && !strings.Contains(res.Name, "/") {
			return schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: res.Name,
			}, res.Namespaced, nil
		}
	}

	return schema.GroupVersionResource{}, false, fmt.Errorf("resource for kind %s not found in %s", kind, apiVersion)
}
