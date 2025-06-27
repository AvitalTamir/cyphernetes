package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

type QueryRequest struct {
	Query string `json:"query"`
}

type QueryResponse struct {
	Result string `json:"result"`
	Graph  string `json:"graph"`
}

type ContextInfo struct {
	Context   string `json:"context"`
	Namespace string `json:"namespace,omitempty"`
}

func setupAPIRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		api.POST("/query", handleQuery)
		api.GET("/autocomplete", handleAutocomplete)
		api.GET("/convert-resource-name", handleConvertResourceName)
		api.GET("/context", handleGetContext)
		api.GET("/health", handleHealth)

		api.GET("/config", handleGetConfig)
		api.POST("/config", handleSetConfig)

		api.GET("/namespaces", handleGetNamespaces)
		api.POST("/namespace", handleSetNamespace)
	}
}

func handleQuery(c *gin.Context) {
	var req QueryRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// Create the API server provider
	p, err := apiserver.NewAPIServerProviderWithOptions(&apiserver.APIServerProviderConfig{
		DryRun: DryRun,
	})
	if err != nil {
		fmt.Printf("Provider error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error creating provider: %v", err)})
		return
	}

	// Initialize the executor instance with the provider
	executor := core.GetQueryExecutorInstance(p)
	if executor == nil {
		fmt.Printf("Failed to initialize executor\n")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize query executor"})
		return
	}

	// Parse the query
	ast, err := core.ParseQuery(req.Query)
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Error parsing query: %v", err)})
		return
	}

	// Execute the query
	result, err := executor.Execute(ast, core.Namespace)
	if err != nil {
		fmt.Printf("Execution error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error executing query: %v", err)})
		return
	}

	// Deduplicate nodes before serialization
	seenNodes := make(map[string]bool)
	var dedupedNodes []core.Node
	for _, node := range result.Graph.Nodes {
		nodeKey := fmt.Sprintf("%s/%s", node.Kind, node.Name)
		if !seenNodes[nodeKey] {
			seenNodes[nodeKey] = true
			dedupedNodes = append(dedupedNodes, node)
		}
	}
	result.Graph.Nodes = dedupedNodes

	// Convert to JSON
	resultJson, err := json.Marshal(result.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error marshaling result: %v", err)})
		return
	}

	// Sanitize the graph before marshalling
	cleanGraph, err := sanitizeGraph(result.Graph, string(resultJson))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error sanitizing graph: %v", err)})
		return
	}
	result.Graph = cleanGraph

	graphJson, err := json.Marshal(result.Graph)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error marshaling graph: %v", err)})
		return
	}

	c.JSON(http.StatusOK, QueryResponse{
		Result: string(resultJson),
		Graph:  string(graphJson),
	})
}

func handleAutocomplete(c *gin.Context) {
	query := c.Query("query")
	pos := c.Query("position")

	position, err := strconv.Atoi(pos)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid position"})
		return
	}

	completer := &CyphernetesCompleter{}
	suggestions, _ := completer.Do([]rune(query), position)

	// Convert [][]rune to []string
	stringSuggestions := make([]string, len(suggestions))
	for i, suggestion := range suggestions {
		stringSuggestions[i] = string(suggestion)
	}

	c.JSON(http.StatusOK, gin.H{"suggestions": stringSuggestions})
}

func handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func handleConvertResourceName(c *gin.Context) {
	resourceName := c.Query("name")
	if resourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Resource name is required"})
		return
	}

	p, err := apiserver.NewAPIServerProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create API server provider"})
		return
	}
	gvr, err := p.FindGVR(resourceName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"singular": gvr.Resource})
}

func handleGetContext(c *gin.Context) {
	// Get the kubeconfig loader
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{},
	).RawConfig()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load kubeconfig"})
		return
	}

	// Get current context
	currentContext := config.CurrentContext
	if currentContext == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No current context set"})
		return
	}

	c.JSON(http.StatusOK, ContextInfo{
		Context:   currentContext,
		Namespace: core.Namespace,
	})
}

func handleSetConfig(c *gin.Context) {
	var config struct {
		DryRun *bool `json:"dryRun"`
	}

	if err := c.BindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// Update dry run mode if provided
	if config.DryRun != nil {
		DryRun = *config.DryRun
		executor.Provider().ToggleDryRun()
	}

	// Return the updated configuration
	c.JSON(http.StatusOK, gin.H{
		"dryRun": DryRun,
	})
}

func handleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"dryRun": DryRun,
	})
}

// handleGetNamespaces returns a list of all available namespaces
func handleGetNamespaces(c *gin.Context) {
	// Get the provider from the executor
	if executor == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Executor not initialized"})
		return
	}

	// Get the API server provider directly
	apiProvider, ok := executor.Provider().(*apiserver.APIServerProvider)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Provider is not an APIServerProvider"})
		return
	}

	// Get the clientset directly from the provider
	clientset, err := apiProvider.GetClientset()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get clientset: %v", err)})
		return
	}

	// Get the list of namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list namespaces: %v", err)})
		return
	}

	// Extract namespace names
	namespaceList := make([]string, 0, len(namespaces.Items))
	for _, ns := range namespaces.Items {
		namespaceList = append(namespaceList, ns.Name)
	}

	// Sort the namespaces alphabetically
	sort.Strings(namespaceList)

	// Return the list of namespaces
	c.JSON(http.StatusOK, gin.H{
		"namespaces": namespaceList,
		"current":    core.Namespace,
	})
}

// handleSetNamespace sets the current namespace
func handleSetNamespace(c *gin.Context) {
	var req struct {
		Namespace string `json:"namespace"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// Set the namespace
	core.Namespace = req.Namespace

	// Return the updated namespace
	c.JSON(http.StatusOK, gin.H{
		"namespace": core.Namespace,
	})
}
