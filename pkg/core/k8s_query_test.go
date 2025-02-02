package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/AvitalTamir/jsonpath"
	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MockProvider implements the Provider interface for testing
type MockProvider struct{}

func (m *MockProvider) GetK8sResources(kind string, fieldSelector string, labelSelector string, namespace string) (interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (m *MockProvider) DeleteK8sResources(kind string, name string, namespace string) error {
	return nil
}

func (m *MockProvider) CreateK8sResource(kind string, name string, namespace string, spec interface{}) error {
	return nil
}

func (m *MockProvider) PatchK8sResource(kind string, name string, namespace string, patchJSON []byte) error {
	return nil
}

func (m *MockProvider) FindGVR(kind string) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, nil
}

func (m *MockProvider) GetOpenAPIResourceSpecs() (map[string][]string, error) {
	return map[string][]string{}, nil
}

func (m *MockProvider) CreateProviderForContext(context string) (provider.Provider, error) {
	return &MockProvider{}, nil
}

func TestJsonPath(t *testing.T) {
	jsonString := `{
		"d": [
		  {
			"apiVersion": "apps/v1",
			"kind": "Deployment",
			"metadata": {
			  "annotations": {
				"deployment.kubernetes.io/revision": "2"
			  },
			  "creationTimestamp": "2023-08-23T10:58:46Z",
			  "generation": 2,
			  "labels": {
				"app": "nginx",
				"service": "test",
				"squad": "DevOps"
			  },
			  "managedFields": [
				{
				  "apiVersion": "apps/v1",
				  "fieldsType": "FieldsV1",
				  "fieldsV1": {
					"f:metadata": {
					  "f:labels": {
						".": {},
						"f:app": {}
					  }
					},
					"f:spec": {
					  "f:progressDeadlineSeconds": {},
					  "f:replicas": {},
					  "f:revisionHistoryLimit": {},
					  "f:selector": {},
					  "f:strategy": {
						"f:rollingUpdate": {
						  ".": {},
						  "f:maxSurge": {},
						  "f:maxUnavailable": {}
						},
						"f:type": {}
					  },
					  "f:template": {
						"f:metadata": {
						  "f:labels": {
							".": {},
							"f:app": {}
						  }
						},
						"f:spec": {
						  "f:containers": {
							"k:{\"name\":\"nginx\"}": {
							  ".": {},
							  "f:image": {},
							  "f:imagePullPolicy": {},
							  "f:name": {},
							  "f:resources": {},
							  "f:terminationMessagePath": {},
							  "f:terminationMessagePolicy": {}
							}
						  },
						  "f:dnsPolicy": {},
						  "f:restartPolicy": {},
						  "f:schedulerName": {},
						  "f:securityContext": {},
						  "f:terminationGracePeriodSeconds": {}
						}
					  }
					}
				  },
				  "manager": "kubectl-create",
				  "operation": "Update",
				  "time": "2023-08-23T10:58:46Z"
				},
				{
				  "apiVersion": "apps/v1",
				  "fieldsType": "FieldsV1",
				  "fieldsV1": {
					"f:metadata": {
					  "f:labels": {
						"f:service": {},
						"f:squad": {}
					  }
					},
					"f:spec": {
					  "f:template": {
						"f:metadata": {
						  "f:labels": {
							"f:service": {},
							"f:squad": {}
						  }
						}
					  }
					}
				  },
				  "manager": "kubectl-edit",
				  "operation": "Update",
				  "time": "2023-08-23T11:00:53Z"
				},
				{
				  "apiVersion": "apps/v1",
				  "fieldsType": "FieldsV1",
				  "fieldsV1": {
					"f:metadata": {
					  "f:annotations": {
						".": {},
						"f:deployment.kubernetes.io/revision": {}
					  }
					},
					"f:status": {
					  "f:availableReplicas": {},
					  "f:conditions": {
						".": {},
						"k:{\"type\":\"Available\"}": {
						  ".": {},
						  "f:lastTransitionTime": {},
						  "f:lastUpdateTime": {},
						  "f:message": {},
						  "f:reason": {},
						  "f:status": {},
						  "f:type": {}
						},
						"k:{\"type\":\"Progressing\"}": {
						  ".": {},
						  "f:lastTransitionTime": {},
						  "f:lastUpdateTime": {},
						  "f:message": {},
						  "f:reason": {},
						  "f:status": {},
						  "f:type": {}
						}
					  },
					  "f:observedGeneration": {},
					  "f:readyReplicas": {},
					  "f:replicas": {},
					  "f:updatedReplicas": {}
					}
				  },
				  "manager": "kube-controller-manager",
				  "operation": "Update",
				  "subresource": "status",
				  "time": "2023-11-13T00:25:11Z"
				}
			  ],
			  "name": "nginx",
			  "namespace": "default",
			  "resourceVersion": "645286846",
			  "uid": "9156338c-76f2-4249-8006-d9bb1af8304d"
			},
			"spec": {
			  "progressDeadlineSeconds": 600,
			  "replicas": 1,
			  "revisionHistoryLimit": 10,
			  "selector": {
				"matchLabels": {
				  "app": "nginx"
				}
			  },
			  "strategy": {
				"rollingUpdate": {
				  "maxSurge": "25%",
				  "maxUnavailable": "25%"
				},
				"type": "RollingUpdate"
			  },
			  "template": {
				"metadata": {
				  "creationTimestamp": null,
				  "labels": {
					"app": "nginx",
					"service": "test",
					"squad": "DevOps"
				  }
				},
				"spec": {
				  "containers": [
					{
					  "image": "nginx",
					  "imagePullPolicy": "Always",
					  "name": "nginx",
					  "resources": {},
					  "terminationMessagePath": "/dev/termination-log",
					  "terminationMessagePolicy": "File"
					}
				  ],
				  "dnsPolicy": "ClusterFirst",
				  "restartPolicy": "Always",
				  "schedulerName": "default-scheduler",
				  "securityContext": {},
				  "terminationGracePeriodSeconds": 30
				}
			  }
			},
			"status": {
			  "availableReplicas": 1,
			  "conditions": [
				{
				  "lastTransitionTime": "2023-08-23T10:58:46Z",
				  "lastUpdateTime": "2023-08-23T11:00:55Z",
				  "message": "ReplicaSet \"nginx-5bd7f9f864\" has successfully progressed.",
				  "reason": "NewReplicaSetAvailable",
				  "status": "True",
				  "type": "Progressing"
				},
				{
				  "lastTransitionTime": "2023-11-13T00:25:11Z",
				  "lastUpdateTime": "2023-11-13T00:25:11Z",
				  "message": "Deployment has minimum availability.",
				  "reason": "MinimumReplicasAvailable",
				  "status": "True",
				  "type": "Available"
				}
			  ],
			  "observedGeneration": 2,
			  "readyReplicas": 1,
			  "replicas": 1,
			  "updatedReplicas": 1
			}
		  }
		]
	  }`

	testQueries := []string{"$.d", "$.d[0]", "$.d.#", "$.d[0].kind", "$.d[*].name"}

	for _, query := range testQueries {
		var jsonData interface{}
		json.Unmarshal([]byte(jsonString), &jsonData)
		_, err := jsonpath.JsonPathLookup(jsonData, query)
		if err != nil {
			fmt.Println("Error executing jsonpath: ", err)
			t.Fail()
		}
	}
}

