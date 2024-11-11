package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/tools/clientcmd"
)

type QueryRequest struct {
	Query string `json:"query"`
}

type QueryResponse struct {
	Result string      `json:"result"`
	Graph  interface{} `json:"graph"`
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
	}
}

func handleQuery(c *gin.Context) {
	var req QueryRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	executor := parser.GetQueryExecutorInstance()

	namespace := "default"

	ast, err := parser.ParseQuery(req.Query)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Execute the query using the parser
	result, err := executor.Execute(ast, namespace, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// marshal the result.Data and result.Graph to json
	resultData, err := json.Marshal(result.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	sanitizedGraph, err := sanitizeGraph(result.Graph, string(resultData))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resultGraph, err := json.Marshal(sanitizedGraph)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := QueryResponse{
		Result: string(resultData),
		Graph:  string(resultGraph),
	}

	c.JSON(http.StatusOK, response)
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

func handleConvertResourceName(c *gin.Context) {
	resourceName := c.Query("name")
	if resourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Resource name is required"})
		return
	}

	executor := parser.GetQueryExecutorInstance()
	// Use the FindGVR function to get the singular form
	gvr, err := parser.FindGVR(executor.Clientset, resourceName)
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

	// Get namespace from context
	namespace := "default"
	if context, exists := config.Contexts[currentContext]; exists && context.Namespace != "" {
		namespace = context.Namespace
	}

	c.JSON(http.StatusOK, ContextInfo{
		Context:   currentContext,
		Namespace: namespace,
	})
}
