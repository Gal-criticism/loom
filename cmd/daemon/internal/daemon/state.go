package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionState represents persisted session state
type SessionState struct {
	SessionID string    `json:"session_id"`
	Runtime   string    `json:"runtime"`
	Path      string    `json:"path"`
	StartedAt time.Time `json:"started_at"`
	StoppedAt time.Time `json:"stopped_at,omitempty"`
	Status    string    `json:"status"`
}

// State manages persistent daemon state
type State struct {
	sessions map[string]SessionState
	mu       sync.RWMutex
	path     string
}

// NewState creates a new state manager
func NewState() *State {
	return &State{
		sessions: make(map[string]SessionState),
	}
}

// Load loads state from disk
func (s *State) Load(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var sessions []SessionState
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}

	for _, session := range sessions {
		s.sessions[session.SessionID] = session
	}

	return nil
}

// Save saves state to disk
func (s *State) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.path == "" {
		return nil
	}

	sessions := make([]SessionState, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

// AddSession adds a session to state
func (s *State) AddSession(session SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.SessionID] = session
	s.saveAsync()
}

// GetSession gets a session from state
func (s *State) GetSession(sessionID string) (SessionState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	return session, ok
}

// UpdateSession updates a session in state
func (s *State) UpdateSession(sessionID string, fn func(*SessionState)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, ok := s.sessions[sessionID]; ok {
		fn(&session)
		s.sessions[sessionID] = session
		s.saveAsync()
	}
}

// RemoveSession removes a session from state
func (s *State) RemoveSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	s.saveAsync()
}

// GetAllSessions returns all sessions
func (s *State) GetAllSessions() []SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]SessionState, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// saveAsync saves state asynchronously
func (s *State) saveAsync() {
	go s.Save()
}
