package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"slices"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
)

func sanitizeGraph(g parser.Graph, result string) (parser.Graph, error) {
	// create a unique map of nodes
	nodeMap := make(map[string]parser.Node)
	for _, node := range g.Nodes {
		nodeId := fmt.Sprintf("%s/%s", node.Kind, node.Name)
		nodeMap[nodeId] = node
	}
	g.Nodes = make([]parser.Node, 0, len(nodeMap))
	for _, node := range nodeMap {
		g.Nodes = append(g.Nodes, node)
	}

	// unmarshal the result into a map[string]interface{}
	var resultMap map[string]interface{}
	err := json.Unmarshal([]byte(result), &resultMap)
	if err != nil {
		return g, fmt.Errorf("error unmarshalling result: %w", err)
	}

	// now let's filter out nodes that have no data (in g.Data)
	var filteredNodes []parser.Node
	for _, node := range g.Nodes {
		if resultMap[node.Id] != nil {
			for _, resultMapNode := range resultMap[node.Id].([]interface{}) {
				if resultMapNode.(map[string]interface{})["name"] == node.Name {
					filteredNodes = append(filteredNodes, node)
				}
			}
		}
	}
	g.Nodes = filteredNodes

	filteredNodeIds := []string{}
	for _, node := range filteredNodes {
		nodeId := fmt.Sprintf("%s/%s", node.Kind, node.Name)
		filteredNodeIds = append(filteredNodeIds, nodeId)
	}
	// now let's filter out edges that point to nodes that don't exist
	var filteredEdges []parser.Edge
	for _, edge := range g.Edges {
		if slices.Contains(filteredNodeIds, edge.From) && slices.Contains(filteredNodeIds, edge.To) {
			filteredEdges = append(filteredEdges, edge)
		}
	}
	g.Edges = filteredEdges
	return g, nil
}

func DrawGraphviz(graph parser.Graph, result string) string {
	g := graphviz.New()
	gv, err := g.Graph()
	if err != nil {
		return fmt.Sprintf("Error creating graph: %v", err)
	}
	defer func() {
		if err := gv.Close(); err != nil {
			fmt.Printf("Error closing graph: %v\n", err)
		}
		g.Close()
	}()

	// Create a map to store nodes
	nodes := make(map[string]*cgraph.Node)

	// Create nodes
	for _, node := range graph.Nodes {
		nodeId := fmt.Sprintf("%s/%s", node.Kind, node.Name)
		n, err := gv.CreateNode(nodeId)
		if err != nil {
			return fmt.Sprintf("Error creating node: %v", err)
		}
		n.SetLabel(fmt.Sprintf("*%s* %s", node.Kind, node.Name))
		nodes[nodeId] = n
	}

	// Create edges
	for _, edge := range graph.Edges {
		fromNode, ok := nodes[edge.From]
		if !ok {
			return fmt.Sprintf("Error: node %s not found", edge.From)
		}
		toNode, ok := nodes[edge.To]
		if !ok {
			return fmt.Sprintf("Error: node %s not found", edge.To)
		}
		e, err := gv.CreateEdge(edge.Type, fromNode, toNode)
		if err != nil {
			return fmt.Sprintf("Error creating edge: %v", err)
		}
		e.SetLabel(":" + edge.Type)
	}

	// Iterate over nodes of kind "Namespace" and iterate over all nodes to check if their ".metadata.namespace" matches this namespace
	// if it does, add an edge from the namespace to the node
	for _, node := range graph.Nodes {
		if node.Kind == "Namespace" {
			for _, node2 := range graph.Nodes {
				if node.Namespace != "" && node.Namespace == node2.Namespace {
					nodeId := fmt.Sprintf("%s/%s", node.Kind, node.Name)
					node2Id := fmt.Sprintf("%s/%s", node2.Kind, node2.Name)
					gv.CreateEdge(string(parser.NamespaceHasResource), nodes[nodeId], nodes[node2Id])
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := g.Render(gv, graphviz.Format("dot"), &buf); err != nil {
		fmt.Println("Error rendering graph:", err)
	}

	ascii, err := dotToAscii(buf.String(), true)
	if err != nil {
		return fmt.Sprintf("Error converting graph to ASCII: %v", err)
	}

	return "\n" + ascii
}

func dotToAscii(dot string, fancy bool) (string, error) {
	url := "https://ascii.cyphernet.es/dot-to-ascii.php"
	boxart := 0
	if fancy {
		boxart = 1
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	q := req.URL.Query()
	q.Add("boxart", strconv.Itoa(boxart))
	q.Add("src", dot)
	req.URL.RawQuery = q.Encode()

	response, err := http.Get(req.URL.String())
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
