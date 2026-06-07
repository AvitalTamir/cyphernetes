package core

import "fmt"

func getResourceMetadata(resource map[string]interface{}) (map[string]interface{}, error) {
	metadata, ok := resource["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("metadata is not a map")
	}
	return metadata, nil
}

func getResourceName(metadata map[string]interface{}) (string, error) {
	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("metadata.name is not a non-empty string")
	}
	return name, nil
}

func getResourceKind(resource map[string]interface{}) (string, error) {
	kind, ok := resource["kind"].(string)
	if !ok || kind == "" {
		return "", fmt.Errorf("kind is not a non-empty string")
	}
	return kind, nil
}
