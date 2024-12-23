package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"slices"

	"github.com/avitaltamir/cyphernetes/pkg/core"
)

func sanitizeGraph(g core.Graph, result string) (core.Graph, error) {
	// create a unique map of nodes
	nodeMap := make(map[string]core.Node)
	for _, node := range g.Nodes {
		nodeId := fmt.Sprintf("%s/%s", node.Kind, node.Name)
		nodeMap[nodeId] = node
	}
	g.Nodes = make([]core.Node, 0, len(nodeMap))
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
	var filteredNodes []core.Node
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
	var filteredEdges []core.Edge
	for _, edge := range g.Edges {
		if slices.Contains(filteredNodeIds, edge.From) && slices.Contains(filteredNodeIds, edge.To) {
			filteredEdges = append(filteredEdges, edge)
		}
	}
	g.Edges = filteredEdges
	return g, nil
}

func mergeGraphs(graph core.Graph, newGraph core.Graph) core.Graph {
	// merge the nodes
	graph.Nodes = append(graph.Nodes, newGraph.Nodes...)
	// merge the edges
	graph.Edges = append(graph.Edges, newGraph.Edges...)
	return graph
}

func drawGraph(graph core.Graph, result string) (string, error) {
	graph, err := sanitizeGraph(graph, result)
	if err != nil {
		return "", fmt.Errorf("error sanitizing graph: %w", err)
	}

	var graphString strings.Builder
	graphString.WriteString("graph {\n")
	if graphLayoutLR {
		graphString.WriteString("\trankdir = LR;\n\n")
	}

	for _, edge := range graph.Edges {
		graphString.WriteString(fmt.Sprintf("\"*%s* %s\" -> \"*%s* %s\" [label=\":%s\"];\n",
			getKindFromNodeId(edge.From),
			getNameFromNodeId(edge.From),
			getKindFromNodeId(edge.To),
			getNameFromNodeId(edge.To),
			edge.Type))
	}

	// iterate over graph.Nodes and find nodes which are not in the graphString
	for _, node := range graph.Nodes {
		if !strings.Contains(graphString.String(), fmt.Sprintf("\"%s %s\"", node.Kind, node.Name)) {
			graphString.WriteString(fmt.Sprintf("\"*%s* %s\";\n", node.Kind, node.Name))
		}
	}

	graphString.WriteString("}")

	ascii, err := dotToAscii(graphString.String(), true)
	if err != nil {
		return "", fmt.Errorf("error converting graph to ASCII: %w", err)
	}

	return "\n" + ascii, nil
}

func getKindFromNodeId(nodeId string) string {
	parts := strings.Split(nodeId, "/")
	return parts[0]
}

func getNameFromNodeId(nodeId string) string {
	parts := strings.Split(nodeId, "/")
	return parts[1]
}

func dotToAscii(dot string, fancy bool) (string, error) {
	fmt.Println("Converting Graph to ASCII...")
	defer func() {
		fmt.Print("\033[1A\033[K") // Move cursor up one line and clear it
	}()

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
