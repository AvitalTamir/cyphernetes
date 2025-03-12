package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func createCompatiblePatch(path []string, value interface{}) []interface{} {
	debugLog("Creating patch for path: %v with value: %v\n", path, value)

	// For any path that ends with a key that might contain dots (like labels or annotations),
	// we need to handle it specially to ensure we only update that specific key
	if len(path) >= 2 {
		// Check if we're updating a map field (like labels, annotations, or any other map)
		// We'll detect this by checking if the last part of the path contains dots or slashes
		// which would indicate it's a key in a map
		lastPart := path[len(path)-1]
		if strings.Contains(lastPart, ".") || strings.Contains(lastPart, "/") {
			debugLog("Detected path with dots in last part: %s\n", lastPart)
			// This is likely a key in a map, so we should create a patch that only updates this key
			// Extract the map path (everything except the last part)
			mapPath := path[:len(path)-1]
			jsonMapPath := "/" + strings.Join(mapPath, "/")

			// For array indices, convert [n] to /n
			re := regexp.MustCompile(`\[(\d+)\]`)
			jsonMapPath = re.ReplaceAllString(jsonMapPath, "/$1")

			debugLog("Map path: %s\n", jsonMapPath)

			// Create a patch that uses the "test" operation to check if the map exists
			// If it doesn't, this will fail and we'll need to create it
			testPatch := map[string]interface{}{
				"op":    "test",
				"path":  jsonMapPath,
				"value": map[string]interface{}{},
			}

			// Create a patch that adds just the specific key-value pair to the map
			// Properly escape the key according to JSON Patch spec:
			// '~' is escaped as '~0' and '/' is escaped as '~1'
			escapedKey := strings.ReplaceAll(lastPart, "~", "~0")
			escapedKey = strings.ReplaceAll(escapedKey, "/", "~1")
			// Do NOT escape dots - they are valid in JSON Patch paths
			debugLog("Escaped key: %s\n", escapedKey)

			addPatch := map[string]interface{}{
				"op":    "add",
				"path":  jsonMapPath + "/" + escapedKey,
				"value": value,
			}

			debugLog("Created test patch: %+v\n", testPatch)
			debugLog("Created add patch: %+v\n", addPatch)

			return []interface{}{testPatch, addPatch}
		}
	}

	// For regular paths, we need to ensure all parent paths exist
	// Special handling for common paths that need to be created as empty objects
	// This is a more targeted approach than trying to build every possible path
	if len(path) >= 2 {
		// Check for common paths that might need to be created first
		if path[0] == "metadata" {
			if len(path) >= 3 && (path[1] == "annotations" || path[1] == "labels") {
				// For metadata.labels or metadata.annotations, we need to use a test patch
				// followed by an add patch to trigger the strategic merge patch in the provider
				mapPath := fmt.Sprintf("/metadata/%s", path[1])

				// Create a test patch to check if the map exists
				testPatch := map[string]interface{}{
					"op":    "test",
					"path":  mapPath,
					"value": map[string]interface{}{},
				}

				// Create an add patch for the specific key
				keyPath := fmt.Sprintf("%s/%s", mapPath, path[2])
				addPatch := map[string]interface{}{
					"op":    "add",
					"path":  keyPath,
					"value": value,
				}

				debugLog("Created test patch: %+v\n", testPatch)
				debugLog("Created add patch: %+v\n", addPatch)

				return []interface{}{testPatch, addPatch}
			}
		} else if path[0] == "spec" && len(path) >= 3 {
			if path[1] == "template" && path[2] == "spec" {
				// Handle pod spec paths like spec.template.spec.containers[0].resources
				if len(path) >= 5 && strings.HasPrefix(path[3], "containers[") {
					// Extract the container index
					containerIndexMatch := regexp.MustCompile(`\[(\d+)\]`).FindStringSubmatch(path[3])
					if len(containerIndexMatch) < 2 {
						// If we can't extract the index, fall back to regular patching
						goto DEFAULT_CASE
					}

					containerIndex := containerIndexMatch[1]

					// For container resources, we'll use a special approach
					// Create a single strategic merge patch instead of multiple JSON patches
					// This is more reliable for container resources

					// Create a special patch that signals to the provider to use a strategic merge patch
					specialPatch := map[string]interface{}{
						"op":    "test",
						"path":  "/spec/template/spec/containers",
						"value": []interface{}{},
					}

					// Create a patch that adds the specific property
					// We'll use a special format that the provider will recognize
					// and convert to a strategic merge patch
					containerPath := fmt.Sprintf("/spec/template/spec/containers/%s", containerIndex)

					// Build the path to the specific property
					propertyPath := containerPath
					for i := 4; i < len(path); i++ {
						part := path[i]
						// Handle array indices in the path
						if strings.Contains(part, "[") && strings.Contains(part, "]") {
							part = regexp.MustCompile(`\[(\d+)\]`).ReplaceAllString(part, "/$1")
						}
						propertyPath += "/" + part
					}

					// Create the add patch
					addPatch := map[string]interface{}{
						"op":    "add",
						"path":  propertyPath,
						"value": value,
					}

					debugLog("Created special container patch: %+v\n", specialPatch)
					debugLog("Created add patch: %+v\n", addPatch)

					return []interface{}{specialPatch, addPatch}
				}
			}
		}
	}

DEFAULT_CASE:
	// For all other paths, use a simple add operation
	// This will work if the parent path already exists
	jsonPath := "/" + strings.Join(path, "/")

	// For array indices, convert [n] to /n
	re := regexp.MustCompile(`\[(\d+)\]`)
	jsonPath = re.ReplaceAllString(jsonPath, "/$1")

	debugLog("Regular path: %s\n", jsonPath)

	patch := map[string]interface{}{
		"op":    "add",
		"path":  jsonPath,
		"value": value,
	}

	debugLog("Created regular patch: %+v\n", patch)

	return []interface{}{patch}
}

