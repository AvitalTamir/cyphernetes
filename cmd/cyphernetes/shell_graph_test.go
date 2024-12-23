package main

import (
	"reflect"
	"testing"

	"github.com/AvitalTamir/cyphernetes/pkg/core"
)

func TestSanitizeGraph(t *testing.T) {
	testCases := []struct {
		name     string
		input    core.Graph
		result   string
		expected core.Graph
	}{
		{
			name: "Filter out nodes and edges",
			input: core.Graph{
				Nodes: []core.Node{
					{Id: "Pod", Kind: "Pod", Name: "pod1"},
					{Id: "Service", Kind: "Service", Name: "svc1"},
				},
				Edges: []core.Edge{
					{From: "Pod", To: "Service", Type: "EXPOSE"},
				},
			},
			result: `{"Pod":[{"name":"pod1"}]}`,
			expected: core.Graph{
				Nodes: []core.Node{
					{Id: "Pod", Kind: "Pod", Name: "pod1"},
				},
				Edges: []core.Edge(nil),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := sanitizeGraph(tc.input, tc.result)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("\nExpected: %#v\n     Got: %#v", tc.expected, result)
			}
		})
	}
}

func TestMergeGraphs(t *testing.T) {
	graph1 := core.Graph{
		Nodes: []core.Node{{Id: "Pod/pod1", Kind: "Pod", Name: "pod1"}},
		Edges: []core.Edge{{From: "Pod/pod1", To: "Service/svc1", Type: "EXPOSE"}},
	}
	graph2 := core.Graph{
		Nodes: []core.Node{{Id: "Service/svc1", Kind: "Service", Name: "svc1"}},
		Edges: []core.Edge{{From: "Service/svc1", To: "Ingress/ing1", Type: "ROUTE"}},
	}
	expected := core.Graph{
		Nodes: []core.Node{
			{Id: "Pod/pod1", Kind: "Pod", Name: "pod1"},
			{Id: "Service/svc1", Kind: "Service", Name: "svc1"},
		},
		Edges: []core.Edge{
			{From: "Pod/pod1", To: "Service/svc1", Type: "EXPOSE"},
			{From: "Service/svc1", To: "Ingress/ing1", Type: "ROUTE"},
		},
	}

	result := mergeGraphs(graph1, graph2)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, but got %+v", expected, result)
	}
}

func TestDrawGraph(t *testing.T) {
	graph := core.Graph{
		Nodes: []core.Node{
			{Id: "Pod/pod1", Kind: "Pod", Name: "pod1"},
			{Id: "Service/svc1", Kind: "Service", Name: "svc1"},
		},
		Edges: []core.Edge{
			{From: "Pod/pod1", To: "Service/svc1", Type: "EXPOSES"},
		},
	}
	result := `{"Pod":[{"name":"pod1"}],"Service":[{"name":"svc1"}]}`

	_, err := drawGraph(graph, result)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Note: We're not checking the actual ASCII output here as it depends on an external service
}

func TestGetKindFromNodeId(t *testing.T) {
	testCases := []struct {
		nodeId   string
		expected string
	}{
		{"Pod/pod1", "Pod"},
		{"Service/svc1", "Service"},
	}

	for _, tc := range testCases {
		result := getKindFromNodeId(tc.nodeId)
		if result != tc.expected {
			t.Errorf("For nodeId %s, expected %s, but got %s", tc.nodeId, tc.expected, result)
		}
	}
}

func TestGetNameFromNodeId(t *testing.T) {
	testCases := []struct {
		nodeId   string
		expected string
	}{
		{"Pod/pod1", "pod1"},
		{"Service/svc1", "svc1"},
	}

	for _, tc := range testCases {
		result := getNameFromNodeId(tc.nodeId)
		if result != tc.expected {
			t.Errorf("For nodeId %s, expected %s, but got %s", tc.nodeId, tc.expected, result)
		}
	}
}

// Note: We're not testing dotToAscii function as it depends on an external service
