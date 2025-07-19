# Design Notes

This file contains detailed design specifications and architectural decisions for the new Cyphernetes program.

## Architecture Design

### High-Level Architecture

**Frontend Stack**:
- React + TypeScript (consistent with existing web UI)
- WebSocket for real-time collaboration
- State management for notebook data (Redux/Zustand)
- Rich editor component for query cells (Monaco Editor or CodeMirror)
- Visualization libraries:
  - Existing: react-force-graph-2d for graph visualizations
  - New: react-table for tabular data
  - JSON/YAML viewers with syntax highlighting

**Backend Components**:
- Go backend extending existing Cyphernetes web server
- WebSocket server for real-time collaboration
- Notebook storage service (file-based or database)
- Query execution engine (leverage existing Cyphernetes core)
- User management and authentication service
- Session management for collaborative editing

**Key Architectural Decisions**:
1. Build as a new module within existing codebase vs separate service
2. Storage mechanism for notebooks (SQLite, PostgreSQL, or file-based)
3. Authentication strategy (JWT, sessions, OAuth integration)
4. Real-time sync protocol (WebSocket with CRDT or operational transforms)
5. Query execution model (continuous polling vs WebSocket push)

## API Design

### REST API Endpoints

**Notebook Management**:
```
GET    /api/notebooks              # List all notebooks
POST   /api/notebooks              # Create new notebook
GET    /api/notebooks/:id          # Get notebook details
PUT    /api/notebooks/:id          # Update notebook metadata
DELETE /api/notebooks/:id          # Delete notebook
POST   /api/notebooks/:id/fork     # Fork a notebook
```

**Cell Operations**:
```
POST   /api/notebooks/:id/cells    # Add cell to notebook
PUT    /api/notebooks/:id/cells/:cellId    # Update cell
DELETE /api/notebooks/:id/cells/:cellId    # Delete cell
POST   /api/notebooks/:id/cells/:cellId/execute    # Execute single cell
POST   /api/notebooks/:id/execute-all    # Execute all cells
```

**Collaboration**:
```
POST   /api/share/generate-pin     # Generate pin for sharing
POST   /api/share/connect          # Connect using pin
GET    /api/share/sessions         # List active sessions
DELETE /api/share/sessions/:id     # Disconnect session
```

**WireGuard Management**:
```
GET    /api/wireguard/status       # WireGuard interface status
POST   /api/wireguard/peer         # Add peer using pin
DELETE /api/wireguard/peer/:id     # Remove peer
```

### WebSocket Protocol

**Connection**: `ws://localhost:8080/ws/notebook/:id`

**Message Types**:
```typescript
// Client -> Server
type ClientMessage = 
  | { type: "cell-update", cellId: string, changes: Y.js update }
  | { type: "cell-execute", cellId: string }
  | { type: "cursor-position", cellId: string, position: number }
  | { type: "user-presence", status: "active" | "idle" }

// Server -> Client  
type ServerMessage =
  | { type: "sync-update", update: Uint8Array }  // Y.js sync
  | { type: "execution-result", cellId: string, result: any }
  | { type: "execution-status", cellId: string, status: "running" | "complete" | "error" }
  | { type: "user-joined", userId: string, username: string }
  | { type: "user-left", userId: string }
  | { type: "cursor-update", userId: string, cellId: string, position: number }
```

## Data Models

### Core Entities