func TestConvertToMilliCPU(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{"Valid milliCPU", "100m", 100, false},
		{"Valid standard CPU", "1", 1000, false},
		{"Valid standard CPU", "2", 2000, false},
		{"Valid standard CPU", "1.5", 1500, false},
		{"Valid standard CPU", "3.7", 3700, false},
		{"Valid standard CPU", "0.1", 100, false},
		{"Valid standard CPU", "1.234", 1234, false},
		{"Valid standard CPU", "0.001", 1, false},
		{"Invalid milliCPU", "100x", 0, true},
		{"invalid format", "abc", 0, true},
		{"Zero milliCPU", "0m", 0, false},
		{"Zero standard CPU", "0", 0, false},
		{"Empty string", "", 0, true},
		{"milliCPU format", "500m", 500, false},
		{"milliCPU format", "1000m", 1000, false},
		{"large number in standard CPU", "100", 100000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToMilliCPU(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToMilliCPU() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("convertToMilliCPU() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConvertMilliCPUToStandard(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"Less than 1000m", 500, "500m"},
		{"Exactly 1000m", 1000, "1"},
		{"More than 1000m, whole number", 2000, "2"},
		{"More than 1000m, decimal", 1500, "1.5"},
		{"Large number", 5678, "5.678"},
		{"Large number", 12340, "12.34"},
		{"Zero", 0, "0m"},
		{"Very small number", 1, "1m"},
		{"Very large number", 10000, "10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMilliCPUToStandard(tt.input)
			if result != tt.expected {
				t.Errorf("convertMilliCPUToStandard(%d) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertMemoryToBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		{"Exabyte", "1E", 1e18, false},
		{"Petabyte", "1P", 1e15, false},
		{"Terabyte", "1T", 1e12, false},
		{"Gigabyte", "1G", 1e9, false},
		{"Megabyte", "1M", 1e6, false},
		{"Kilobyte", "1k", 1e3, false},
		{"Exbibyte", "1Ei", 1 << 60, false},
		{"Pebibyte", "1Pi", 1 << 50, false},
		{"Tebibyte", "1Ti", 1 << 40, false},
		{"Gibibyte", "1Gi", 1 << 30, false},
		{"Mebibyte", "1Mi", 1 << 20, false},
		{"Kibibyte", "1Ki", 1 << 10, false},
		{"Bytes", "1000", 1000, false},
		{"Invalid", "1X", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertMemoryToBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertMemoryToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("convertMemoryToBytes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConvertBytesToMemory(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		// Binary (power-of-two) units
		{"Exbibyte", 1 << 60, "1Ei"},
		{"Pebibyte", 1 << 50, "1Pi"},
		{"Tebibyte", 1 << 40, "1Ti"},
		{"Gibibyte", 1 << 30, "1Gi"},
		{"Mebibyte", 1 << 20, "1Mi"},
		{"Kibibyte", 1 << 10, "1Ki"},
		{"Exabyte", 1e18, "1E"},
		{"Petabyte", 1e15, "1P"},
		{"Terabyte", 1e12, "1T"},
		{"Gigabyte", 1e9, "1G"},
		{"Megabyte", 1e6, "1M"},
		{"Kilobyte", 1e3, "1k"},
		{"Zero bytes", 0, "0"},
		{"One byte", 1, "1"},
		{"999 bytes", 999, "999"},
		{"1.5 Gibibytes", 1610612736, "1.5Gi"},
		{"1.5 Mebibytes", 1572864, "1.5Mi"},
		{"1.5 Kibibytes", 1536, "1.5Ki"},
		{"Large binary", 1125899906842624, "1Pi"},
		{"Large decimal", 1000000000000000, "1P"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertBytesToMemory(tt.input)
			if result != tt.expected {
				t.Errorf("convertBytesToMemory(%d) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    reflect.Value
		expected []string
		wantErr  bool
	}{
		{
			name:     "String slice",
			input:    reflect.ValueOf([]string{"a", "b", "c"}),
			expected: []string{"a", "b", "c"},
			wantErr:  false,
		},
		{
			name:     "Int slice",
			input:    reflect.ValueOf([]int{1, 2, 3}),
			expected: []string{"1", "2", "3"},
			wantErr:  false,
		},
		{
			name:     "Not a slice",
			input:    reflect.ValueOf("not a slice"),
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToStringSlice(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToStringSlice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("convertToStringSlice() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSumMilliCPU(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected int
		wantErr  bool
	}{
		{"Valid milliCPU", []string{"100m", "200m", "300m"}, 600, false},
		{"Valid standard CPU", []string{"1", "2", "3"}, 6000, false},
		{"Mixed formats", []string{"1", "2000m", "3"}, 6000, false},
		{"Invalid input", []string{"1", "2", "invalid"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sumMilliCPU(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("sumMilliCPU() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("sumMilliCPU() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSumMemoryBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected int64
		wantErr  bool
	}{
		{"Valid memory units", []string{"1Gi", "2Mi", "3Ki"}, 1075842048, false},
		{"Mixed units 1", []string{"1G", "2M", "3k"}, 1002003000, false},
		{"Mixed units 2", []string{"1Ei", "2Ti", "3Mi", "1P", "2G", "3k"}, 1153923705633251256, false},
		{"Mixed units 3", []string{"1Pi", "2Gi", "3Ki", "1E", "2T", "3M"}, 1001127902057329344, false},
		{"Bytes", []string{"1000", "2000", "3000"}, 6000, false},
		{"Invalid input", []string{"1Gi", "2Mi", "invalid", "5K"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sumMemoryBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("sumMemoryBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("sumMemoryBytes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// MockRelationshipResolver is used for testing
type MockRelationshipResolver struct {
	potentialKindsByNode map[string][]string
}

func (m *MockRelationshipResolver) FindPotentialKindsIntersection(relationships []*Relationship) []string {
	// If no mapping is provided, return empty slice
	if len(m.potentialKindsByNode) == 0 {
		return []string{}
	}

	// Find the first kindless node and get its potential kinds
	var result []string
	for _, rel := range relationships {
		if rel.LeftNode.ResourceProperties.Kind == "" {
			if kinds, ok := m.potentialKindsByNode[rel.LeftNode.ResourceProperties.Name]; ok {
				result = kinds
				break
			}
		}
		if rel.RightNode.ResourceProperties.Kind == "" {
			if kinds, ok := m.potentialKindsByNode[rel.RightNode.ResourceProperties.Name]; ok {
				result = kinds
				break
			}
		}
	}

	// For each additional kindless node, intersect its potential kinds with the result
	for _, rel := range relationships {
		if rel.LeftNode.ResourceProperties.Kind == "" {
			if kinds, ok := m.potentialKindsByNode[rel.LeftNode.ResourceProperties.Name]; ok {
				if result == nil {
					result = kinds
				} else {
					result = intersectKinds(result, kinds)
				}
			}
		}
		if rel.RightNode.ResourceProperties.Kind == "" {
			if kinds, ok := m.potentialKindsByNode[rel.RightNode.ResourceProperties.Name]; ok {
				if result == nil {
					result = kinds
				} else {
					result = intersectKinds(result, kinds)
				}
			}
		}
	}

	return result
}

// Helper function to find intersection of two string slices
func intersectKinds(a, b []string) []string {
	set := make(map[string]bool)
	for _, k := range a {
		set[k] = true
	}

	var result []string
	for _, k := range b {
		if set[k] {
			result = append(result, k)
		}
	}
	return result
}

func TestRewriteQueryForKindlessNodes(t *testing.T) {
	mockProvider := &MockProvider{}
	tests := []struct {
		name          string
		query         string
		mockKinds     map[string][]string
		expectedQuery string
		expectedError bool
		errorContains string
	}{
		{
			name:          "No kindless nodes",
			query:         "MATCH (d:Deployment)->(p:Pod) RETURN d, p",
			mockKinds:     map[string][]string{},
			expectedQuery: "",
			expectedError: false,
		},
		{
			name:          "Single kindless node with one potential kind",
			query:         "MATCH (d:Deployment)->(x) RETURN d, x",
			mockKinds:     map[string][]string{"x": {"Pod"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod) RETURN d__exp__0, x__exp__0",
			expectedError: false,
		},
		{
			name:          "Single kindless node with multiple potential kinds",
			query:         "MATCH (d:Deployment)->(x) RETURN d, x",
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) RETURN d__exp__0, x__exp__0, d__exp__1, x__exp__1",
			expectedError: false,
		},
		{
			name:          "Multiple kindless nodes with same potential kind",
			query:         "MATCH (d:Deployment)->(x), (s:Service)->(x) RETURN d, s, x",
			mockKinds:     map[string][]string{"x": {"Pod"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (s__exp__0:Service)->(x__exp__0:Pod) RETURN d__exp__0, s__exp__0, x__exp__0",
			expectedError: false,
		},
		{
			name:          "Multiple kindless nodes with different potential kinds",
			query:         "MATCH (d:Deployment)->(x), (s:Service)->(y) RETURN d, s, x, y",
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}, "y": {"Pod", "Endpoints"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (s__exp__0:Service)->(y__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet), (s__exp__1:Service)->(y__exp__1:Endpoints) RETURN d__exp__0, s__exp__0, x__exp__0, y__exp__0, d__exp__1, s__exp__1, x__exp__1, y__exp__1",
			expectedError: false,
		},
		{
			name:          "Multiple kindless nodes with intersecting potential kinds",
			query:         "MATCH (d:Deployment)->(x), (s:Service)->(x) RETURN d, s, x",
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: "MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (s__exp__0:Service)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet), (s__exp__1:Service)->(x__exp__1:ReplicaSet) RETURN d__exp__0, s__exp__0, x__exp__0, d__exp__1, s__exp__1, x__exp__1",
			expectedError: false,
		},
		{
			name:          "Kindless node with properties",
			query:         `MATCH (d:Deployment)->(x {name: "test"}) RETURN d, x`,
			mockKinds:     map[string][]string{"x": {"Pod"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod {name: "test"}) RETURN d__exp__0, x__exp__0`,
			expectedError: false,
		},
		{
			name:          "Match/Set/Return with multiple potential kinds",
			query:         `MATCH (d:Deployment)->(x) SET x.metadata.labels.foo = "bar" RETURN d, x`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) SET x__exp__0.metadata.labels.foo = "test", x__exp__1.metadata.labels.foo = "test" RETURN d__exp__0, x__exp__0, d__exp__1, x__exp__1`,
			expectedError: false,
		},
		{
			name:          "Match/Where/Return with node properties and multiple potential kinds",
			query:         `MATCH (d:Deployment)->(x {name: "test"}) WHERE x.metadata.labels.foo = "bar" RETURN d, x`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod {name: "test"}), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet {name: "test"}) WHERE x__exp__0.metadata.labels.foo = "bar", x__exp__1.metadata.labels.foo = "bar" RETURN d__exp__0, x__exp__0, d__exp__1, x__exp__1`,
			expectedError: false,
		},
		{
			name:          "Match/Delete with node properties and multiple potential kinds",
			query:         `MATCH (d:Deployment)->(x {name: "test"}) DELETE x`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod {name: "test"}), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet {name: "test"}) DELETE x__exp__0, x__exp__1`,
			expectedError: false,
		},
		{
			name:          "Match/Return with aggregation",
			query:         `MATCH (d:Deployment)->(x) RETURN COUNT {d}, SUM {x}`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) RETURN COUNT {d__exp__0}, SUM {x__exp__0}, COUNT {d__exp__1}, SUM {x__exp__1}`,
			expectedError: false,
		},
		{
			name:          "Multiple kindless nodes with different potential kinds returning a mixture of aggregation and non-aggregation with properties",
			query:         `MATCH (d:Deployment {name: "test"})->(x), (s:Service)->(y) RETURN d, COUNT {s}, x, SUM {y.spec.replicas}`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}, "y": {"Pod", "Endpoints"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (s__exp__0:Service)->(y__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet), (s__exp__1:Service)->(y__exp__1:Endpoints) RETURN d__exp__0, COUNT {s__exp__0}, x__exp__0, SUM {y__exp__0.spec.replicas}, d__exp__1, COUNT {s__exp__1}, x__exp__1, SUM {y__exp__1.spec.replicas}`,
			expectedError: false,
		},
		{
			name:          "No potential kinds found",
			query:         "MATCH (d:Deployment)->(x) RETURN d, x",
			mockKinds:     map[string][]string{},
			expectedError: true,
			errorContains: "unable to determine kind for nodes in relationship",
		},
		{
			name:          "No relationships for kindless node",
			query:         "MATCH (x) RETURN x",
			mockKinds:     map[string][]string{},
			expectedError: true,
			errorContains: "kindless nodes may only be used in a relationship",
		},
		{
			name:          "Kindless-to-kindless relationship",
			query:         "MATCH (x)->(y) RETURN x, y",
			mockKinds:     map[string][]string{},
			expectedError: true,
			errorContains: "chaining two unknown nodes (kindless-to-kindless) is not supported",
		},
		{
			name:          "Match/Return with AS aliases",
			query:         `MATCH (d:Deployment)->(x) RETURN d.metadata.name AS deployment_name, x.spec.replicas AS replica_count`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) RETURN d__exp__0.metadata.name AS deployment_name, x__exp__0.spec.replicas AS replica_count, d__exp__1.metadata.name AS deployment_name, x__exp__1.spec.replicas AS replica_count`,
			expectedError: false,
		},
		{
			name:          "Match/Return with mixed AS aliases and aggregations",
			query:         `MATCH (d:Deployment)->(x) RETURN d.metadata.name AS deployment_name, COUNT {x} AS pod_count`,
			mockKinds:     map[string][]string{"x": {"Pod", "ReplicaSet"}},
			expectedQuery: `MATCH (d__exp__0:Deployment)->(x__exp__0:Pod), (d__exp__1:Deployment)->(x__exp__1:ReplicaSet) RETURN d__exp__0.metadata.name AS deployment_name, COUNT {x__exp__0} AS pod_count, d__exp__1.metadata.name AS deployment_name, COUNT {x__exp__1} AS pod_count`,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock
			mockFindPotentialKinds = func(relationships []*Relationship) []string {
				resolver := &MockRelationshipResolver{potentialKindsByNode: tt.mockKinds}
				return resolver.FindPotentialKindsIntersection(relationships)
			}

			// Parse the original query
			ast, err := ParseQuery(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse query: %v", err)
			}

			// Create a query executor with mock providermake
			executor, err := NewQueryExecutor(mockProvider)
			if err != nil {
				t.Fatalf("Failed to create query executor: %v", err)
			}

			// Call rewriteQueryForKindlessNodes
			rewrittenAst, err := executor.rewriteQueryForKindlessNodes(ast)

			// Check error expectations
			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// For queries that don't need rewriting
			if tt.expectedQuery == "" {
				if rewrittenAst != nil {
					t.Error("Expected no rewrite, but got a rewritten AST")
				}
				return
			}

			// Parse the expected query for comparison
			expectedAst, err := ParseQuery(tt.expectedQuery)
			if err != nil {
				t.Fatalf("Failed to parse expected query: %v", err)
			}

			// Debug output
			t.Logf("\nTest case: %s\nExpected AST: %+v\nGot AST: %+v", tt.name, expectedAst, rewrittenAst)

			// Compare the ASTs
			if !reflect.DeepEqual(rewrittenAst, expectedAst) {
				// Print more detailed comparison
				if len(rewrittenAst.Clauses) != len(expectedAst.Clauses) {
					t.Errorf("Number of clauses don't match. Expected %d, got %d", len(expectedAst.Clauses), len(rewrittenAst.Clauses))
				}

				for i, clause := range expectedAst.Clauses {
					if i >= len(rewrittenAst.Clauses) {
						t.Errorf("Missing clause at index %d", i)
						continue
					}

					switch c := clause.(type) {
					case *MatchClause:
						if mc, ok := rewrittenAst.Clauses[i].(*MatchClause); ok {
							t.Logf("Match clause comparison:\nExpected: %+v\nGot: %+v", c, mc)
						} else {
							t.Errorf("Expected MatchClause at index %d, got %T", i, rewrittenAst.Clauses[i])
						}
					case *ReturnClause:
						if rc, ok := rewrittenAst.Clauses[i].(*ReturnClause); ok {
							t.Logf("Return clause comparison:\nExpected: %+v\nGot: %+v", c, rc)
						} else {
							t.Errorf("Expected ReturnClause at index %d, got %T", i, rewrittenAst.Clauses[i])
						}
					}
				}
			}
		})
	}
}
