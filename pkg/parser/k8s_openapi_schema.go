package parser

import (
	"context"
	"fmt"
	"log"
	"strings"

	openapi_v3 "github.com/google/gnostic/openapiv3"
	"google.golang.org/protobuf/encoding/protojson"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func fetchOpenAPISpecV3() (*openapi_v3.Document, error) {
	// Attempt to create an in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create config: %v", err)
		}
	}

	// Create a discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %v", err)
	}

	// Get the OpenAPIV3Client
	openAPIV3Client := discoveryClient.OpenAPIV3()

	// Fetch the OpenAPI v3 schema paths
	docPaths, err := openAPIV3Client.Paths()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenAPI v3 paths: %v", err)
	}

	var combinedDoc *openapi_v3.Document

	// Combine all OpenAPI v3 schemas
	for path, gv := range docPaths {
		// Fetch schema for each group version
		rawSpec, err := openAPIV3Client.Get(context.Background(), gv)
		if err != nil {
			log.Printf("Failed to fetch OpenAPI v3 schema for path %s: %v", path, err)
			continue
		}

		// Decode rawSpec into openapi_v3.Document
		partialDoc := &openapi_v3.Document{}
		err = protojson.Unmarshal(rawSpec, partialDoc)
		if err != nil {
			log.Printf("Failed to decode OpenAPI v3 schema for path %s: %v", path, err)
			continue
		}

		// Merge the partialDoc into combinedDoc
		if combinedDoc == nil {
			combinedDoc = partialDoc
		} else {
			mergeOpenAPIDocuments(combinedDoc, partialDoc)
		}
	}

	if combinedDoc == nil {
		return nil, fmt.Errorf("no OpenAPI v3 schemas could be fetched")
	}

	return combinedDoc, nil
}

func mergeOpenAPIDocuments(combinedDoc, partialDoc *openapi_v3.Document) {
	// Merge Components.Schemas
	if partialDoc.Components != nil && partialDoc.Components.Schemas != nil {
		if combinedDoc.Components == nil {
			combinedDoc.Components = &openapi_v3.Components{
				Schemas: &openapi_v3.Schemas{},
			}
		}
		if combinedDoc.Components.Schemas.AdditionalProperties == nil {
			combinedDoc.Components.Schemas.AdditionalProperties = []*openapi_v3.NamedSchema{}
		}
		combinedDoc.Components.Schemas.AdditionalProperties = append(
			combinedDoc.Components.Schemas.AdditionalProperties,
			partialDoc.Components.Schemas.AdditionalProperties...,
		)
	}
}

func ExtractResourceSpecsV3(doc *openapi_v3.Document) (map[string][]string, error) {
	resourceSpecs := make(map[string][]string)

	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil, fmt.Errorf("no components or schemas found in OpenAPI v3 document")
	}

	for _, namedSchema := range doc.Components.Schemas.AdditionalProperties {
		definitionName := namedSchema.Name
		schemaOrRef := namedSchema.Value

		if !isTopLevelResource(definitionName) {
			continue
		}

		resolvedSchema, err := resolveSchemaOrReferenceV3(doc, schemaOrRef)
		if err != nil {
			log.Printf("Failed to resolve schema for %s: %v", definitionName, err)
			continue
		}

		pathsSet := make(map[string]struct{})
		collectPathsUniqueV3(doc, resolvedSchema, "$", pathsSet)

		// Convert the set to a slice
		paths := make([]string, 0, len(pathsSet))
		for path := range pathsSet {
			paths = append(paths, path)
		}

		gvrKey := extractGVRKeyFromDefinitionName(definitionName)
		if gvrKey == "" {
			log.Printf("Skipping (invalid GVR key): %s", definitionName)
			continue
		}

		log.Printf("Adding resourceSpec: %s", gvrKey)
		resourceSpecs[gvrKey] = paths
	}

	return resourceSpecs, nil
}

func collectPathsUniqueV3(doc *openapi_v3.Document, schema *openapi_v3.Schema, currentPath string, pathsSet map[string]struct{}) {
	if schema == nil {
		return
	}

	// Add the current path if it's not the root
	if currentPath != "$" {
		pathsSet[currentPath] = struct{}{}
		log.Printf("Added path: %s", currentPath)
	}

	// Handle properties
	if schema.Properties != nil {
		for _, namedSchema := range schema.Properties.AdditionalProperties {
			propName := namedSchema.Name
			propSchemaOrRef := namedSchema.Value
			resolvedPropSchema, err := resolveSchemaOrReferenceV3(doc, propSchemaOrRef)
			if err == nil {
				nextPath := fmt.Sprintf("%s.%s", currentPath, propName)
				collectPathsUniqueV3(doc, resolvedPropSchema, nextPath, pathsSet)
			}
		}
	}

	// Handle items (arrays)
	if schema.Items != nil && schema.Items.SchemaOrReference != nil {
		resolvedItemSchema, err := resolveSchemaOrReferenceV3(doc, schema.Items.SchemaOrReference)
		if err == nil {
			nextPath := fmt.Sprintf("%s[]", currentPath)
			collectPathsUniqueV3(doc, resolvedItemSchema, nextPath, pathsSet)
		}
	}

	// Handle additionalProperties (maps)
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.SchemaOrReference != nil {
		resolvedAdditionalSchema, err := resolveSchemaOrReferenceV3(doc, schema.AdditionalProperties.SchemaOrReference)
		if err == nil {
			nextPath := fmt.Sprintf("%s.*", currentPath)
			collectPathsUniqueV3(doc, resolvedAdditionalSchema, nextPath, pathsSet)
		}
	}
}

