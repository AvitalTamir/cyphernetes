package notebook

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

//go:embed static/*
var staticFiles embed.FS

// ServerConfig holds configuration for the notebook server
type ServerConfig struct {
	Port    int
	DataDir string
}

// TunnelConnection represents an active tunnel connection
type TunnelConnection struct {
	Subdomain string
	Token     string
	ExpiresAt time.Time
	StopChan  chan bool
}

// Server represents the notebook server
type Server struct {
	config      ServerConfig
	router      *gin.Engine
	store       *Store
	sessions    *SessionManager
	executor    *QueryExecutor
	upgrader    websocket.Upgrader
	cleanupStop chan bool
	tunnels     map[string]*TunnelConnection // subdomain -> connection
	tunnelsMu   sync.RWMutex
}

// NewServer creates a new notebook server instance
func NewServer(config ServerConfig) (*Server, error) {
	// Expand home directory if needed
	if config.DataDir[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.DataDir = filepath.Join(home, config.DataDir[2:])
	}

	// Ensure data directory exists
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize SQLite store
	store, err := NewStore(filepath.Join(config.DataDir, "notebooks.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	// Clean up any existing share tokens on startup
	if err := store.DeleteAllShareTokens(); err != nil {
		fmt.Printf("Warning: Failed to clean up share tokens on startup: %v\n", err)
	}

	// Initialize query executor
	executor, err := NewQueryExecutor(false) // Set to true for dry-run mode
	if err != nil {
		return nil, fmt.Errorf("failed to initialize query executor: %w", err)
	}

	// Create server instance
	s := &Server{
		config:      config,
		store:       store,
		sessions:    NewSessionManager(),
		executor:    executor,
		cleanupStop: make(chan bool),
		tunnels:     make(map[string]*TunnelConnection),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper origin checking for security
				return true
			},
		},
	}

	// Set up routes
	s.setupRoutes()

	// Start periodic token cleanup
	s.startTokenCleanup()

	return s, nil
}

// Start starts the notebook server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Port)
	return s.router.Run(addr)
}

// Stop gracefully stops the notebook server
func (s *Server) Stop() error {
	// Stop the token cleanup goroutine
	close(s.cleanupStop)

	// Stop all active tunnels
	s.stopAllTunnels()

	// Close the store
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close store: %w", err)
	}

	return nil
}

// startTokenCleanup starts a background goroutine to periodically clean up expired tokens
func (s *Server) startTokenCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := s.store.DeleteExpiredShareTokens(); err != nil {
					fmt.Printf("Warning: Failed to clean up expired tokens: %v\n", err)
				}
			case <-s.cleanupStop:
				return
			}
		}
	}()
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Use Gin in release mode for production
	gin.SetMode(gin.ReleaseMode)

	s.router = gin.New()
	s.router.Use(gin.Recovery())
	s.router.Use(gin.Logger())

	// API routes
	api := s.router.Group("/api")
	{
		// Notebook management
		api.GET("/notebooks", s.listNotebooks)
		api.POST("/notebooks", s.createNotebook)
		api.GET("/notebooks/:id", s.getNotebook)
		api.PUT("/notebooks/:id", s.updateNotebook)
		api.DELETE("/notebooks/:id", s.deleteNotebook)
		api.POST("/notebooks/:id/fork", s.forkNotebook)

		// Cell operations
		api.POST("/notebooks/:id/cells", s.addCell)
		api.PUT("/notebooks/:id/cells/:cellId", s.updateCell)
		api.DELETE("/notebooks/:id/cells/:cellId", s.deleteCell)
		api.PUT("/notebooks/:id/cells/reorder", s.reorderCells)
		api.POST("/notebooks/:id/cells/:cellId/execute", s.executeCell)
		api.GET("/notebooks/:id/cells/:cellId/logs", s.getLogs)
		api.POST("/notebooks/:id/execute-all", s.executeAllCells)

		// Context and namespace operations
		api.GET("/context", s.getContext)
		api.GET("/namespaces", s.getNamespaces)
		api.POST("/namespace", s.setNamespace)

		// Autocomplete
		api.GET("/autocomplete", s.handleAutocomplete)

		// Collaboration
		api.POST("/share/generate-token", s.generateShareToken)
		api.GET("/share/sessions", s.listSessions)
		api.DELETE("/share/sessions/:id", s.disconnectSession)
	}

	// WebSocket endpoint
	s.router.GET("/ws/notebook/:id", s.handleWebSocket)

	// Serve static files from the embedded filesystem
	staticContent, err := fs.Sub(staticFiles, "static")
	if err != nil {
		fmt.Printf("Warning: Could not access embedded notebook files: %v\n", err)
		// Fallback to serving a simple message
		s.router.GET("/", func(c *gin.Context) {
			c.HTML(http.StatusOK, "index.html", gin.H{
				"title": "Cyphernetes Notebooks",
			})
		})
	} else {
		s.router.StaticFS("/static", http.FS(staticContent))

		// Apply share token validation middleware to the main route
		s.router.Use(s.validateShareToken())

		// Catch-all route to serve the React app
		s.router.NoRoute(gin.WrapH(http.FileServer(http.FS(staticContent))))
	}
}

// startTunnel creates and starts a tunnel connection
func (s *Server) startTunnel(subdomain, token string, expiresAt time.Time) error {
	s.tunnelsMu.Lock()
	defer s.tunnelsMu.Unlock()

	// Check if tunnel already exists
	if _, exists := s.tunnels[subdomain]; exists {
		return fmt.Errorf("tunnel for subdomain %s already exists", subdomain)
	}

	// Create tunnel connection
	tunnel := &TunnelConnection{
		Subdomain: subdomain,
		Token:     token,
		ExpiresAt: expiresAt,
		StopChan:  make(chan bool, 1),
	}

	s.tunnels[subdomain] = tunnel

	// Start tunnel in background
	go s.runTunnel(tunnel)

	fmt.Printf("🌐 Started tunnel: %s.go.cyphernet.es -> localhost:%d\n", subdomain, s.config.Port)
	return nil
}

