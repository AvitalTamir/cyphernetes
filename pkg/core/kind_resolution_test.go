package core

import (
	"reflect"
	"sort"
	"testing"
)

func TestFindPotentialKindsIntersection(t *testing.T) {
	tests := []struct {
		name          string
		relationships []*Relationship
		want          []string
	}{
		{
			name: "single relationship with unknown left node",
			relationships: []*Relationship{
				{
					LeftNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "x",
							Kind: "",
						},
					},
					RightNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "s",
							Kind: "services",
						},
					},
				},
			},
			want: []string{"daemonsets", "deployments", "endpoints", "ingresses", "mutatingwebhookconfigurations", "pods", "replicasets", "statefulsets", "validatingwebhookconfigurations"},
		},
		{
			name: "two relationships with common kinds",
			relationships: []*Relationship{
				{
					LeftNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "x",
							Kind: "",
						},
					},
					RightNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "s",
							Kind: "services",
						},
					},
				},
				{
					LeftNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "x",
							Kind: "",
						},
					},
					RightNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "p",
							Kind: "pods",
						},
					},
				},
			},
			want: []string{"daemonsets", "replicasets", "statefulsets"},
		},
		{
			name:          "empty relationships",
			relationships: []*Relationship{},
			want:          []string{},
		},
		{
			name: "no unknown kinds",
			relationships: []*Relationship{
				{
					LeftNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "s",
							Kind: "services",
						},
					},
					RightNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "p",
							Kind: "pods",
						},
					},
				},
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindPotentialKindsIntersection(tt.relationships)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindPotentialKindsIntersection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateAnonymousNode(t *testing.T) {
	tests := []struct {
		name          string
		node          *NodePattern
		relationships []*Relationship
		wantErr       bool
	}{
		{
			name: "non-anonymous node",
			node: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name: "x",
					Kind: "pods",
				},
				IsAnonymous: false,
			},
			relationships: []*Relationship{},
			wantErr:       false,
		},
		{
			name: "anonymous node in relationship",
			node: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name: "_anon1",
					Kind: "",
				},
				IsAnonymous: true,
			},
			relationships: []*Relationship{
				{
					LeftNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "_anon1",
							Kind: "",
						},
					},
					RightNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "s",
							Kind: "services",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "standalone anonymous node",
			node: &NodePattern{
				ResourceProperties: &ResourceProperties{
					Name: "_anon1",
					Kind: "",
				},
				IsAnonymous: true,
			},
			relationships: []*Relationship{},
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAnonymousNode(tt.node, tt.relationships)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAnonymousNode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFindPotentialKindsWithPartialKnownRelationship(t *testing.T) {
	tests := []struct {
		name          string
		relationships []*Relationship
		want          []string
	}{
		{
			name: "pod to unknown kind",
			relationships: []*Relationship{
				{
					LeftNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Kind: "pod",
							Name: "p",
						},
					},
					RightNode: &NodePattern{
						ResourceProperties: &ResourceProperties{
							Name: "x",
						},
					},
				},
			},
			want: []string{"services", "networkpolicies", "poddisruptionbudgets", "replicasets", "statefulsets", "daemonsets", "jobs", "cronjobs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindPotentialKindsIntersection(tt.relationships)
			sort.Strings(got)
			sort.Strings(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindPotentialKindsIntersection() = %v, want %v", got, tt.want)
			}
		})
	}
}
