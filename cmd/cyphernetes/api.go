package main

import (
	"encoding/json"
	"net/http"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/gin-gonic/gin"
)

type QueryRequest struct {
	Query string `json:"query"`
}

type QueryResponse struct {
	Result string      `json:"result"`
	Graph  interface{} `json:"graph"`
}

func setupAPIRoutes(router *gin.Engine) {
	router.POST("/api/query", handleQuery)
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
	result, err := executor.Execute(ast, namespace)
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