func setValueAtPath(data interface{}, path string, value interface{}) error {
	// Convert path to array of parts
	parts := strings.Split(strings.TrimPrefix(path, "."), ".")

	// Create compatible patch format
	patches := createCompatiblePatch(parts, value)

	// Apply patches to the data
	if m, ok := data.(map[string]interface{}); ok {
		// First update the in-memory representation
		updateResultMap(m, parts, value)

		// Then apply the JSON patch if needed
		patchJSON, err := json.Marshal(patches)
		if err != nil {
			return fmt.Errorf("error marshalling patches: %s", err)
		}

		// Store the patch for later use if needed
		if metadata, ok := m["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				debugLog("Created patch for %s: %s", name, string(patchJSON))
			}
		}

		return nil
	}

	return fmt.Errorf("data must be a map[string]interface{}, got %T", data)
}

func updateResultMap(resource map[string]interface{}, path []string, value interface{}) {
	current := resource
	for i := 0; i < len(path)-1; i++ {
		part := path[i]
		if current[part] == nil {
			current[part] = make(map[string]interface{})
		}
		if m, ok := current[part].(map[string]interface{}); ok {
			current = m
		} else {
			// If it's not a map, create one
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}
	current[path[len(path)-1]] = value
}

func (q *QueryExecutor) PatchK8sResource(resource map[string]interface{}, patchJSON []byte) error {
	// Get the resource details
	name := resource["metadata"].(map[string]interface{})["name"].(string)
	namespace := ""
	if ns, ok := resource["metadata"].(map[string]interface{})["namespace"]; ok {
		namespace = ns.(string)
	}
	kind := resource["kind"].(string)

	return q.provider.PatchK8sResource(kind, name, namespace, patchJSON)
}

func (q *QueryExecutor) handleSetClause(c *SetClause) error {
	debugLog("Processing %d key-value pairs\n", len(c.KeyValuePairs))

	for i, kvp := range c.KeyValuePairs {
		debugLog("Processing key-value pair %d: %s = %v", i, kvp.Key, kvp.Value)

		// Extract the resource name (first part before any dot)
		parts := strings.SplitN(kvp.Key, ".", 2)
		resultMapKey := parts[0]
		debugLog("Resource name: %s", resultMapKey)

		resources, ok := resultMap[resultMapKey].([]map[string]interface{})
		if !ok {
			return fmt.Errorf("could not find resources for node %s in MATCH clause", resultMapKey)
		}
		debugLog("Found %d resources for node %s", len(resources), resultMapKey)

		// Find the matching node from the stored match nodes
		var nodeKind string
		for _, node := range q.matchNodes {
			if node.ResourceProperties.Name == resultMapKey {
				nodeKind = node.ResourceProperties.Kind
				debugLog("Found kind %s for node %s", nodeKind, resultMapKey)
				break
			}
		}
		if nodeKind == "" {
			return fmt.Errorf("could not find kind for node %s in MATCH clause", resultMapKey)
		}

		for j, resource := range resources {
			debugLog("Processing resource %d of %d", j+1, len(resources))

			if strings.Contains(kvp.Key, "[*]") {
				debugLog("Detected wildcard path: %s", kvp.Key)
				// Handle wildcard updates
				err := applyWildcardUpdate(resource, kvp.Key, kvp.Value)
				if err != nil {
					return err
				}
				debugLog("Successfully applied wildcard update")
			} else {
				debugLog("Processing regular path update")
				// Regular path update
				// First remove the resource name prefix (e.g., "d.")
				remainingPath := ""
				if len(parts) > 1 {
					remainingPath = parts[1]
				}
				debugLog("Remaining path after removing resource prefix: %s", remainingPath)

				// Split the path handling escaped dots
				pathParts := splitEscapedPath(remainingPath)
				debugLog("Path parts after splitting escaped dots: %v", pathParts)

				patches := createCompatiblePatch(pathParts, kvp.Value)
				patchJSON, err := json.Marshal(patches)
				if err != nil {
					return fmt.Errorf("error marshalling patches: %s", err)
				}
				debugLog("Created patch JSON: %s", string(patchJSON))

				metadata, ok := resource["metadata"].(map[string]interface{})
				if !ok {
					return fmt.Errorf("resource metadata is not a map")
				}

				name, ok := metadata["name"].(string)
				if !ok {
					return fmt.Errorf("resource name is not a string")
				}

				namespace := getNamespaceName(metadata)
				debugLog("Resource: %s/%s in namespace %s", nodeKind, name, namespace)
				debugLog("Patch JSON: %s", string(patchJSON))
				debugLog("Current resource state: %+v", resource)

				err = q.provider.PatchK8sResource(nodeKind, name, namespace, patchJSON)
				if err != nil {
					return fmt.Errorf("error patching resource: %s", err)
				}
				debugLog("Successfully applied patch")

				// Verify the patch was applied
				debugLog("Verifying patch was applied")
				updatedResource, err := q.provider.GetK8sResources(nodeKind, fmt.Sprintf("metadata.name=%s", name), "", namespace)
				if err != nil {
					return fmt.Errorf("error verifying patch: %s", err)
				}
				debugLog("Updated resource: %+v", updatedResource)
			}
		}
	}
	return nil
}
