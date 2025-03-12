package core

import (
	"fmt"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func InitGVRCache(p provider.Provider) error {
	if GvrCache == nil {
		GvrCache = make(map[string]schema.GroupVersionResource)
	}

	// Let the provider handle caching internally
	// We'll just initialize an empty cache
	return nil
}

func InitResourceSpecs(p provider.Provider) error {
	if ResourceSpecs == nil {
		ResourceSpecs = make(map[string][]string)
	}

	debugLog("Getting OpenAPI resource specs...")
	specs, err := p.GetOpenAPIResourceSpecs()
	if err != nil {
		return fmt.Errorf("error getting resource specs: %w", err)
	}

	debugLog("Got specs for %d resources", len(specs))
	ResourceSpecs = specs

	return nil
}

func (q *QueryExecutor) resourcePropertyName(n *NodePattern) (string, error) {
	var ns string

	gvr, err := q.provider.FindGVR(n.ResourceProperties.Kind)
	if err != nil {
		return "", err
	}

	if n.ResourceProperties.Properties == nil {
		return fmt.Sprintf("%s_%s", Namespace, gvr.Resource), nil
	}

	for _, prop := range n.ResourceProperties.Properties.PropertyList {
		if prop.Key == "namespace" || prop.Key == "metadata.namespace" {
			ns = prop.Value.(string)
			break
		}
	}

	if ns == "" {
		ns = Namespace
	}

	return fmt.Sprintf("%s_%s", ns, gvr.Resource), nil
}

func (q *QueryExecutor) GetOpenAPIResourceSpecs() (map[string][]string, error) {
	specs, err := q.provider.GetOpenAPIResourceSpecs()
	if err != nil {
		return nil, fmt.Errorf("error getting OpenAPI resource specs: %w", err)
	}
	return specs, nil
}
