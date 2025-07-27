package notebook

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// DNS service API structures
type DNSServiceRequest struct {
	ExpiresIn int `json:"expires_in"` // seconds
}

type DNSServiceResponse struct {
	Subdomain string `json:"subdomain"`
	ExpiresAt string `json:"expires_at"`
	ExpiresIn int    `json:"expires_in"`
}

// Helper function to check if request is from a shared session (read-only)
func (s *Server) isSharedSession(c *gin.Context) bool {
	return c.GetHeader("X-Shared-Session") == "true"
}

// Helper function to get subdomain from DNS service
func (s *Server) getSubdomainFromDNS(expiresIn int) (string, error) {
	reqBody := DNSServiceRequest{
		ExpiresIn: expiresIn,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	// Call the DNS service
	resp, err := client.Post("https://go.cyphernet.es/api/subdomain", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to call DNS service: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("DNS service returned status %d: %s", resp.StatusCode, string(body))
	}
	
	var dnsResp DNSServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&dnsResp); err != nil {
		return "", fmt.Errorf("failed to decode DNS response: %w", err)
	}
	
	if dnsResp.Subdomain == "" {
		return "", fmt.Errorf("DNS service returned empty subdomain")
	}
	
	return dnsResp.Subdomain, nil
}

// Middleware for share token validation
func (s *Server) validateShareToken() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			// No token provided, continue normally
			c.Next()
			return
		}

		// Clean up expired tokens first
		s.store.DeleteExpiredShareTokens()

		// Validate the token
		shareToken, err := s.store.GetShareToken(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Token validation failed"})
			c.Abort()
			return
		}

		if shareToken == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Check if token is expired
		if time.Now().After(shareToken.ExpiresAt) {
			s.store.DeleteShareToken(token)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token has expired"})
			c.Abort()
			return
		}

		// Store share token info in context for later use
		c.Set("share_token", shareToken)
		c.Next()
	})
}

// Notebook management handlers

