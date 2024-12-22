package provider

type Provider interface {
	GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error)
	DeleteK8sResources(kind, name, namespace string) error
	CreateK8sResource(kind, name, namespace string, body interface{}) error
	PatchK8sResource(kind, name, namespace string, body interface{}) error
}