**Notebook**:
```go
type Notebook struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    OwnerID     string    `json:"owner_id"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    Cells       []Cell    `json:"cells"`
    SharedWith  []string  `json:"shared_with"` // User IDs
    IsPublic    bool      `json:"is_public"`
    Tags        []string  `json:"tags"`
}
```

**Cell**:
```go
type Cell struct {
    ID              string           `json:"id"`
    NotebookID      string           `json:"notebook_id"`
    Type            CellType         `json:"type"` // "query", "markdown", "visualization"
    Query           string           `json:"query"`
    VisualizationType VisType        `json:"visualization_type"` // "json", "yaml", "table", "graph", etc.
    RefreshInterval int              `json:"refresh_interval"` // seconds, 0 = manual only
    Position        Position         `json:"position"` // for dashboard layout
    Results         interface{}      `json:"results"`
    LastExecuted    time.Time        `json:"last_executed"`
    IsRunning       bool             `json:"is_running"`
    Config          CellConfig       `json:"config"` // visualization-specific settings
}
```

**User**:
```go
type User struct {
    ID           string    `json:"id"`
    Username     string    `json:"username"`
    Email        string    `json:"email"`
    CreatedAt    time.Time `json:"created_at"`
    Notebooks    []string  `json:"notebooks"` // Notebook IDs
    Preferences  UserPrefs `json:"preferences"`
}
```

**Collaboration Session**:
```go
type CollabSession struct {
    NotebookID   string              `json:"notebook_id"`
    ActiveUsers  map[string]UserInfo `json:"active_users"`
    Cursors      map[string]Cursor   `json:"cursors"`
    LastActivity time.Time           `json:"last_activity"`
}
```

## Real-time Collaboration: OT vs CRDT

### Operational Transforms (OT)
**How it works**: When users make concurrent edits, OT transforms operations so they can be applied in any order while maintaining consistency.

**Pros**:
- Mature technology (used by Google Docs)
- Smaller payload size (sends operations, not full state)
- Good libraries exist (ShareJS, OT.js)
- Works well for text editing

**Cons**:
- Requires central server to order operations
- Complex to implement correctly
- Can have edge cases with network partitions

### CRDTs (Conflict-free Replicated Data Types)
**How it works**: Data structures that automatically merge concurrent changes without conflicts.

**Pros**:
- No central coordination needed (perfect for P2P)
- Eventually consistent by design
- Handles network partitions gracefully
- Simpler mental model

**Cons**:
- Larger memory/storage footprint
- Can be slower for large documents
- Fewer mature libraries for complex use cases

### Recommendation for Cyphernetes Notebooks
**Use CRDTs** - specifically Yjs (https://yjs.dev/)

**Reasons**:
1. **Perfect for P2P**: No central server needed to coordinate
2. **WireGuard-friendly**: Works well with intermittent connections
3. **Good for our use case**: Notebook cells are relatively independent
4. **Yjs is production-ready**: Used by many projects, has y-websocket for easy integration
5. **Supports our data types**: Text, JSON, arrays - everything we need

**Implementation approach**:
- Each notebook is a Yjs document
- Cells are Y.Array items
- Cell content uses Y.Text for queries
- Metadata uses Y.Map for properties
- y-websocket for WebSocket synchronization

## Integration Points

### Existing Components to Leverage

1. **Core Query Engine** (`pkg/core`)
   - Use existing parser and executor
   - Leverage relationship mappings
   - Reuse macro system

2. **API Server Provider** (`pkg/provider/apiserver`)
   - Direct integration for query execution
   - Inherits local kubeconfig permissions

3. **Web Assets** (`web/`)
   - Reuse syntax highlighting components
   - Adapt graph visualization (react-force-graph-2d)
   - Share TypeScript types where possible

### New Components Needed

1. **Notebook Command** (`cmd/cyphernetes/notebook.go`)
   - New Cobra command
   - Starts HTTP/WebSocket server
   - Manages WireGuard interface

2. **Notebook Server** (`pkg/notebook/`)
   - HTTP routes for notebook operations
   - WebSocket handler for real-time sync
   - SQLite storage layer
   - Pin-code authentication

3. **Notebook Frontend** (`notebook/`)
   - New React app for notebook UI (following same pattern as `web/`)
   - Yjs integration for collaboration
   - Cell components with visualizations
   - WireGuard connection management UI
   - Built with Vite/TypeScript like existing web UI

## Performance Considerations
*Performance requirements and optimization strategies*

## Security Considerations
*Security requirements and measures*