func (s *Server) listNotebooks(c *gin.Context) {
	// TODO: Get user ID from session/auth
	userID := "default-user"

	notebooks, err := s.store.ListNotebooks(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Ensure we return an empty array instead of null
	if notebooks == nil {
		notebooks = []*Notebook{}
	}

	c.JSON(http.StatusOK, notebooks)
}

func (s *Server) createNotebook(c *gin.Context) {
	// Prevent creation from shared sessions
	if s.isSharedSession(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot create notebooks in shared mode"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Get user ID from session/auth
	userID := "default-user"

	notebook, err := s.store.CreateNotebook(req.Name, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, notebook)
}

func (s *Server) getNotebook(c *gin.Context) {
	id := c.Param("id")

	notebook, err := s.store.GetNotebook(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notebook not found"})
		return
	}

	// Get cells for the notebook
	cells, err := s.store.GetCells(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Ensure cells is not nil
	if cells == nil {
		cells = []*Cell{}
	}

	response := struct {
		*Notebook
		Cells []*Cell `json:"cells"`
	}{
		Notebook: notebook,
		Cells:    cells,
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) updateNotebook(c *gin.Context) {
	// Prevent updates from shared sessions
	if s.isSharedSession(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot update notebooks in shared mode"})
		return
	}

	id := c.Param("id")

	var req struct {
		Name *string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update the notebook
	err := s.store.UpdateNotebook(id, req.Name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Notebook not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return the updated notebook
	notebook, err := s.store.GetNotebook(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, notebook)
}

func (s *Server) deleteNotebook(c *gin.Context) {
	// Prevent deletion from shared sessions
	if s.isSharedSession(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete notebooks in shared mode"})
		return
	}

	id := c.Param("id")

	// Delete the notebook from storage
	err := s.store.DeleteNotebook(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Notebook not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete notebook"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notebook deleted successfully"})
}

func (s *Server) forkNotebook(c *gin.Context) {
	// TODO: Implement notebook forking
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// Cell management handlers

func (s *Server) addCell(c *gin.Context) {
	// Prevent cell creation from shared sessions
	if s.isSharedSession(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot add cells in shared mode"})
		return
	}

	notebookID := c.Param("id")

	var cell Cell
	if err := c.ShouldBindJSON(&cell); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid JSON: %v", err)})
		return
	}

	// Get current cells to shift their positions
	cells, err := s.store.GetCells(notebookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get cells: %v", err)})
		return
	}

	// Shift all existing cells down by incrementing their positions
	for _, existingCell := range cells {
		err = s.store.UpdateCellPosition(existingCell.ID, existingCell.Position+1)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update cell positions: %v", err)})
			return
		}
	}

	// Set defaults - new cell gets position 0 (top)
	cell.NotebookID = notebookID
	cell.Position = 0
	cell.RowIndex = 0 // New cell at the top
	cell.ColIndex = 0
	cell.LayoutMode = LayoutSingle
	if cell.VisualizationType == "" {
		cell.VisualizationType = VisTypeJSON
	}
	if cell.Type == "" {
		cell.Type = CellTypeQuery
	}
	if cell.Query == "" {
		cell.Query = ""
	}

	createdCell, err := s.store.CreateCell(notebookID, &cell)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create cell: %v", err)})
		return
	}

	// Broadcast to all connected users
	s.sessions.BroadcastToNotebook(notebookID, map[string]interface{}{
		"type": "cell-added",
		"cell": createdCell,
	}, "")

	c.JSON(http.StatusCreated, createdCell)
}

func (s *Server) updateCell(c *gin.Context) {
	// Prevent cell updates from shared sessions
	if s.isSharedSession(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot update cells in shared mode"})
		return
	}

	notebookID := c.Param("id")
	cellID := c.Param("cellId")

	var updates struct {
		Query             *string            `json:"query"`
		VisualizationType *VisualizationType `json:"visualization_type"`
		RefreshInterval   *int               `json:"refresh_interval"`
		Config            *CellConfig        `json:"config"`
		Name              *string            `json:"name"`
	}

	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update the cell in the database
	err := s.store.UpdateCell(cellID, updates.Query, updates.VisualizationType, updates.RefreshInterval, updates.Config, updates.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cell"})
		return
	}

	// Touch the notebook to update its timestamp
	s.store.touchNotebook(notebookID)

	// Broadcast the cell update to all connected users
	s.sessions.BroadcastToNotebook(notebookID, map[string]interface{}{
		"type":    "cell-updated",
		"cellId":  cellID,
		"updates": updates,
	}, "")

	c.JSON(http.StatusOK, gin.H{"message": "Cell updated successfully"})
}

func (s *Server) deleteCell(c *gin.Context) {
	// Prevent cell deletion from shared sessions
	if s.isSharedSession(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete cells in shared mode"})
		return
	}

	notebookID := c.Param("id")
	cellID := c.Param("cellId")

	// Delete the cell from storage
	err := s.store.DeleteCell(cellID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete cell"})
		return
	}

	// Touch the notebook to update its timestamp
	s.store.touchNotebook(notebookID)

	// Broadcast the cell deletion to all connected users
	s.sessions.BroadcastToNotebook(notebookID, map[string]interface{}{
		"type":   "cell-deleted",
		"cellId": cellID,
	}, "")

	c.JSON(http.StatusOK, gin.H{"message": "Cell deleted successfully"})
}

func (s *Server) executeCell(c *gin.Context) {
	notebookID := c.Param("id")
	cellID := c.Param("cellId")

	// Parse request body for context and namespace
	var req struct {
		Context   string `json:"context"`
		Namespace string `json:"namespace"`
	}

	// Bind JSON, but don't fail if empty (use defaults)
	c.ShouldBindJSON(&req)

	// Get the cell from storage
	cells, err := s.store.GetCells(notebookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cells"})
		return
	}

	// Find the specific cell
	var targetCell *Cell
	for _, cell := range cells {
		if cell.ID == cellID {
			targetCell = cell
			break
		}
	}

	if targetCell == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cell not found"})
		return
	}

	// Only execute query cells
	if targetCell.Type != CellTypeQuery {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only query cells can be executed"})
		return
	}

	// Use provided namespace or fall back to stored config or default
	namespace := req.Namespace
	if namespace == "" && targetCell.Config.Namespace != "" {
		namespace = targetCell.Config.Namespace
	}
	if namespace == "" {
		namespace = "default"
	}

	// Execute the query with the specified namespace
	result, err := s.executor.ExecuteQuery(targetCell.Query, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Query execution failed: %v", err)})
		return
	}

	// Store the results in the cell
	err = s.store.UpdateCellResults(cellID, result, result.Error)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store results"})
		return
	}

	// Broadcast the results to all connected users
	s.sessions.BroadcastToNotebook(notebookID, map[string]interface{}{
		"type":   "execution-result",
		"cellId": cellID,
		"result": result,
	}, "")

	c.JSON(http.StatusOK, gin.H{
		"cellId": cellID,
		"result": result,
	})
}

func (s *Server) getLogs(c *gin.Context) {
	// Parse query parameters
	podName := c.Query("pod")
	podsParam := c.Query("pods")
	container := c.Query("container")
	namespace := c.Query("namespace")
	follow := c.Query("follow") == "true"
	tailLinesStr := c.Query("tail_lines")
	sinceTime := c.Query("since_time")

	// Validate required parameters - either pod or pods must be provided
	if podName == "" && podsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pod or pods parameter is required"})
		return
	}

	if namespace == "" {
		namespace = "default"
	}

	// Parse tail_lines parameter
	var tailLines *int64
	if tailLinesStr != "" {
		if lines, err := strconv.ParseInt(tailLinesStr, 10, 64); err == nil {
			tailLines = &lines
		}
	}

	// Create a new API server provider to get the clientset
	provider, err := apiserver.NewAPIServerProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create provider: %v", err)})
		return
	}

	// Cast to APIServerProvider to access GetClientset
	apiProvider, ok := provider.(*apiserver.APIServerProvider)
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

	// Prepare log options
	logOptions := &corev1.PodLogOptions{
		Follow:     follow,
		TailLines:  tailLines,
		Timestamps: true,
	}

	// Set container if specified
	if container != "" {
		logOptions.Container = container
	}

	// Parse since_time if provided
	if sinceTime != "" {
		if parsedTime, err := time.Parse(time.RFC3339, sinceTime); err == nil {
			sinceTimeMetav1 := metav1.NewTime(parsedTime)
			logOptions.SinceTime = &sinceTimeMetav1
		}
	}

	// Determine which pods to stream from
	var pods []string
	if podsParam != "" {
		pods = strings.Split(podsParam, ",")
	} else {
		pods = []string{podName}
	}

	// If follow is true, stream logs via Server-Sent Events
	if follow {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")

		// Flush headers
		c.Writer.Flush()

		// Create context for all streams
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Stream from multiple pods
		streamMultiplePods(c, clientset, namespace, pods, container, logOptions, ctx)
		return
	}

	// For non-streaming requests, get logs from the first pod only
	firstPod := pods[0]
	podLogsRequest := clientset.CoreV1().Pods(namespace).GetLogs(firstPod, logOptions)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := podLogsRequest.Stream(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get logs: %v", err)})
		return
	}
	defer stream.Close()

	// For non-streaming requests, read all logs and return as JSON
	logs, err := io.ReadAll(stream)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read logs: %v", err)})
		return
	}

	// Split logs into lines
	logLines := strings.Split(string(logs), "\n")

	// Remove empty last line if present
	if len(logLines) > 0 && logLines[len(logLines)-1] == "" {
		logLines = logLines[:len(logLines)-1]
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":      logLines,
		"pod":       firstPod,
		"container": container,
		"namespace": namespace,
	})
}

