package notebook

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SessionManager manages active collaboration sessions
type SessionManager struct {
	sessions map[string]map[string]*UserSession // notebookID -> userID -> session
	mu       sync.RWMutex
}

// UserSession represents a single user's session
type UserSession struct {
	UserID       string
	Username     string
	Conn         *websocket.Conn
	NotebookID   string
	ConnectedAt  time.Time
	LastActivity time.Time
	IsOwner      bool
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]map[string]*UserSession),
	}
}

// AddSession adds a new user session
func (sm *SessionManager) AddSession(notebookID, userID, username string, conn *websocket.Conn, isOwner bool) *UserSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.sessions[notebookID] == nil {
		sm.sessions[notebookID] = make(map[string]*UserSession)
	}

	session := &UserSession{
		UserID:       userID,
		Username:     username,
		Conn:         conn,
		NotebookID:   notebookID,
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
		IsOwner:      isOwner,
	}

	sm.sessions[notebookID][userID] = session
	return session
}

// RemoveSession removes a user session
func (sm *SessionManager) RemoveSession(notebookID, userID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if notebook, exists := sm.sessions[notebookID]; exists {
		delete(notebook, userID)
		if len(notebook) == 0 {
			delete(sm.sessions, notebookID)
		}
	}
}

// GetSession gets a specific user session
func (sm *SessionManager) GetSession(notebookID, userID string) (*UserSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if notebook, exists := sm.sessions[notebookID]; exists {
		session, ok := notebook[userID]
		return session, ok
	}
	return nil, false
}

// GetNotebookSessions gets all sessions for a notebook
func (sm *SessionManager) GetNotebookSessions(notebookID string) []*UserSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var sessions []*UserSession
	if notebook, exists := sm.sessions[notebookID]; exists {
		for _, session := range notebook {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// BroadcastToNotebook sends a message to all users in a notebook
func (sm *SessionManager) BroadcastToNotebook(notebookID string, message interface{}, excludeUserID string) {
	sm.mu.RLock()
	sessions := sm.GetNotebookSessions(notebookID)
	sm.mu.RUnlock()

	for _, session := range sessions {
		if session.UserID != excludeUserID {
			// Send in a goroutine to avoid blocking
			go func(s *UserSession) {
				if err := s.Conn.WriteJSON(message); err != nil {
					// Connection might be closed, remove session
					sm.RemoveSession(s.NotebookID, s.UserID)
				}
			}(session)
		}
	}
}

// UpdateActivity updates the last activity timestamp for a session
func (sm *SessionManager) UpdateActivity(notebookID, userID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if notebook, exists := sm.sessions[notebookID]; exists {
		if session, ok := notebook[userID]; ok {
			session.LastActivity = time.Now()
		}
	}
}

// CleanupInactiveSessions removes sessions that have been inactive for too long
func (sm *SessionManager) CleanupInactiveSessions(timeout time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for notebookID, notebook := range sm.sessions {
		for userID, session := range notebook {
			if now.Sub(session.LastActivity) > timeout {
				session.Conn.Close()
				delete(notebook, userID)
			}
		}
		if len(notebook) == 0 {
			delete(sm.sessions, notebookID)
		}
	}
}