func resolveSchemaOrReferenceV3(doc *openapi_v3.Document, schemaOrRef *openapi_v3.SchemaOrReference) (*openapi_v3.Schema, error) {
	if schemaOrRef == nil {
		return nil, fmt.Errorf("schemaOrRef is nil")
	}

	if schemaOrRef.Value != nil {
		return schemaOrRef.Value, nil
	}

	if schemaOrRef.Reference != nil {
		ref := schemaOrRef.Reference.XRef

		// Resolve the reference
		if strings.HasPrefix(ref, "#/components/schemas/") {
			refName := strings.TrimPrefix(ref, "#/components/schemas/")

			// Find the schema in components/schemas
			if doc != nil && doc.Components != nil && doc.Components.Schemas != nil {
				for _, namedSchema := range doc.Components.Schemas.AdditionalProperties {
					if namedSchema.Name == refName {
						return resolveSchemaOrReferenceV3(doc, namedSchema.Value)
					}
				}
			}
		}

		return nil, fmt.Errorf("schema not found for reference: %s", ref)
	}

	return nil, fmt.Errorf("unable to resolve schema or reference")
}

func GetOpenAPIDocumentV3() (*openapi_v3.Document, error) {
	doc, err := fetchOpenAPISpecV3()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenAPI v3 document: %v", err)
	}
	return doc, nil
}

func isTopLevelResource(definitionName string) bool {
	// For debugging purposes, allow all definitions
	return true
}

func extractGVRKeyFromDefinitionName(definitionName string) string {
	// Possible formats:
	// "io.k8s.api.<group>.<version>.<Kind>"
	// "io.k8s.<group>.<version>.<Kind>"
	parts := strings.Split(definitionName, ".")
	if len(parts) < 5 {
		return ""
	}

	// Determine the starting index of the group
	startIndex := 2 // Default start index if format is "io.k8s.<group>.<version>.<Kind>"
	if parts[1] == "k8s" && parts[2] == "api" {
		// If format is "io.k8s.api.<group>.<version>.<Kind>"
		startIndex = 3
	}

	group := strings.Join(parts[startIndex:len(parts)-2], ".")
	version := parts[len(parts)-2]
	kind := parts[len(parts)-1]

	// If group is "core", set it to empty string
	if group == "core" {
		group = ""
	}

	// Convert kind to resource name (plural, lowercase)
	resource := kindToResourceName(kind)

	// Construct GVR key in the format "group/version/resource"
	gvrKey := fmt.Sprintf("%s/%s/%s", group, version, resource)
	log.Printf("Definition Name: %s, GVR Key: %s", definitionName, gvrKey)
	return gvrKey
}

func kindToResourceName(kind string) string {
	// Known mappings of Kind to Resource name
	kindToResourceMap := map[string]string{
		"Endpoints":     "endpoints",
		"EndpointSlice": "endpointslices",
		"Namespace":     "namespaces",
		"Node":          "nodes",
		"Ingress":       "ingresses",
		"NetworkPolicy": "networkpolicies",
		"Pod":           "pods",
		"Service":       "services",
		"Secret":        "secrets",
		// Add more known exceptions as needed
	}

	if resource, ok := kindToResourceMap[kind]; ok {
		return resource
	}

	// Fallback to simple pluralization
	resource := strings.ToLower(kind)
	if strings.HasSuffix(resource, "s") || strings.HasSuffix(resource, "x") || strings.HasSuffix(resource, "z") || strings.HasSuffix(resource, "ch") || strings.HasSuffix(resource, "sh") {
		resource += "es"
	} else if strings.HasSuffix(resource, "y") && !strings.HasSuffix(resource, "ay") && !strings.HasSuffix(resource, "ey") && !strings.HasSuffix(resource, "iy") && !strings.HasSuffix(resource, "oy") && !strings.HasSuffix(resource, "uy") {
		resource = strings.TrimSuffix(resource, "y") + "ies"
	} else {
		resource += "s"
	}

	return resource
}