func streamMultiplePods(c *gin.Context, clientset kubernetes.Interface, namespace string, pods []string, container string, logOptions *corev1.PodLogOptions, ctx context.Context) {
	// Channel to merge all log streams
	logChan := make(chan string, 100)
	doneChan := make(chan bool)

	// Start streaming from each pod
	for _, pod := range pods {
		go func(podName string) {
			defer func() {
				doneChan <- true
			}()

			// Get logs for this pod
			podLogOptions := &corev1.PodLogOptions{
				Follow:     logOptions.Follow,
				TailLines:  logOptions.TailLines,
				Timestamps: logOptions.Timestamps,
			}
			if container != "" {
				podLogOptions.Container = container
			}

			podLogsRequest := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOptions)
			stream, err := podLogsRequest.Stream(ctx)
			if err != nil {
				// Send error for this pod
				logChan <- fmt.Sprintf("Error streaming from pod %s: %v", podName, err)
				return
			}
			defer stream.Close()

			// Stream logs with pod name prefix
			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				line := scanner.Text()
				// Create JSON message with pod info
				message := fmt.Sprintf(`{"pod": "%s", "message": "%s"}`, podName, strings.ReplaceAll(line, "\"", "\\\""))
				logChan <- message
			}

			if err := scanner.Err(); err != nil {
				logChan <- fmt.Sprintf(`{"pod": "%s", "error": "%s"}`, podName, strings.ReplaceAll(err.Error(), "\"", "\\\""))
			}
		}(pod)
	}

	// Use a WaitGroup to ensure the streaming goroutine completes
	var wg sync.WaitGroup
	wg.Add(1)

	// Stream logs as they come in
	go func() {
		defer wg.Done()
		for logMessage := range logChan {
			select {
			case <-ctx.Done():
				return
			default:
				fmt.Fprintf(c.Writer, "data: %s\n\n", logMessage)
				c.Writer.Flush()
			}
		}
	}()

	// Wait for all pods to finish or context to be cancelled
	completedPods := 0
	for completedPods < len(pods) {
		select {
		case <-doneChan:
			completedPods++
		case <-ctx.Done():
			close(logChan)
			wg.Wait() // Wait for streaming goroutine to finish
			fmt.Fprintf(c.Writer, "event: end\ndata: Log stream ended\n\n")
			c.Writer.Flush()
			return
		}
	}

	close(logChan)
	wg.Wait() // Wait for streaming goroutine to finish
	fmt.Fprintf(c.Writer, "event: end\ndata: Log stream ended\n\n")
	c.Writer.Flush()
}

