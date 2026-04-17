package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FileSessionStore implements SessionStore with file system
type FileSessionStore struct {
	basePath string
	mu       sync.RWMutex
	closed   bool
}

// NewFileSessionStore creates a new file-based store
func NewFileSessionStore(basePath string) (*FileSessionStore, error) {
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &FileSessionStore{
		basePath: basePath,
	}, nil
}

// Close closes the store
func (s *FileSessionStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// Create creates a new session
func (s *FileSessionStore) Create(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	if err := session.Validate(); err != nil {
		return err
	}

	session.UpdateTimestamps()

	// Check if session already exists
	sessionDir := s.sessionDir(session.ID)
	if _, err := os.Stat(sessionDir); err == nil {
		return ErrSessionExists
	}

	// Create session directory
	if err := os.MkdirAll(sessionDir, 0750); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Write metadata
	if err := s.writeSessionMetadata(session); err != nil {
		// Cleanup on failure
		os.RemoveAll(sessionDir)
		return err
	}

	// Create empty messages file
	messagesPath := filepath.Join(sessionDir, "messages.jsonl")
	if err := s.writeAtomic(messagesPath, []byte{}); err != nil {
		os.RemoveAll(sessionDir)
		return fmt.Errorf("failed to create messages file: %w", err)
	}

	return nil
}

// Get retrieves a session by ID
func (s *FileSessionStore) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	return s.readSessionMetadata(id)
}

// Update updates a session
func (s *FileSessionStore) Update(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	if err := session.Validate(); err != nil {
		return err
	}

	session.UpdateTimestamps()

	return s.writeSessionMetadata(session)
}

// Delete permanently deletes a session
func (s *FileSessionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	sessionDir := s.sessionDir(id)
	if err := os.RemoveAll(sessionDir); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// List lists sessions with filter options
func (s *FileSessionStore) List(opts ListOptions) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		if !s.isValidSessionDir(sessionID) {
			continue
		}

		session, err := s.readSessionMetadata(sessionID)
		if err != nil {
			continue // Skip invalid sessions
		}

		// Apply filters
		if opts.Status != "" && session.Status != opts.Status {
			continue
		}
		if opts.Runtime != "" && session.Runtime != opts.Runtime {
			continue
		}
		if !opts.Before.IsZero() && session.UpdatedAt.After(opts.Before) {
			continue
		}
		if !opts.After.IsZero() && session.UpdatedAt.Before(opts.After) {
			continue
		}

		sessions = append(sessions, session)
	}

	// Sort by UpdatedAt descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	// Apply limit and offset
	if opts.Offset > 0 {
		if opts.Offset >= len(sessions) {
			return []*Session{}, nil
		}
		sessions = sessions[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(sessions) {
		sessions = sessions[:opts.Limit]
	}

	return sessions, nil
}

// GetActive returns all active sessions
func (s *FileSessionStore) GetActive() ([]*Session, error) {
	return s.List(ListOptions{Status: StatusActive})
}

// GetByRuntimeSessionID finds a session by runtime session ID
func (s *FileSessionStore) GetByRuntimeSessionID(runtimeSessionID string) (*Session, error) {
	sessions, err := s.List(ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		if session.RuntimeSessionID == runtimeSessionID {
			return session, nil
		}
	}

	return nil, ErrSessionNotFound
}

// Archive archives a session
func (s *FileSessionStore) Archive(id string) error {
	session, err := s.Get(id)
	if err != nil {
		return err
	}

	session.Status = StatusArchived
	return s.Update(session)
}

// Unarchive unarchives a session
func (s *FileSessionStore) Unarchive(id string) error {
	session, err := s.Get(id)
	if err != nil {
		return err
	}

	session.Status = StatusActive
	return s.Update(session)
}

// Cleanup deletes archived sessions older than the specified time
func (s *FileSessionStore) Cleanup(before time.Time) (int, error) {
	sessions, err := s.List(ListOptions{
		Status: StatusArchived,
		Before: before,
	})
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, session := range sessions {
		if err := s.Delete(session.ID); err == nil {
			deleted++
		}
	}

	return deleted, nil
}

// AppendMessage appends a message to a session
func (s *FileSessionStore) AppendMessage(sessionID string, message *StoredMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	// Verify session exists
	sessionDir := s.sessionDir(sessionID)
	if _, err := os.Stat(sessionDir); err != nil {
		return ErrSessionNotFound
	}

	// Append message to file
	messagesPath := filepath.Join(sessionDir, "messages.jsonl")
	f, err := os.OpenFile(messagesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open messages file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Update session message count
	session, err := s.readSessionMetadata(sessionID)
	if err != nil {
		return err
	}
	session.MessageCount++
	session.LastMessageAt = time.Now()
	return s.writeSessionMetadata(session)
}

// GetMessages retrieves messages from a session
func (s *FileSessionStore) GetMessages(sessionID string, limit, offset int) ([]*StoredMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	messagesPath := filepath.Join(s.sessionDir(sessionID), "messages.jsonl")
	f, err := os.Open(messagesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*StoredMessage{}, nil
		}
		return nil, fmt.Errorf("failed to open messages file: %w", err)
	}
	defer f.Close()

	var messages []*StoredMessage
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var msg StoredMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue // Skip invalid messages
		}
		messages = append(messages, &msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan messages: %w", err)
	}

	// Reverse to get newest first
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	// Apply offset and limit
	if offset > 0 {
		if offset >= len(messages) {
			return []*StoredMessage{}, nil
		}
		messages = messages[offset:]
	}
	if limit > 0 && limit < len(messages) {
		messages = messages[:limit]
	}

	return messages, nil
}

// GetBasePath returns the base path of the store
func (s *FileSessionStore) GetBasePath() string {
	return s.basePath
}

// Helper methods

func (s *FileSessionStore) sessionDir(sessionID string) string {
	return filepath.Join(s.basePath, "session-"+sessionID)
}

func (s *FileSessionStore) isValidSessionDir(name string) bool {
	return len(name) > 8 && name[:8] == "session-"
}

func (s *FileSessionStore) readSessionMetadata(sessionID string) (*Session, error) {
	metadataPath := filepath.Join(s.sessionDir(sessionID), "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to read session metadata: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

func (s *FileSessionStore) writeSessionMetadata(session *Session) error {
	metadataPath := filepath.Join(s.sessionDir(session.ID), "metadata.json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	return s.writeAtomic(metadataPath, data)
}

// writeAtomic writes data to a file atomically
func (s *FileSessionStore) writeAtomic(path string, data []byte) error {
	// Create temp file in same directory for atomic rename
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Cleanup temp file on error
	defer func() {
		if err != nil {
			os.Remove(tempPath)
		}
	}()

	// Write data
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to sync file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions
	if err := os.Chmod(tempPath, 0600); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	// Sync parent directory to ensure rename is persisted
	if dirFile, err := os.Open(dir); err == nil {
		dirFile.Sync()
		dirFile.Close()
	}

	return nil
}
