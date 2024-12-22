package parser

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func init() {
	LogLevel = "debug"
}

func TestRecursiveParser(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Expression
		wantErr bool
	}{
		{
			name:  "simple match return",
			input: "MATCH (pod:Pod) RETURN pod.metadata.name",
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "pod",
									Kind: "Pod",
								},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "pod.metadata.name"},
						},
					},
				},
			},
		},
		{
			name:  "match with where clause",
			input: `MATCH (pod:Pod) WHERE pod.metadata.name = "nginx" RETURN pod`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "pod",
									Kind: "Pod",
								},
							},
						},
						ExtraFilters: []*KeyValuePair{
							{
								Key:      "pod.metadata.name",
								Value:    "nginx",
								Operator: "EQUALS",
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "pod"},
						},
					},
				},
			},
		},
		{
			name:  "match with properties",
			input: `MATCH (d:deploy { service: "foo", app: "bar"}), (s:Service {service: "foo", app: "bar"}) RETURN s.spec.ports, d.metadata.name`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "d",
									Kind: "deploy",
									Properties: &Properties{
										PropertyList: []*Property{
											{Key: "service", Value: "foo"},
											{Key: "app", Value: "bar"},
										},
									},
								},
							},
							{
								ResourceProperties: &ResourceProperties{
									Name: "s",
									Kind: "Service",
									Properties: &Properties{
										PropertyList: []*Property{
											{Key: "service", Value: "foo"},
											{Key: "app", Value: "bar"},
										},
									},
								},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "s.spec.ports"},
							{JsonPath: "d.metadata.name"},
						},
					},
				},
			},
		},
		{
			name:  "match with relationship",
			input: "MATCH (pod:Pod)->(svc:Service) RETURN pod,svc",
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "pod",
									Kind: "Pod",
								},
							},
							{
								ResourceProperties: &ResourceProperties{
									Name: "svc",
									Kind: "Service",
								},
							},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								LeftNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "pod",
										Kind: "Pod",
									},
								},
								RightNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "svc",
										Kind: "Service",
									},
								},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "pod"},
							{JsonPath: "svc"},
						},
					},
				},
			},
		},
		{
			name:  "match with relationship properties",
			input: `MATCH (pod:Pod)-[r:uses {port: 80}]->(svc:Service) RETURN pod,r,svc`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "pod",
									Kind: "Pod",
								},
							},
							{
								ResourceProperties: &ResourceProperties{
									Name: "svc",
									Kind: "Service",
								},
							},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								ResourceProperties: &ResourceProperties{
									Name: "r",
									Kind: "uses",
									Properties: &Properties{
										PropertyList: []*Property{
											{
												Key:   "port",
												Value: int(80),
											},
										},
									},
								},
								LeftNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "pod",
										Kind: "Pod",
									},
								},
								RightNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "svc",
										Kind: "Service",
									},
								},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "pod"},
							{JsonPath: "r"},
							{JsonPath: "svc"},
						},
					},
				},
			},
		},
		{
			name:  "match with context",
			input: "IN production MATCH (pod:Pod) RETURN pod",
			want: &Expression{
				Contexts: []string{"production"},
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "pod",
									Kind: "Pod",
								},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "pod"},
						},
					},
				},
			},
		},
		{
			name:  "match with array index",
			input: `MATCH (pod:Pod) WHERE pod.spec.containers[0].image = "nginx" RETURN pod`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "pod",
									Kind: "Pod",
								},
							},
						},
						ExtraFilters: []*KeyValuePair{
							{
								Key:      "pod.spec.containers[0].image",
								Value:    "nginx",
								Operator: "EQUALS",
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "pod"},
						},
					},
				},
			},
		},
		{
			name: "create with complex json",
			input: `CREATE (d:Deployment {
				"metadata": {
					"name": "child-of-test",
					"labels": {
						"app": "child-of-test"
					}
				},
				"spec": {
					"selector": {
						"matchLabels": {
							"app": "child-of-test"
						}
					},
					"template": {
						"metadata": {
							"labels": {
								"app": "child-of-test"
							}
						},
						"spec": {
							"containers": [
								{
									"name": "child-of-test",
									"image": "nginx:latest"
								}
							]
						}
					}
				}
			})`,
			want: &Expression{
				Clauses: []Clause{
					&CreateClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "d",
									Kind: "Deployment",
									JsonData: `{
										"metadata": {
											"name": "child-of-test",
											"labels": {
												"app": "child-of-test"
											}
										},
										"spec": {
											"selector": {
												"matchLabels": {
													"app": "child-of-test"
												}
											},
											"template": {
												"metadata": {
													"labels": {
														"app": "child-of-test"
													}
												},
												"spec": {
													"containers": [
														{
															"name": "child-of-test",
															"image": "nginx:latest"
														}
													]
												}
											}
										}
									}`,
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "match and create relationship",
			input: `MATCH (d:Deployment {name: "child-of-test"}) CREATE (d)->(s:Service)`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "d",
									Kind: "Deployment",
									Properties: &Properties{
										PropertyList: []*Property{
											{Key: "name", Value: "child-of-test"},
										},
									},
								},
							},
						},
					},
					&CreateClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "d",
									Kind: "",
								},
							},
							{
								ResourceProperties: &ResourceProperties{
									Name: "s",
									Kind: "Service",
								},
							},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								LeftNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "d",
										Kind: "",
									},
								},
								RightNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "s",
										Kind: "Service",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "match and delete with relationship",
			input: `MATCH (d:Deployment {name: "child-of-test"})->(s:Service) DELETE d, s`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "d",
									Kind: "Deployment",
									Properties: &Properties{
										PropertyList: []*Property{
											{Key: "name", Value: "child-of-test"},
										},
									},
								},
							},
							{
								ResourceProperties: &ResourceProperties{
									Name: "s",
									Kind: "Service",
								},
							},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								LeftNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "d",
										Kind: "Deployment",
										Properties: &Properties{
											PropertyList: []*Property{
												{Key: "name", Value: "child-of-test"},
											},
										},
									},
								},
								RightNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "s",
										Kind: "Service",
									},
								},
							},
						},
					},
					&DeleteClause{
						NodeIds: []string{"d", "s"},
					},
				},
			},
		},
		{
			name:  "match with dashed context names",
			input: "IN kind-kind, kind-kind-prod MATCH (d:deployments) WHERE d.spec.replicas = 1 RETURN d.spec.replicas",
			want: &Expression{
				Contexts: []string{"kind-kind", "kind-kind-prod"},
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "d",
									Kind: "deployments",
								},
							},
						},
						ExtraFilters: []*KeyValuePair{
							{
								Key:      "d.spec.replicas",
								Value:    1,
								Operator: "EQUALS",
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "d.spec.replicas"},
						},
					},
				},
			},
		},
		{
			name:  "match with array wildcard",
			input: `MATCH (d:deployment {name:"auth-service"})->(s:svc)->(p:pod) RETURN SUM { p.spec.containers[*].resources.requests.cpu } AS totalCPUReq`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "d",
									Kind: "deployment",
									Properties: &Properties{
										PropertyList: []*Property{
											{Key: "name", Value: "auth-service"},
										},
									},
								},
							},
							{
								ResourceProperties: &ResourceProperties{
									Name: "s",
									Kind: "svc",
								},
							},
							{
								ResourceProperties: &ResourceProperties{
									Name: "p",
									Kind: "pod",
								},
							},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								LeftNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "d",
										Kind: "deployment",
										Properties: &Properties{
											PropertyList: []*Property{
												{Key: "name", Value: "auth-service"},
											},
										},
									},
								},
								RightNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "s",
										Kind: "svc",
									},
								},
							},
							{
								Direction: Right,
								LeftNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "s",
										Kind: "svc",
									},
								},
								RightNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "p",
										Kind: "pod",
									},
								},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{
								JsonPath:  "p.spec.containers[*].resources.requests.cpu",
								Aggregate: "SUM",
								Alias:     "totalCPUReq",
							},
						},
					},
				},
			},
		},
		{
			name:  "match with COUNT aggregation",
			input: `MATCH (d:Deployment)->(rs:ReplicaSet)->(p:Pod) RETURN COUNT{p} AS TotalPods`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{ResourceProperties: &ResourceProperties{Name: "d", Kind: "Deployment"}},
							{ResourceProperties: &ResourceProperties{Name: "rs", Kind: "ReplicaSet"}},
							{ResourceProperties: &ResourceProperties{Name: "p", Kind: "Pod"}},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								LeftNode:  &NodePattern{ResourceProperties: &ResourceProperties{Name: "d", Kind: "Deployment"}},
								RightNode: &NodePattern{ResourceProperties: &ResourceProperties{Name: "rs", Kind: "ReplicaSet"}},
							},
							{
								Direction: Right,
								LeftNode:  &NodePattern{ResourceProperties: &ResourceProperties{Name: "rs", Kind: "ReplicaSet"}},
								RightNode: &NodePattern{ResourceProperties: &ResourceProperties{Name: "p", Kind: "Pod"}},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{
								JsonPath:  "p",
								Aggregate: "COUNT",
								Alias:     "TotalPods",
							},
						},
					},
				},
			},
		},
		{
			name:  "match with various WHERE operators",
			input: `MATCH (p:Pod) WHERE p.status.phase != "Running", p.metadata.name =~ "^test-.*", p.spec.containers[0].resources.requests.memory CONTAINS "Gi" RETURN p.metadata.name`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{ResourceProperties: &ResourceProperties{Name: "p", Kind: "Pod"}},
						},
						ExtraFilters: []*KeyValuePair{
							{Key: "p.status.phase", Value: "Running", Operator: "NOT_EQUALS"},
							{Key: "p.metadata.name", Value: "^test-.*", Operator: "REGEX_COMPARE"},
							{Key: "p.spec.containers[0].resources.requests.memory", Value: "Gi", Operator: "CONTAINS"},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "p.metadata.name"},
						},
					},
				},
			},
		},
		{
			name:  "match with multiple array wildcards",
			input: `MATCH (d:Deployment)->(p:Pod) RETURN SUM { p.spec.containers[*].volumeMounts[*].name } AS totalMounts`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{ResourceProperties: &ResourceProperties{Name: "d", Kind: "Deployment"}},
							{ResourceProperties: &ResourceProperties{Name: "p", Kind: "Pod"}},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								LeftNode:  &NodePattern{ResourceProperties: &ResourceProperties{Name: "d", Kind: "Deployment"}},
								RightNode: &NodePattern{ResourceProperties: &ResourceProperties{Name: "p", Kind: "Pod"}},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{
								JsonPath:  "p.spec.containers[*].volumeMounts[*].name",
								Aggregate: "SUM",
								Alias:     "totalMounts",
							},
						},
					},
				},
			},
		},
		{
			name:  "match with namespace override",
			input: `MATCH (d:Deployment {namespace: "staging"})->(s:Service {namespace: "staging"}) RETURN d.metadata.name, s.spec.clusterIP`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "d",
									Kind: "Deployment",
									Properties: &Properties{
										PropertyList: []*Property{
											{Key: "namespace", Value: "staging"},
										},
									},
								},
							},
							{
								ResourceProperties: &ResourceProperties{
									Name: "s",
									Kind: "Service",
									Properties: &Properties{
										PropertyList: []*Property{
											{Key: "namespace", Value: "staging"},
										},
									},
								},
							},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								LeftNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "d",
										Kind: "Deployment",
										Properties: &Properties{
											PropertyList: []*Property{
												{Key: "namespace", Value: "staging"},
											},
										},
									},
								},
								RightNode: &NodePattern{
									ResourceProperties: &ResourceProperties{
										Name: "s",
										Kind: "Service",
										Properties: &Properties{
											PropertyList: []*Property{
												{Key: "namespace", Value: "staging"},
											},
										},
									},
								},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "d.metadata.name"},
							{JsonPath: "s.spec.clusterIP"},
						},
					},
				},
			},
		},
		{
			name:  "match with case-insensitive resource kinds",
			input: `MATCH (p:POD), (d:deployment), (s:SVC) RETURN p.metadata.name, d.metadata.name, s.metadata.name`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{ResourceProperties: &ResourceProperties{Name: "p", Kind: "POD"}},
							{ResourceProperties: &ResourceProperties{Name: "d", Kind: "deployment"}},
							{ResourceProperties: &ResourceProperties{Name: "s", Kind: "SVC"}},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "p.metadata.name"},
							{JsonPath: "d.metadata.name"},
							{JsonPath: "s.metadata.name"},
						},
					},
				},
			},
		},
		{
			name:  "match and set multiple fields",
			input: `MATCH (d:Deployment) SET d.spec.replicas = 3, d.metadata.labels.updated = "true", d.spec.template.spec.containers[0].image = "nginx:latest" RETURN d.metadata.name`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{ResourceProperties: &ResourceProperties{Name: "d", Kind: "Deployment"}},
						},
					},
					&SetClause{
						KeyValuePairs: []*KeyValuePair{
							{Key: "d.spec.replicas", Value: 3, Operator: "EQUALS"},
							{Key: "d.metadata.labels.updated", Value: "true", Operator: "EQUALS"},
							{Key: "d.spec.template.spec.containers[0].image", Value: "nginx:latest", Operator: "EQUALS"},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "d.metadata.name"},
						},
					},
				},
			},
		},
		{
			name:  "match and delete multiple nodes",
			input: `MATCH (d:Deployment)->(s:Service)->(i:Ingress) DELETE d, s, i`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{ResourceProperties: &ResourceProperties{Name: "d", Kind: "Deployment"}},
							{ResourceProperties: &ResourceProperties{Name: "s", Kind: "Service"}},
							{ResourceProperties: &ResourceProperties{Name: "i", Kind: "Ingress"}},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								LeftNode:  &NodePattern{ResourceProperties: &ResourceProperties{Name: "d", Kind: "Deployment"}},
								RightNode: &NodePattern{ResourceProperties: &ResourceProperties{Name: "s", Kind: "Service"}},
							},
							{
								Direction: Right,
								LeftNode:  &NodePattern{ResourceProperties: &ResourceProperties{Name: "s", Kind: "Service"}},
								RightNode: &NodePattern{ResourceProperties: &ResourceProperties{Name: "i", Kind: "Ingress"}},
							},
						},
					},
					&DeleteClause{
						NodeIds: []string{"d", "s", "i"},
					},
				},
			},
		},
		{
			name:  "match and create with relationship",
			input: `MATCH (d:Deployment {name: "nginx"}) CREATE (d)->(s:Service) RETURN s.metadata.name`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{
								ResourceProperties: &ResourceProperties{
									Name: "d",
									Kind: "Deployment",
									Properties: &Properties{
										PropertyList: []*Property{
											{Key: "name", Value: "nginx"},
										},
									},
								},
							},
						},
					},
					&CreateClause{
						Nodes: []*NodePattern{
							{ResourceProperties: &ResourceProperties{Name: "d"}},
							{ResourceProperties: &ResourceProperties{Name: "s", Kind: "Service"}},
						},
						Relationships: []*Relationship{
							{
								Direction: Right,
								LeftNode:  &NodePattern{ResourceProperties: &ResourceProperties{Name: "d"}},
								RightNode: &NodePattern{ResourceProperties: &ResourceProperties{Name: "s", Kind: "Service"}},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "s.metadata.name"},
						},
					},
				},
			},
		},
		{
			name:  "match with either relationship direction",
			input: `MATCH (d:Deployment)<-(s:Service) RETURN d.metadata.name, s.metadata.name`,
			want: &Expression{
				Clauses: []Clause{
					&MatchClause{
						Nodes: []*NodePattern{
							{ResourceProperties: &ResourceProperties{Name: "d", Kind: "Deployment"}},
							{ResourceProperties: &ResourceProperties{Name: "s", Kind: "Service"}},
						},
						Relationships: []*Relationship{
							{
								Direction: Left,
								LeftNode:  &NodePattern{ResourceProperties: &ResourceProperties{Name: "d", Kind: "Deployment"}},
								RightNode: &NodePattern{ResourceProperties: &ResourceProperties{Name: "s", Kind: "Service"}},
							},
						},
					},
					&ReturnClause{
						Items: []*ReturnItem{
							{JsonPath: "d.metadata.name"},
							{JsonPath: "s.metadata.name"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseQuery(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				// Special handling for JSON data comparison
				if len(got.Clauses) > 0 && len(tt.want.Clauses) > 0 {
					if createClause, ok := got.Clauses[0].(*CreateClause); ok {
						if wantCreateClause, ok := tt.want.Clauses[0].(*CreateClause); ok {
							if len(createClause.Nodes) > 0 && len(wantCreateClause.Nodes) > 0 {
								// Normalize the JSON data
								var gotJSON, wantJSON interface{}
								if err := json.Unmarshal([]byte(createClause.Nodes[0].ResourceProperties.JsonData), &gotJSON); err == nil {
									if err := json.Unmarshal([]byte(wantCreateClause.Nodes[0].ResourceProperties.JsonData), &wantJSON); err == nil {
										if reflect.DeepEqual(gotJSON, wantJSON) {
											// If the JSON content matches, update the JsonData to match formatting
											createClause.Nodes[0].ResourceProperties.JsonData = wantCreateClause.Nodes[0].ResourceProperties.JsonData
										}
									}
								}
							}
						}
					}
				}

				// Now do the final comparison
				if !reflect.DeepEqual(got, tt.want) {
					gotJSON, _ := json.MarshalIndent(got, "", "  ")
					wantJSON, _ := json.MarshalIndent(tt.want, "", "  ")
					t.Errorf("ParseQuery() mismatch:\nGOT:\n%s\n\nWANT:\n%s", string(gotJSON), string(wantJSON))
				}
			}
		})
	}
}

func TestParserErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "incomplete match query",
			input:   "MATCH (pod:Pod)",
			wantErr: "incomplete expression",
		},
		{
			name:    "missing closing parenthesis",
			input:   "MATCH (pod:Pod RETURN pod",
			wantErr: "expected )",
		},
		{
			name:    "invalid relationship direction",
			input:   "MATCH (pod:Pod)<<(svc:Service) RETURN pod",
			wantErr: "unexpected relationship token",
		},
		{
			name:    "missing kind after colon",
			input:   "MATCH (pod:) RETURN pod",
			wantErr: "expected kind identifier",
		},
		{
			name:    "invalid operator in where clause",
			input:   "MATCH (pod:Pod) WHERE pod.metadata.name ?? 'nginx' RETURN pod",
			wantErr: "expected operator",
		},
		{
			name:    "invalid clause combination",
			input:   "CREATE (pod:Pod) DELETE pod",
			wantErr: "DELETE can only follow MATCH",
		},
		{
			name:    "missing return value",
			input:   "MATCH (pod:Pod) RETURN",
			wantErr: "expected identifier",
		},
		{
			name:    "invalid array index",
			input:   "MATCH (pod:Pod) WHERE pod.spec.containers[a].image = 'nginx' RETURN pod",
			wantErr: "expected number in array index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing input: %q", tt.input)
			parser := NewRecursiveParser(tt.input)
			_, err := parser.Parse()
			if err == nil {
				t.Errorf("ParseQuery() expected error containing %q, got nil", tt.wantErr)
				return
			}
			t.Logf("Got error: %v", err)
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseQuery() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
