package notebook

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

//go:embed static/*
var staticFiles embed.FS

// ServerConfig holds configuration for the notebook server
type ServerConfig struct {
	Port            int
	DataDir         string
	EnableWireGuard bool
	WireGuardPort   int
}

// Server represents the notebook server
type Server struct {
	config   ServerConfig
	router   *gin.Engine
	store    *Store
	sessions *SessionManager
	executor *QueryExecutor
	upgrader websocket.Upgrader
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

	// Initialize query executor
	executor, err := NewQueryExecutor(false) // Set to true for dry-run mode
	if err != nil {
		return nil, fmt.Errorf("failed to initialize query executor: %w", err)
	}

	// Create server instance
	s := &Server{
		config:   config,
		store:    store,
		sessions: NewSessionManager(),
		executor: executor,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper origin checking for security
				return true
			},
		},
	}

	// Set up routes
	s.setupRoutes()

	return s, nil
}

// Start starts the notebook server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Port)
	return s.router.Run(addr)
}

// Stop gracefully stops the notebook server
func (s *Server) Stop() error {
	// Close the store
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close store: %w", err)
	}

	// TODO: Clean up WireGuard interfaces if enabled

	return nil
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
		api.GET("/pods", s.getPods)
		api.POST("/namespace", s.setNamespace)
		
		// Autocomplete
		api.GET("/autocomplete", s.handleAutocomplete)

		// Collaboration
		api.POST("/share/generate-pin", s.generatePin)
		api.POST("/share/connect", s.connectWithPin)
		api.GET("/share/sessions", s.listSessions)
		api.DELETE("/share/sessions/:id", s.disconnectSession)

		// WireGuard management
		api.GET("/wireguard/status", s.wireguardStatus)
		api.POST("/wireguard/peer", s.addPeer)
		api.DELETE("/wireguard/peer/:id", s.removePeer)
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
		
		// Catch-all route to serve the React app
		s.router.NoRoute(gin.WrapH(http.FileServer(http.FS(staticContent))))
	}
}