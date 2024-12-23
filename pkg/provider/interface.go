package provider

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// Provider defines the interface for different backend implementations
type Provider interface {
	// Resource Operations
	GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error)
	DeleteK8sResources(kind, name, namespace string) error
	CreateK8sResource(kind, name, namespace string, body interface{}) error
	PatchK8sResource(kind, name, namespace string, body interface{}) error

	// Schema Operations
	FindGVR(kind string) (schema.GroupVersionResource, error)
	GetOpenAPIResourceSpecs() (map[string][]string, error)
	CreateProviderForContext(context string) (Provider, error)

	// Client getters
	GetDiscoveryClient() (discovery.DiscoveryInterface, error)
	GetClientset() (kubernetes.Interface, error)
	GetDynamicClient() (dynamic.Interface, error)
	GetGVRCache() (map[string]schema.GroupVersionResource, error)

	// Cache management methods
	PrintCache() string
	ClearCache() error
}
