package provider

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Provider defines the interface for different backend implementations
type Provider interface {
	// Resource Operations
	// The mutating operations take a dryRun flag: when true the change is sent
	// to the backend in dry-run mode (validated but not persisted). Dry-run is a
	// per-call property, so the same provider can serve dry-run and real calls
	// concurrently.
	GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error)
	DeleteK8sResources(kind, name, namespace string, dryRun bool) error
	CreateK8sResource(kind, name, namespace string, body interface{}, dryRun bool) error
	PatchK8sResource(kind, name, namespace string, patchJSON []byte, dryRun bool) error

	// Schema Operations
	FindGVR(kind string) (schema.GroupVersionResource, error)
	GetOpenAPIResourceSpecs() (map[string][]string, error)
	CreateProviderForContext(context string) (Provider, error)
}