func (s *Server) reorderCells(c *gin.Context) {
	// Prevent cell reordering from shared sessions
	if s.isSharedSession(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot reorder cells in shared mode"})
		return
	}

	notebookID := c.Param("id")

	var request struct {
		CellOrders []struct {
			ID       string `json:"id"`
			Position int    `json:"position"`
		} `json:"cell_orders"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := s.store.ReorderCells(notebookID, request.CellOrders)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reorder cells"})
		return
	}

	// Broadcast to all WebSocket connections
	s.sessions.BroadcastToNotebook(notebookID, map[string]interface{}{
		"type":        "cells-reordered",
		"cell_orders": request.CellOrders,
	}, "")

	c.JSON(http.StatusOK, gin.H{"message": "Cells reordered successfully"})
}

func (s *Server) executeAllCells(c *gin.Context) {
	// TODO: Implement execute all cells
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// Collaboration handlers

func (s *Server) generateShareToken(c *gin.Context) {
	// Prevent token generation from shared sessions
	if s.isSharedSession(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot generate share tokens in shared mode"})
		return
	}

	type GenerateTokenRequest struct {
		NotebookID string `json:"notebook_id"`
	}

	var req GenerateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate a secure random token (32 bytes = 64 hex chars)
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	// Get subdomain from DNS service (10 minutes = 600 seconds)
	subdomain, err := s.getSubdomainFromDNS(600)
	if err != nil {
		// Fallback to mock subdomain if DNS service fails
		fmt.Printf("Warning: DNS service failed, using mock subdomain: %v\n", err)
		subdomainBytes := make([]byte, 6)
		rand.Read(subdomainBytes)
		subdomain = hex.EncodeToString(subdomainBytes)[:12]
	}

	// Create share token with 10 minute expiry
	expiryTime := time.Now().Add(10 * time.Minute)
	shareToken := &ShareToken{
		Token:      token,
		NotebookID: req.NotebookID,
		Subdomain:  subdomain,
		CreatedBy:  "user", // TODO: Get from session/auth
		ExpiresAt:  expiryTime,
	}

	// Store in database
	if err := s.store.CreateShareToken(shareToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create share token"})
		return
	}

	// Start the tunnel connection
	if err := s.startTunnel(subdomain, token, expiryTime); err != nil {
		// If tunnel fails, still return the URL but log the error
		fmt.Printf("Warning: Failed to start tunnel: %v\n", err)
	}

	// Return the shareable URL
	shareURL := fmt.Sprintf("https://%s.go.cyphernet.es/?token=%s", subdomain, token)
	
	c.JSON(http.StatusOK, gin.H{
		"share_url":  shareURL,
		"expires_at": shareToken.ExpiresAt,
		"expires_in": int(time.Until(shareToken.ExpiresAt).Seconds()),
	})
}

// Pin-based connection is deprecated in favor of token-based sharing

func (s *Server) listSessions(c *gin.Context) {
	// TODO: Get notebook ID from request
	notebookID := c.Query("notebook_id")
	if notebookID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notebook_id required"})
		return
	}

	sessions := s.sessions.GetNotebookSessions(notebookID)

	// Convert to response format
	var response []Session
	for _, session := range sessions {
		response = append(response, Session{
			ID:           session.UserID, // Using UserID as session ID for now
			NotebookID:   session.NotebookID,
			UserID:       session.UserID,
			Username:     session.Username,
			ConnectedAt:  session.ConnectedAt,
			LastActivity: session.LastActivity,
			IsOwner:      session.IsOwner,
		})
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) disconnectSession(c *gin.Context) {
	// TODO: Implement session disconnection
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// WebSocket handler

func (s *Server) handleWebSocket(c *gin.Context) {
	notebookID := c.Param("id")

	// TODO: Authenticate user
	userID := "default-user"
	username := "Default User"

	// Upgrade connection to WebSocket
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upgrade to WebSocket"})
		return
	}

	// Add session
	_ = s.sessions.AddSession(notebookID, userID, username, conn, true)
	defer func() {
		s.sessions.RemoveSession(notebookID, userID)
		conn.Close()
	}()

	// Notify other users
	s.sessions.BroadcastToNotebook(notebookID, map[string]interface{}{
		"type": "user-joined",
		"user": map[string]interface{}{
			"id":       userID,
			"username": username,
		},
	}, userID)

	// Handle messages
	for {
		var message map[string]interface{}
		if err := conn.ReadJSON(&message); err != nil {
			break
		}

		s.sessions.UpdateActivity(notebookID, userID)

		// Handle different message types
		switch message["type"] {
		case "cell-update":
			// TODO: Handle cell updates with Y.js
			s.sessions.BroadcastToNotebook(notebookID, message, userID)

		case "cell-execute":
			// TODO: Execute cell and broadcast results

		case "cursor-position":
			// Broadcast cursor position to other users
			s.sessions.BroadcastToNotebook(notebookID, message, userID)

		case "user-presence":
			// Update user presence
			s.sessions.UpdateActivity(notebookID, userID)
		}
	}

	// Notify other users of disconnection
	s.sessions.BroadcastToNotebook(notebookID, map[string]interface{}{
		"type": "user-left",
		"user": map[string]interface{}{
			"id":       userID,
			"username": username,
		},
	}, userID)
}

// Context and namespace handlers

type ContextInfo struct {
	Context   string `json:"context"`
	Namespace string `json:"namespace,omitempty"`
}

func (s *Server) getContext(c *gin.Context) {
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

	// Get the current namespace from the executor (this should be per-cell in the future)
	namespace := "default"
	if s.executor != nil {
		// For now, we'll return "default" but this could be enhanced to track per-cell namespaces
		namespace = "default"
	}

	c.JSON(http.StatusOK, ContextInfo{
		Context:   currentContext,
		Namespace: namespace,
	})
}

func (s *Server) getNamespaces(c *gin.Context) {
	// Create a new API server provider to get the clientset
	provider, err := apiserver.NewAPIServerProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create provider: %v", err)})
		return
	}

	// Cast to APIServerProvider to access GetClientset
	apiProvider, ok := provider.(*apiserver.APIServerProvider)
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
		"current":    "default", // For notebook, this would be per-cell
	})
}

func (s *Server) setNamespace(c *gin.Context) {
	var req struct {
		Namespace string `json:"namespace"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// For notebooks, we don't set a global namespace since each cell can have its own
	// Instead, we just acknowledge the request and let the frontend handle per-cell namespaces
	c.JSON(http.StatusOK, gin.H{
		"namespace": req.Namespace,
		"message":   "Namespace selection acknowledged (handled per-cell in notebooks)",
	})
}

func (s *Server) getSharedNotebook(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token parameter is required"})
		return
	}

	// Clean up expired tokens first
	s.store.DeleteExpiredShareTokens()

	// Validate the token
	shareToken, err := s.store.GetShareToken(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token validation failed"})
		return
	}

	if shareToken == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	// Check if token is expired
	if time.Now().After(shareToken.ExpiresAt) {
		s.store.DeleteShareToken(token)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token has expired"})
		return
	}

	// Get the notebook
	notebook, err := s.store.GetNotebook(shareToken.NotebookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notebook not found"})
		return
	}

	// Get cells for the notebook
	cells, err := s.store.GetCells(shareToken.NotebookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Ensure cells is not nil
	if cells == nil {
		cells = []*Cell{}
	}

	response := struct {
		*Notebook
		Cells []*Cell `json:"cells"`
	}{
		Notebook: notebook,
		Cells:    cells,
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) handleAutocomplete(c *gin.Context) {
	query := c.Query("query")
	pos := c.Query("position")

	position, err := strconv.Atoi(pos)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid position"})
		return
	}

	completer := &CyphernetesCompleter{executor: s.executor}
	suggestions, _ := completer.Do([]rune(query), position)

	// Convert [][]rune to []string
	stringSuggestions := make([]string, len(suggestions))
	for i, suggestion := range suggestions {
		stringSuggestions[i] = string(suggestion)
	}

	c.JSON(http.StatusOK, gin.H{"suggestions": stringSuggestions})
}
