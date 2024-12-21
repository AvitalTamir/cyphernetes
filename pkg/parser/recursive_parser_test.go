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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseQuery(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(tt.want, "", "  ")
				t.Errorf("ParseQuery() mismatch:\nGOT:\n%s\n\nWANT:\n%s", string(gotJSON), string(wantJSON))
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
