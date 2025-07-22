package notebook

import (
	"time"
)

// CellType represents the type of notebook cell
type CellType string

const (
	CellTypeQuery    CellType = "query"
	CellTypeMarkdown CellType = "markdown"
	CellTypeWebpage  CellType = "webpage"
)

// VisualizationType represents how to display query results
type VisualizationType string

const (
	VisTypeJSON  VisualizationType = "json"
	VisTypeYAML  VisualizationType = "yaml"
	VisTypeTable VisualizationType = "table"
	VisTypeGraph VisualizationType = "graph"
)

// LayoutMode represents how cells are arranged in a row
type LayoutMode int

const (
	LayoutSingle LayoutMode = 1 // Full width
	LayoutDouble LayoutMode = 2 // Two cells side by side
	LayoutQuad   LayoutMode = 4 // Four cells in a row
)

// Notebook represents a Cyphernetes notebook
type Notebook struct {
	ID         string    `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	OwnerID    string    `json:"owner_id" db:"owner_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	SharedWith []string  `json:"shared_with"`
	IsPublic   bool      `json:"is_public" db:"is_public"`
	Tags       []string  `json:"tags"`
	Cells      []*Cell   `json:"cells,omitempty"` // Cells in the notebook (for count purposes)
}

// Cell represents a single cell in a notebook
type Cell struct {
	ID                string            `json:"id" db:"id"`
	NotebookID        string            `json:"notebook_id" db:"notebook_id"`
	Type              CellType          `json:"type" db:"type"`
	Name              string            `json:"name,omitempty" db:"name"`
	Query             string            `json:"query" db:"query"`
	VisualizationType VisualizationType `json:"visualization_type" db:"visualization_type"`
	RefreshInterval   int               `json:"refresh_interval" db:"refresh_interval"` // seconds
	Position          int               `json:"position" db:"position"`                 // order in notebook
	RowIndex          int               `json:"row_index" db:"row_index"`               // which row
	ColIndex          int               `json:"col_index" db:"col_index"`               // position in row
	LayoutMode        LayoutMode        `json:"layout_mode" db:"layout_mode"`           // for the row
	Results           interface{}       `json:"results,omitempty"`
	LastExecuted      *time.Time        `json:"last_executed,omitempty" db:"last_executed"`
	IsRunning         bool              `json:"is_running"`
	Error             string            `json:"error,omitempty" db:"error"`
	Config            CellConfig        `json:"config" db:"config"` // JSON stored as string in DB
}

// CellConfig holds visualization-specific configuration
type CellConfig struct {
	// For table visualization
	PageSize int `json:"page_size,omitempty"`

	// For graph visualization
	GraphMode   string `json:"graph_mode,omitempty"`   // "force", "pie", "tree"
	GraphLayout string `json:"graph_layout,omitempty"` // "force", "circular", "tree"
	NodeSize    int    `json:"node_size,omitempty"`

	// For document visualization
	DocumentMode string `json:"document_mode,omitempty"` // "json", "yaml"

	// For visualization mode
	VisualizationMode string `json:"visualization_mode,omitempty"` // "document", "table", "graph"

	// Common settings
	Height int `json:"height,omitempty"` // px

	// Context settings
	Context   string `json:"context,omitempty"`
	Namespace string `json:"namespace,omitempty"`

	// Logs cell specific
	LogsMode       string `json:"logsMode,omitempty"`       // "pod" or "query"
	LogsStreaming  bool   `json:"logs_streaming,omitempty"` // whether logs are currently streaming
}

// User represents a notebook user
type User struct {
	ID          string    `json:"id" db:"id"`
	Username    string    `json:"username" db:"username"`
	DisplayName string    `json:"display_name" db:"display_name"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Session represents an active collaboration session
type Session struct {
	ID           string    `json:"id"`
	NotebookID   string    `json:"notebook_id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastActivity time.Time `json:"last_activity"`
	IsOwner      bool      `json:"is_owner"`
}

// SharePin represents a temporary pin for sharing
type SharePin struct {
	Pin        string    `json:"pin" db:"pin"`
	NotebookID string    `json:"notebook_id" db:"notebook_id"`
	CreatedBy  string    `json:"created_by" db:"created_by"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	ExpiresAt  time.Time `json:"expires_at" db:"expires_at"`
	WGEndpoint string    `json:"wg_endpoint" db:"wg_endpoint"` // WireGuard endpoint
	WGPubKey   string    `json:"wg_pubkey" db:"wg_pubkey"`     // WireGuard public key
}

// WireGuardPeer represents a connected peer
type WireGuardPeer struct {
	ID         string    `json:"id" db:"id"`
	PublicKey  string    `json:"public_key" db:"public_key"`
	Endpoint   string    `json:"endpoint" db:"endpoint"`
	AllowedIPs string    `json:"allowed_ips" db:"allowed_ips"`
	UserID     string    `json:"user_id" db:"user_id"`
	AddedAt    time.Time `json:"added_at" db:"added_at"`
}