// runTunnel runs the actual tunnel connection
func (s *Server) runTunnel(tunnel *TunnelConnection) {
	defer func() {
		s.tunnelsMu.Lock()
		delete(s.tunnels, tunnel.Subdomain)
		s.tunnelsMu.Unlock()
		fmt.Printf("🔌 Tunnel closed: %s.go.cyphernet.es\n", tunnel.Subdomain)
	}()

	// Establish tunnel connection to go.cyphernet.es
	if err := s.connectToTunnelService(tunnel); err != nil {
		fmt.Printf("❌ Failed to connect tunnel: %v\n", err)
		return
	}
}

// connectToTunnelService establishes a WebSocket tunnel connection with the DNS service
func (s *Server) connectToTunnelService(tunnel *TunnelConnection) error {
	// Connect to WebSocket tunnel endpoint
	tunnelURL := fmt.Sprintf("wss://go.cyphernet.es/tunnel/%s", tunnel.Subdomain)
	
	headers := http.Header{}
	headers.Add("Authorization", fmt.Sprintf("Bearer %s", tunnel.Token))
	
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	
	conn, _, err := dialer.Dial(tunnelURL, headers)
	if err != nil {
		return fmt.Errorf("failed to connect to tunnel service: %w", err)
	}
	defer conn.Close()
	
	localURL := fmt.Sprintf("http://localhost:%d", s.config.Port)
	fmt.Printf("✅ Tunnel connected: %s.go.cyphernet.es → %s\n", tunnel.Subdomain, localURL)
	
	// Handle incoming requests from the tunnel
	done := make(chan struct{})
	connMutex := &sync.Mutex{}
	go func() {
		defer close(done)
		for {
			var msg TunnelMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					fmt.Printf("❌ Tunnel read error: %v\n", err)
				}
				return
			}
			
			// Handle the request in a goroutine with shared mutex
			go s.handleTunnelRequest(conn, connMutex, &msg, localURL)
		}
	}()
	
	// Keep connection alive until expiry or stop signal
	select {
	case <-done:
		return nil
	case <-tunnel.StopChan:
		return nil
	case <-time.After(time.Until(tunnel.ExpiresAt)):
		fmt.Printf("⏰ Tunnel expired: %s.go.cyphernet.es\n", tunnel.Subdomain)
		return nil
	}
}

// TunnelMessage represents a message from the tunnel service
type TunnelMessage struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// TunnelResponse represents a response to send back through the tunnel
type TunnelResponse struct {
	ID         string            `json:"id"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

// handleTunnelRequest forwards a request to the local server and sends the response back
func (s *Server) handleTunnelRequest(conn *websocket.Conn, connMutex *sync.Mutex, msg *TunnelMessage, localURL string) {
	// Create HTTP request to local server
	var body io.Reader
	if msg.Body != "" {
		body = bytes.NewReader([]byte(msg.Body))
	}
	
	req, err := http.NewRequest(msg.Method, localURL+msg.URL, body)
	if err != nil {
		s.sendTunnelError(conn, connMutex, msg.ID, http.StatusBadRequest, "Invalid request")
		return
	}
	
	// Set headers
	for key, value := range msg.Headers {
		req.Header.Set(key, value)
	}
	
	// Make request to local server
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.sendTunnelError(conn, connMutex, msg.ID, http.StatusBadGateway, "Failed to forward request")
		return
	}
	defer resp.Body.Close()
	
	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.sendTunnelError(conn, connMutex, msg.ID, http.StatusInternalServerError, "Failed to read response")
		return
	}
	
	// Send response back through tunnel
	tunnelResp := TunnelResponse{
		ID:         msg.ID,
		StatusCode: resp.StatusCode,
		Headers:    make(map[string]string),
		Body:       string(respBody),
	}
	
	// Copy headers
	for key, values := range resp.Header {
		if len(values) > 0 {
			tunnelResp.Headers[key] = values[0]
		}
	}
	
	// Protect WebSocket writes with mutex to prevent concurrent writes
	connMutex.Lock()
	err = conn.WriteJSON(tunnelResp)
	connMutex.Unlock()
	
	if err != nil {
		fmt.Printf("Warning: Failed to send tunnel response: %v\n", err)
	}
}

// sendTunnelError sends an error response through the tunnel
func (s *Server) sendTunnelError(conn *websocket.Conn, connMutex *sync.Mutex, id string, statusCode int, message string) {
	resp := TunnelResponse{
		ID:         id,
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       fmt.Sprintf(`{"error": "%s"}`, message),
	}
	
	// Protect WebSocket writes with mutex to prevent concurrent writes
	connMutex.Lock()
	err := conn.WriteJSON(resp)
	connMutex.Unlock()
	
	if err != nil {
		fmt.Printf("Warning: Failed to send tunnel error: %v\n", err)
	}
}

// stopTunnel stops a specific tunnel
func (s *Server) stopTunnel(subdomain string) {
	s.tunnelsMu.Lock()
	defer s.tunnelsMu.Unlock()

	if tunnel, exists := s.tunnels[subdomain]; exists {
		close(tunnel.StopChan)
	}
}

// stopAllTunnels stops all active tunnels
func (s *Server) stopAllTunnels() {
	s.tunnelsMu.Lock()
	defer s.tunnelsMu.Unlock()

	for _, tunnel := range s.tunnels {
		close(tunnel.StopChan)
	}
}
