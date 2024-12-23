package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/AvitalTamir/cyphernetes/pkg/core"
	"github.com/AvitalTamir/cyphernetes/pkg/provider/apiserver"
	"github.com/gin-gonic/gin"
)

type QueryRequest struct {
	Query string `json:"query"`
}

type QueryResponse struct {
	Result string `json:"result"`
	Graph  string `json:"graph"`
}

func setupAPIRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		api.POST("/query", handleQuery)
		api.GET("/health", handleHealth)
	}
}

func handleQuery(c *gin.Context) {
	var req QueryRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// Create the API server provider
	p, err := apiserver.NewAPIServerProvider()
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
	result, err := executor.Execute(ast, "")
	if err != nil {
		fmt.Printf("Execution error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error executing query: %v", err)})
		return
	}

	// Marshal the result data to JSON string
	resultData, err := json.Marshal(result.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error marshalling results: %v", err)})
		return
	}

	// Sanitize the graph data
	sanitizedGraph, err := sanitizeGraph(result.Graph, string(resultData))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error sanitizing graph: %v", err)})
		return
	}

	// Marshal the sanitized graph to JSON string
	graphData, err := json.Marshal(sanitizedGraph)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error marshalling graph: %v", err)})
		return
	}

	// Return the response with both result and graph as strings
	response := QueryResponse{
		Result: string(resultData),
		Graph:  string(graphData),
	}

	c.JSON(http.StatusOK, response)
}

func handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
