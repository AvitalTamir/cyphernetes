package notebook

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

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
	notebookID := c.Param("id")
	
	var cell Cell
	if err := c.ShouldBindJSON(&cell); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid JSON: %v", err)})
		return
	}

	// Get current cell count to set position
	cells, err := s.store.GetCells(notebookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get cells: %v", err)})
		return
	}

	// Set defaults
	cell.NotebookID = notebookID
	cell.Position = len(cells)
	cell.RowIndex = len(cells) // Each cell starts in its own row
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
	notebookID := c.Param("id")
	cellID := c.Param("cellId")
	
	var updates struct {
		Query             *string            `json:"query"`
		VisualizationType *VisualizationType `json:"visualization_type"`
		RefreshInterval   *int               `json:"refresh_interval"`
		Config            *CellConfig        `json:"config"`
	}

	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update the cell in the database
	err := s.store.UpdateCell(cellID, updates.Query, updates.VisualizationType, updates.RefreshInterval, updates.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cell"})
		return
	}

	// Touch the notebook to update its timestamp
	s.store.touchNotebook(notebookID)

	// Broadcast the cell update to all connected users
	s.sessions.BroadcastToNotebook(notebookID, map[string]interface{}{
		"type":   "cell-updated",
		"cellId": cellID,
		"updates": updates,
	}, "")

	c.JSON(http.StatusOK, gin.H{"message": "Cell updated successfully"})
}

func (s *Server) deleteCell(c *gin.Context) {
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

	// Execute the query
	result, err := s.executor.ExecuteQuery(targetCell.Query, "default")
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

func (s *Server) reorderCells(c *gin.Context) {
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
		"type": "cells-reordered",
		"cell_orders": request.CellOrders,
	}, "")
	
	c.JSON(http.StatusOK, gin.H{"message": "Cells reordered successfully"})
}

func (s *Server) executeAllCells(c *gin.Context) {
	// TODO: Implement execute all cells
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// Collaboration handlers

func (s *Server) generatePin(c *gin.Context) {
	// Generate a random 6-digit pin
	bytes := make([]byte, 3)
	rand.Read(bytes)
	hexStr := hex.EncodeToString(bytes)
	pin := fmt.Sprintf("%06s", hexStr[:6])

	// TODO: Store pin in database with expiration
	// TODO: Generate WireGuard keys if enabled
	// TODO: Return pin with connection info

	c.JSON(http.StatusOK, gin.H{
		"pin":        pin,
		"expires_at": time.Now().Add(10 * time.Minute),
	})
}

func (s *Server) connectWithPin(c *gin.Context) {
	// TODO: Implement pin-based connection
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

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

// WireGuard handlers

func (s *Server) wireguardStatus(c *gin.Context) {
	if !s.config.EnableWireGuard {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}

	// TODO: Check actual WireGuard interface status
	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"port":    s.config.WireGuardPort,
		"peers":   0, // TODO: Get actual peer count
	})
}

func (s *Server) addPeer(c *gin.Context) {
	// TODO: Implement WireGuard peer addition
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

func (s *Server) removePeer(c *gin.Context) {
	// TODO: Implement WireGuard peer removal
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