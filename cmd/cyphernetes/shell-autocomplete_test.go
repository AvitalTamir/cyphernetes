package main

import (
	"reflect"
	"testing"
)

func TestCyphernetesCompleterDo(t *testing.T) {
	completer := &CyphernetesCompleter{}

	tests := []struct {
		name     string
		input    string
		pos      int
		expected []string
		length   int
	}{
		{
			name:     "Empty input",
			input:    "",
			pos:      0,
			expected: []string{},
			length:   0,
		},
		{
			name:     "Keyword suggestion",
			input:    "mat",
			pos:      3,
			expected: []string{"ch"},
			length:   3,
		},
		{
			name:     "JSONPath suggestion",
			input:    "MATCH (p:Pod) RETURN p.meta",
			pos:      26,
			expected: []string{"adata", "adata.name", "adata.namespace", "adata.labels", "adata.annotations", "adata.creationTimestamp", "adata.uid"},
			length:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions, length := completer.Do([]rune(tt.input), tt.pos)

			// Convert [][]rune to []string for easier comparison
			gotSuggestions := make([]string, len(suggestions))
			for i, s := range suggestions {
				gotSuggestions[i] = string(s)
			}

			if !reflect.DeepEqual(gotSuggestions, tt.expected) {
				t.Errorf("Expected suggestions %v, but got %v", tt.expected, gotSuggestions)
			}

			if length != tt.length {
				t.Errorf("Expected length %d, but got %d", tt.length, length)
			}
		})
	}
}

func TestIsMacroContext(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Macro context", ":getpo", true},
		{"Not macro context", "MATCH (n:Pod)", false},
		{"Empty string", "", false},
		{"Colon only", ":", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMacroContext(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, but got %v", tt.expected, result)
			}
		})
	}
}

func TestGetKindForIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		identifier string
		expected   string
	}{
		{"Simple case", "MATCH (p:Pod)", "p", "Pod"},
		{"Multiple identifiers", "MATCH (p:Pod), (d:Deployment)", "d", "Deployment"},
		{"No match", "MATCH (p:Pod)", "x", ""},
		{"Empty line", "", "p", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getKindForIdentifier(tt.line, tt.identifier)
			if result != tt.expected {
				t.Errorf("Expected %q, but got %q", tt.expected, result)
			}
		})
	}
}

func TestIsJSONPathContext(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		pos      int
		lastWord string
		expected bool
	}{
		// {"JSONPath context", "RETURN p.met", 16, "p.metadata", true},
		{"Not JSONPath context", "MATCH (p:Pod)", 12, "Pod", false},
		{"Empty string", "", 0, "", false},
		// {"SET context", "SET p.metadata.name", 19, "p.metadata.name", true},
		{"WHERE context", "WHERE p.metadata.name", 21, "p.metadata.name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isJSONPathContext(tt.line, tt.pos, tt.lastWord)
			if result != tt.expected {
				t.Errorf("Expected %v, but got %v", tt.expected, result)
			}
		})
	}
}

// Mock structures and functions

type MockMacroManager struct {
	Macros map[string]*Macro
}

func (m *MockMacroManager) AddMacro(macro *Macro, overwrite bool) {
	m.Macros[macro.Name] = macro
}

func (m *MockMacroManager) ExecuteMacro(name string, args []string) ([]string, error) {
	return nil, nil
}

func (m *MockMacroManager) LoadMacrosFromFile(filename string) error {
	return nil
}

func (m *MockMacroManager) LoadMacrosFromString(name, content string) error {
	return nil
}

func TestInitResourceSpecs(t *testing.T) {
	// Clear the resourceSpecs map before running the test
	resourceSpecs = make(map[string][]string)

	// Call the function we want to test
	initResourceSpecs()

	// Define the expected resources
	expectedResources := []string{
		"pods",
		"deployments",
		"services",
		"ingresses",
		"replicasets",
		"daemonsets",
		"statefulsets",
		"jobs",
		"cronjobs",
	}

	// Check if all expected resources are present
	for _, resource := range expectedResources {
		if specs, ok := resourceSpecs[resource]; !ok {
			t.Errorf("Expected resource %s not found in resourceSpecs", resource)
		} else if len(specs) == 0 {
			t.Errorf("Resource %s has no specs", resource)
		}
	}

	// Check a few specific paths for each resource type
	testCases := []struct {
		resource string
		path     string
	}{
		{"pods", "$.spec.containers"},
		{"deployments", "$.spec.replicas"},
		{"services", "$.spec.clusterIP"},
		{"ingresses", "$.spec.rules"},
		{"replicasets", "$.spec.selector"},
		{"daemonsets", "$.spec.updateStrategy"},
		{"statefulsets", "$.spec.serviceName"},
		{"jobs", "$.spec.parallelism"},
		{"cronjobs", "$.spec.schedule"},
	}

	for _, tc := range testCases {
		if specs, ok := resourceSpecs[tc.resource]; ok {
			found := false
			for _, spec := range specs {
				if spec == tc.path {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected path %s not found in resource %s", tc.path, tc.resource)
			}
		} else {
			t.Errorf("Resource %s not found in resourceSpecs", tc.resource)
		}
	}

	// Check if metadataJsonPaths are defined
	if len(metadataJsonPaths) == 0 {
		t.Error("metadataJsonPaths is empty")
	}

	// Verify that metadataJsonPaths contains the expected paths
	expectedMetadataPaths := []string{
		"$.metadata",
		"$.metadata.name",
		"$.metadata.namespace",
		"$.metadata.labels",
		"$.metadata.annotations",
		"$.metadata.creationTimestamp",
		"$.metadata.uid",
		"$.status",
	}

	for _, path := range expectedMetadataPaths {
		found := false
		for _, metadataPath := range metadataJsonPaths {
			if metadataPath == path {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected metadata path %s not found in metadataJsonPaths", path)
		}
	}
}
