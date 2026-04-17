// Package store provides session storage interfaces and types
package store

import (
	"encoding/json"
	"time"
)

// SessionStatus represents session status
type SessionStatus string

const (
	StatusActive   SessionStatus = "active"
	StatusPaused   SessionStatus = "paused"
	StatusArchived SessionStatus = "archived"
	StatusError    SessionStatus = "error"
)

// Session represents a chat session
type Session struct {
	ID               string        `json:"id"`
	Path             string        `json:"path"`    // Working directory
	Runtime          string        `json:"runtime"` // claude/opencode
	Status           SessionStatus `json:"status"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	LastMessageAt    time.Time     `json:"last_message_at"`
	MessageCount     int           `json:"message_count"`
	RuntimeSessionID string        `json:"runtime_session_id,omitempty"` // Claude's UUID
	Title            string        `json:"title,omitempty"`
	ErrorMsg         string        `json:"error_msg,omitempty"`
}

// ListOptions for listing sessions
type ListOptions struct {
	Status  SessionStatus
	Runtime string
	Before  time.Time
	After   time.Time
	Limit   int
	Offset  int
}

// SessionStore defines session storage interface
type SessionStore interface {
	// CRUD operations
	Create(session *Session) error
	Get(id string) (*Session, error)
	Update(session *Session) error
	Delete(id string) error

	// Query operations
	List(opts ListOptions) ([]*Session, error)
	GetActive() ([]*Session, error)
	GetByRuntimeSessionID(runtimeSessionID string) (*Session, error)

	// Lifecycle operations
	Archive(id string) error
	Unarchive(id string) error
	Cleanup(before time.Time) (int, error) // returns deleted count

	// Message operations
	AppendMessage(sessionID string, message *StoredMessage) error
	GetMessages(sessionID string, limit, offset int) ([]*StoredMessage, error)
}

// StoredMessage represents a stored message
type StoredMessage struct {
	ID       string          `json:"id"`
	Time     int64           `json:"time"`
	Role     string          `json:"role"` // user/agent
	Type     string          `json:"type"` // text/tool/file/turn-start/turn-end
	Content  string          `json:"content"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// Validate validates the session
func (s *Session) Validate() error {
	if s.ID == "" {
		return ErrInvalidSession
	}
	if s.Path == "" {
		return ErrInvalidPath
	}
	if s.Runtime == "" {
		return ErrInvalidRuntime
	}
	return nil
}

// UpdateTimestamps updates the timestamps
func (s *Session) UpdateTimestamps() {
	now := time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
}
