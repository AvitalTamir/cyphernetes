package core

import (
	"fmt"
	"sort"
	"strings"

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

func (q *QueryExecutor) resourceFetchKey(n *NodePattern, namespace, fieldSelector, labelSelector string, extraFilters []*Filter) (string, error) {
	gvr, err := tryResolveGVR(q.provider, n.ResourceProperties.Kind)
	if err != nil {
		return "", err
	}

	properties := []string{}
	if n.ResourceProperties.Properties != nil {
		for _, prop := range n.ResourceProperties.Properties.PropertyList {
			properties = append(properties, fmt.Sprintf("%s=%#v", prop.Key, prop.Value))
		}
	}
	sort.Strings(properties)

	return strings.Join([]string{
		namespace,
		gvr.Group,
		gvr.Version,
		gvr.Resource,
		n.ResourceProperties.Name,
		fieldSelector,
		labelSelector,
		strings.Join(properties, ","),
		filterSignature(extraFilters),
	}, "\x00"), nil
}

func (q *QueryExecutor) GetOpenAPIResourceSpecs() (map[string][]string, error) {
	specs, err := q.provider.GetOpenAPIResourceSpecs()
	if err != nil {
		return nil, fmt.Errorf("error getting OpenAPI resource specs: %w", err)
	}
	return specs, nil
}
