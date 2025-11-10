package core

import (
	"reflect"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// testProvider extends MockProvider to override FindGVR for our test cases
type testProvider struct {
	MockProvider
}

func (t *testProvider) FindGVR(kind string) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{Resource: kind}, nil
}

func TestFindPotentialKindsIntersection(t *testing.T) {
	mockProvider := &testProvider{}

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
			got, err := FindPotentialKindsIntersection(tt.relationships, mockProvider)
			if err != nil {
				t.Errorf("FindPotentialKindsIntersection() error = %v", err)
			}
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
	mockProvider := &testProvider{}

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
							Kind: "pods",
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
			want: []string{"services", "networkpolicies", "persistentvolumeclaims", "poddisruptionbudgets", "replicasets", "statefulsets", "daemonsets", "jobs", "cronjobs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindPotentialKindsIntersection(tt.relationships, mockProvider)
			if err != nil {
				t.Errorf("FindPotentialKindsIntersection() error = %v", err)
			}
			sort.Strings(got)
			sort.Strings(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindPotentialKindsIntersection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindPotentialKinds(t *testing.T) {
	mockProvider := &testProvider{}
	LogLevel = "debug"
	tests := []struct {
		name       string
		sourceKind string
		want       []string
	}{
		{
			name:       "pods to services (hardcoded)",
			sourceKind: "pods",
			want:       []string{"cronjobs", "daemonsets", "jobs", "networkpolicies", "persistentvolumeclaims", "poddisruptionbudgets", "replicasets", "services", "statefulsets"},
		},
		{
			name:       "services to pods (reverse)",
			sourceKind: "services",
			want:       []string{"daemonsets", "deployments", "endpoints", "ingresses", "mutatingwebhookconfigurations", "pods", "replicasets", "statefulsets", "validatingwebhookconfigurations"},
		},
		{
			name:       "case insensitive - PODS",
			sourceKind: "PODS",
			want:       []string{"cronjobs", "daemonsets", "jobs", "networkpolicies", "persistentvolumeclaims", "poddisruptionbudgets", "replicasets", "services", "statefulsets"},
		},
		{
			name:       "non-existent kind",
			sourceKind: "nonexistentkind",
			want:       []string{},
		},
		{
			name:       "kind with no relationships",
			sourceKind: "configmaps",
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindPotentialKinds(tt.sourceKind, mockProvider)
			if err != nil {
				t.Errorf("FindPotentialKinds() error = %v", err)
			}
			// Sort both slices for consistent comparison
			sort.Strings(got)
			sort.Strings(tt.want)

			// Special handling for empty slices
			if len(got) == 0 && len(tt.want) == 0 {
				return // Both are empty, test passes
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindPotentialKinds() = %v, want %v", got, tt.want)
			}
		})
	}
}
