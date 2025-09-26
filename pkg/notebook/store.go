package notebook

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Store handles persistence for notebooks
type Store struct {
	db *sql.DB
}

// NewStore creates a new store instance
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate creates or updates the database schema
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS notebooks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		owner_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		is_public BOOLEAN DEFAULT FALSE
	);

	CREATE TABLE IF NOT EXISTS cells (
		id TEXT PRIMARY KEY,
		notebook_id TEXT NOT NULL,
		type TEXT NOT NULL,
		name TEXT DEFAULT '',
		query TEXT,
		visualization_type TEXT DEFAULT 'json',
		refresh_interval INTEGER DEFAULT 0,
		position INTEGER NOT NULL,
		row_index INTEGER NOT NULL,
		col_index INTEGER NOT NULL,
		layout_mode INTEGER DEFAULT 1,
		last_executed DATETIME,
		error TEXT,
		results TEXT,
		config TEXT DEFAULT '{}',
		FOREIGN KEY (notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		display_name TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS notebook_shares (
		notebook_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		shared_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (notebook_id, user_id),
		FOREIGN KEY (notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS share_tokens (
		token TEXT PRIMARY KEY,
		notebook_id TEXT NOT NULL,
		subdomain TEXT NOT NULL,
		created_by TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY (notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS notebook_tags (
		notebook_id TEXT NOT NULL,
		tag TEXT NOT NULL,
		PRIMARY KEY (notebook_id, tag),
		FOREIGN KEY (notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE
	);

	-- Indexes for performance
	CREATE INDEX IF NOT EXISTS idx_cells_notebook ON cells(notebook_id);
	CREATE INDEX IF NOT EXISTS idx_cells_position ON cells(notebook_id, position);
	CREATE INDEX IF NOT EXISTS idx_notebook_shares_user ON notebook_shares(user_id);
	CREATE INDEX IF NOT EXISTS idx_share_pins_expires ON share_pins(expires_at);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Run migrations for existing databases
	return s.runMigrations()
}

// runMigrations handles database schema migrations
func (s *Store) runMigrations() error {
	// Check if results column exists in cells table
	var columnExists bool
	err := s.db.QueryRow("SELECT count(*) FROM pragma_table_info('cells') WHERE name='results'").Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("failed to check results column: %w", err)
	}

	// Add results column if it doesn't exist
	if !columnExists {
		_, err = s.db.Exec("ALTER TABLE cells ADD COLUMN results TEXT")
		if err != nil {
			return fmt.Errorf("failed to add results column: %w", err)
		}
	}

	// Check if name column exists in cells table
	var nameColumnExists bool
	err = s.db.QueryRow("SELECT count(*) FROM pragma_table_info('cells') WHERE name='name'").Scan(&nameColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check name column: %w", err)
	}

	// Add name column if it doesn't exist
	if !nameColumnExists {
		_, err = s.db.Exec("ALTER TABLE cells ADD COLUMN name TEXT DEFAULT ''")
		if err != nil {
			return fmt.Errorf("failed to add name column: %w", err)
		}
	}

	return nil
}

// CreateNotebook creates a new notebook
func (s *Store) CreateNotebook(name, ownerID string) (*Notebook, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `INSERT INTO notebooks (id, name, owner_id, created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query, id, name, ownerID, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create notebook: %w", err)
	}

	return &Notebook{
		ID:         id,
		Name:       name,
		OwnerID:    ownerID,
		CreatedAt:  now,
		UpdatedAt:  now,
		SharedWith: []string{},
		Tags:       []string{},
	}, nil
}

// GetNotebook retrieves a notebook by ID
func (s *Store) GetNotebook(id string) (*Notebook, error) {
	query := `SELECT id, name, owner_id, created_at, updated_at, is_public 
	          FROM notebooks WHERE id = ?`

	var nb Notebook
	err := s.db.QueryRow(query, id).Scan(
		&nb.ID, &nb.Name, &nb.OwnerID,
		&nb.CreatedAt, &nb.UpdatedAt, &nb.IsPublic,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("notebook not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notebook: %w", err)
	}

	// Load shared users
	nb.SharedWith, err = s.getSharedUsers(id)
	if err != nil {
		return nil, err
	}
	if nb.SharedWith == nil {
		nb.SharedWith = []string{}
	}

	// Load tags
	nb.Tags, err = s.getNotebookTags(id)
	if err != nil {
		return nil, err
	}
	if nb.Tags == nil {
		nb.Tags = []string{}
	}

	return &nb, nil
}

// UpdateNotebook updates notebook fields
func (s *Store) UpdateNotebook(id string, name *string) error {
	// Build dynamic update query
	setParts := []string{}
	args := []interface{}{}

	if name != nil {
		setParts = append(setParts, "name = ?")
		args = append(args, *name)
	}

	if len(setParts) == 0 {
		return nil // Nothing to update
	}

	// Always update the updated_at timestamp
	setParts = append(setParts, "updated_at = CURRENT_TIMESTAMP")

	// Add notebook ID to args
	args = append(args, id)

	updateQuery := fmt.Sprintf("UPDATE notebooks SET %s WHERE id = ?", strings.Join(setParts, ", "))
	result, err := s.db.Exec(updateQuery, args...)
	if err != nil {
		return fmt.Errorf("failed to update notebook: %w", err)
	}

	// Check if notebook was actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("notebook not found")
	}

	return nil
}

// DeleteNotebook removes a notebook and all its cells
func (s *Store) DeleteNotebook(id string) error {
	// Start a transaction to ensure all deletions succeed or fail together
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete the notebook (cells will be deleted automatically due to CASCADE)
	query := `DELETE FROM notebooks WHERE id = ?`
	result, err := tx.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete notebook: %w", err)
	}

	// Check if notebook was actually deleted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("notebook not found")
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListNotebooks lists all notebooks for a user
func (s *Store) ListNotebooks(userID string) ([]*Notebook, error) {
	query := `
	SELECT DISTINCT n.id, n.name, n.owner_id, n.created_at, n.updated_at, n.is_public,
	       COALESCE(cell_count.count, 0) as cell_count
	FROM notebooks n
	LEFT JOIN notebook_shares ns ON n.id = ns.notebook_id
	LEFT JOIN (
		SELECT notebook_id, COUNT(*) as count 
		FROM cells 
		GROUP BY notebook_id
	) cell_count ON n.id = cell_count.notebook_id
	WHERE n.owner_id = ? OR ns.user_id = ? OR n.is_public = TRUE
	ORDER BY n.updated_at DESC`

	rows, err := s.db.Query(query, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list notebooks: %w", err)
	}
	defer rows.Close()

	var notebooks []*Notebook
	for rows.Next() {
		var nb Notebook
		var cellCount int
		if err := rows.Scan(&nb.ID, &nb.Name, &nb.OwnerID,
			&nb.CreatedAt, &nb.UpdatedAt, &nb.IsPublic, &cellCount); err != nil {
			return nil, err
		}

		// Ensure arrays are not nil
		nb.SharedWith = []string{}
		nb.Tags = []string{}

		// Create empty cells array with the correct length for count purposes
		nb.Cells = make([]*Cell, cellCount)

		notebooks = append(notebooks, &nb)
	}

	return notebooks, nil
}

// CreateCell adds a new cell to a notebook
func (s *Store) CreateCell(notebookID string, cell *Cell) (*Cell, error) {
	cell.ID = uuid.New().String()
	cell.NotebookID = notebookID

	// Ensure Config is initialized
	if cell.Config.Height == 0 {
		cell.Config.Height = 300 // Default height
	}

	configJSON, err := json.Marshal(cell.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `INSERT INTO cells (id, notebook_id, type, name, query, visualization_type, 
	          refresh_interval, position, row_index, col_index, layout_mode, config)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(query, cell.ID, cell.NotebookID, cell.Type, cell.Name, cell.Query,
		cell.VisualizationType, cell.RefreshInterval, cell.Position,
		cell.RowIndex, cell.ColIndex, cell.LayoutMode, string(configJSON))

	if err != nil {
		return nil, fmt.Errorf("failed to create cell: %w", err)
	}

	// Update notebook's updated_at timestamp
	s.touchNotebook(notebookID)

	return cell, nil
}

// GetCells retrieves all cells for a notebook
func (s *Store) GetCells(notebookID string) ([]*Cell, error) {
	query := `SELECT id, notebook_id, type, name, query, visualization_type, refresh_interval,
	          position, row_index, col_index, layout_mode, last_executed, error, results, config
	          FROM cells WHERE notebook_id = ? ORDER BY position`

	rows, err := s.db.Query(query, notebookID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells: %w", err)
	}
	defer rows.Close()

	var cells []*Cell
	for rows.Next() {
		var cell Cell
		var configJSON string
		var resultsJSON sql.NullString
		var lastExec sql.NullTime
		var errStr sql.NullString

		err := rows.Scan(&cell.ID, &cell.NotebookID, &cell.Type, &cell.Name, &cell.Query,
			&cell.VisualizationType, &cell.RefreshInterval, &cell.Position,
			&cell.RowIndex, &cell.ColIndex, &cell.LayoutMode,
			&lastExec, &errStr, &resultsJSON, &configJSON)
		if err != nil {
			return nil, err
		}

		if lastExec.Valid {
			cell.LastExecuted = &lastExec.Time
		}
		if errStr.Valid {
			cell.Error = errStr.String
		}
		if resultsJSON.Valid {
			if err := json.Unmarshal([]byte(resultsJSON.String), &cell.Results); err != nil {
				return nil, fmt.Errorf("failed to unmarshal results: %w", err)
			}
		}

		// Unmarshal config
		if err := json.Unmarshal([]byte(configJSON), &cell.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		cells = append(cells, &cell)
	}

	return cells, nil
}

// Helper functions

func (s *Store) getSharedUsers(notebookID string) ([]string, error) {
	query := `SELECT user_id FROM notebook_shares WHERE notebook_id = ?`
	rows, err := s.db.Query(query, notebookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		users = append(users, userID)
	}
	return users, nil
}

func (s *Store) getNotebookTags(notebookID string) ([]string, error) {
	query := `SELECT tag FROM notebook_tags WHERE notebook_id = ?`
	rows, err := s.db.Query(query, notebookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (s *Store) touchNotebook(notebookID string) {
	query := `UPDATE notebooks SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	s.db.Exec(query, notebookID)
}

// UpdateCellResults updates the results and error for a cell
func (s *Store) UpdateCellResults(cellID string, results interface{}, errorMsg string) error {
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	query := `UPDATE cells SET results = ?, error = ?, last_executed = CURRENT_TIMESTAMP WHERE id = ?`
	_, err = s.db.Exec(query, string(resultsJSON), errorMsg, cellID)
	if err != nil {
		return fmt.Errorf("failed to update cell results: %w", err)
	}

	return nil
}

// UpdateCell updates specific fields of a cell
func (s *Store) UpdateCell(cellID string, query *string, vizType *VisualizationType, refreshInterval *int, config *CellConfig, name *string) error {
	// Build dynamic update query
	setParts := []string{}
	args := []interface{}{}

	if query != nil {
		setParts = append(setParts, "query = ?")
		args = append(args, *query)
	}

	if vizType != nil {
		setParts = append(setParts, "visualization_type = ?")
		args = append(args, *vizType)
	}

	if refreshInterval != nil {
		setParts = append(setParts, "refresh_interval = ?")
		args = append(args, *refreshInterval)
	}

	if config != nil {
		configJSON, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}
		setParts = append(setParts, "config = ?")
		args = append(args, string(configJSON))
	}

	if name != nil {
		setParts = append(setParts, "name = ?")
		args = append(args, *name)
	}

	if len(setParts) == 0 {
		return nil // Nothing to update
	}

	// Add cell ID to args
	args = append(args, cellID)

	updateQuery := fmt.Sprintf("UPDATE cells SET %s WHERE id = ?", strings.Join(setParts, ", "))
	_, err := s.db.Exec(updateQuery, args...)
	if err != nil {
		return fmt.Errorf("failed to update cell: %w", err)
	}

	return nil
}

// UpdateCellPosition updates just the position of a cell
func (s *Store) UpdateCellPosition(cellID string, position int) error {
	query := `UPDATE cells SET position = ? WHERE id = ?`
	_, err := s.db.Exec(query, position, cellID)
	if err != nil {
		return fmt.Errorf("failed to update cell position: %w", err)
	}
	return nil
}

// DeleteCell removes a cell from the database
func (s *Store) DeleteCell(cellID string) error {
	query := `DELETE FROM cells WHERE id = ?`
	_, err := s.db.Exec(query, cellID)
	if err != nil {
		return fmt.Errorf("failed to delete cell: %w", err)
	}

	return nil
}

func (s *Store) ReorderCells(notebookID string, cellOrders []struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
}) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update positions for all cells
	for _, order := range cellOrders {
		query := `UPDATE cells SET position = ?, row_index = ? WHERE id = ? AND notebook_id = ?`
		_, err := tx.Exec(query, order.Position, order.Position, order.ID, notebookID)
		if err != nil {
			return fmt.Errorf("failed to update cell position: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CreateShareToken creates a new share token for a notebook
func (s *Store) CreateShareToken(token *ShareToken) error {
	query := `INSERT INTO share_tokens (token, notebook_id, subdomain, created_by, expires_at)
	          VALUES (?, ?, ?, ?, ?)`
	
	_, err := s.db.Exec(query, token.Token, token.NotebookID, token.Subdomain,
		token.CreatedBy, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create share token: %w", err)
	}
	
	return nil
}

// GetShareToken retrieves a share token by token value
func (s *Store) GetShareToken(token string) (*ShareToken, error) {
	query := `SELECT token, notebook_id, subdomain, created_by, created_at, expires_at
	          FROM share_tokens WHERE token = ?`
	
	var st ShareToken
	err := s.db.QueryRow(query, token).Scan(
		&st.Token, &st.NotebookID, &st.Subdomain,
		&st.CreatedBy, &st.CreatedAt, &st.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Token not found
		}
		return nil, fmt.Errorf("failed to get share token: %w", err)
	}
	
	return &st, nil
}

// DeleteExpiredShareTokens removes all expired share tokens
func (s *Store) DeleteExpiredShareTokens() error {
	query := `DELETE FROM share_tokens WHERE expires_at < ?`
	_, err := s.db.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to delete expired share tokens: %w", err)
	}
	return nil
}

// DeleteShareToken removes a specific share token
func (s *Store) DeleteShareToken(token string) error {
	query := `DELETE FROM share_tokens WHERE token = ?`
	_, err := s.db.Exec(query, token)
	if err != nil {
		return fmt.Errorf("failed to delete share token: %w", err)
	}
	return nil
}

// DeleteAllShareTokens removes all share tokens (used on startup)
func (s *Store) DeleteAllShareTokens() error {
	query := `DELETE FROM share_tokens`
	result, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to delete all share tokens: %w", err)
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("🧹 Cleaned up %d expired share tokens on startup\n", rowsAffected)
	}
	
	return nil
}
