// Package session provides session management
package session

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/loom/daemon/internal/store"
)

// Manager manages session lifecycle
type Manager struct {
	store      store.SessionStore
	index      *store.SessionIndex
	scanner    *Scanner
	hookServer *HookServer
	logger     *slog.Logger

	activeSessions map[string]*ActiveSession
	mu             sync.RWMutex
}

// ActiveSession represents an active (running) session
type ActiveSession struct {
	Session   *store.Session
	PID       int
	StartTime time.Time
	Thinking  bool
	Tracker   *ThinkingTracker
}

// CreateOptions for creating a new session
type CreateOptions struct {
	Path    string
	Runtime string
	Title   string
}

// StartOptions for starting a session
type StartOptions struct {
	PID              int
	RuntimeSessionID string
}

// NewManager creates a new session manager
func NewManager(storePath string, logger *slog.Logger) (*Manager, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	// Create store
	s, err := store.NewFileSessionStore(storePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	// Create index
	idx := store.NewSessionIndex(s)
	if err := idx.Load(); err != nil {
		logger.Warn("failed to load session index", "error", err)
	}

	return &Manager{
		store:          s,
		index:          idx,
		activeSessions: make(map[string]*ActiveSession),
		logger:         logger,
	}, nil
}

// CreateSession creates a new session
func (m *Manager) CreateSession(opts CreateOptions) (*store.Session, error) {
	session := &store.Session{
		ID:        uuid.New().String(),
		Path:      opts.Path,
		Runtime:   opts.Runtime,
		Status:    store.StatusActive,
		Title:     opts.Title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := m.store.Create(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	m.index.Add(session)

	m.logger.Info("session created",
		"session_id", session.ID,
		"path", session.Path,
		"runtime", session.Runtime,
	)

	return session, nil
}

// StartSession starts a session with runtime
func (m *Manager) StartSession(sessionID string, opts StartOptions) error {
	session, err := m.store.Get(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already active
	if _, ok := m.activeSessions[sessionID]; ok {
		return fmt.Errorf("session already active: %s", sessionID)
	}

	// Update session
	session.Status = store.StatusActive
	session.RuntimeSessionID = opts.RuntimeSessionID
	session.UpdatedAt = time.Now()

	if err := m.store.Update(session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	m.index.Update(session)

	// Create active session
	m.activeSessions[sessionID] = &ActiveSession{
		Session:   session,
		PID:       opts.PID,
		StartTime: time.Now(),
	}

	m.logger.Info("session started",
		"session_id", sessionID,
		"pid", opts.PID,
		"runtime_session_id", opts.RuntimeSessionID,
	)

	return nil
}

// ResumeSession resumes an existing session
func (m *Manager) ResumeSession(sessionID string) error {
	session, err := m.store.Get(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	if session.Status == store.StatusArchived {
		return fmt.Errorf("cannot resume archived session")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already active
	if _, ok := m.activeSessions[sessionID]; ok {
		return nil // Already active, nothing to do
	}

	// Update session status
	session.Status = store.StatusActive
	session.UpdatedAt = time.Now()

	if err := m.store.Update(session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	m.index.Update(session)

	// Create active session entry (without PID for now)
	m.activeSessions[sessionID] = &ActiveSession{
		Session:   session,
		StartTime: time.Now(),
	}

	m.logger.Info("session resumed", "session_id", sessionID)

	return nil
}

// StopSession stops a running session
func (m *Manager) StopSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	active, ok := m.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session not active: %s", sessionID)
	}

	// Stop thinking tracker if exists
	if active.Tracker != nil {
		active.Tracker.Stop()
	}

	// Stop process if we have a PID
	if active.PID > 0 {
		process, err := os.FindProcess(active.PID)
		if err == nil {
			if err := process.Signal(os.Interrupt); err != nil {
				m.logger.Warn("failed to send interrupt, trying kill",
					"pid", active.PID,
					"error", err,
				)
				process.Kill()
			}
		}
	}

	// Update session status
	session := active.Session
	session.Status = store.StatusPaused
	session.UpdatedAt = time.Now()

	if err := m.store.Update(session); err != nil {
		m.logger.Warn("failed to update session status", "error", err)
	}

	m.index.Update(session)
	delete(m.activeSessions, sessionID)

	m.logger.Info("session stopped", "session_id", sessionID)

	return nil
}

// GetSession gets a session (from index for active, store for archived)
func (m *Manager) GetSession(id string) (*store.Session, error) {
	// Try index first (fast path)
	if session, ok := m.index.Get(id); ok {
		return session, nil
	}

	// Fall back to store
	return m.store.Get(id)
}

// ListSessions lists sessions with filter
func (m *Manager) ListSessions(opts store.ListOptions) ([]*store.Session, error) {
	// Use index for better performance
	return m.index.List(opts), nil
}

// ArchiveSession archives a session
func (m *Manager) ArchiveSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop if active
	if active, ok := m.activeSessions[id]; ok {
		if active.Tracker != nil {
			active.Tracker.Stop()
		}
		delete(m.activeSessions, id)
	}

	if err := m.store.Archive(id); err != nil {
		return err
	}

	// Update index
	if session, err := m.store.Get(id); err == nil {
		m.index.Update(session)
	}

	m.logger.Info("session archived", "session_id", id)

	return nil
}

// UnarchiveSession unarchives a session
func (m *Manager) UnarchiveSession(id string) error {
	if err := m.store.Unarchive(id); err != nil {
		return err
	}

	// Update index
	if session, err := m.store.Get(id); err == nil {
		m.index.Update(session)
	}

	m.logger.Info("session unarchived", "session_id", id)

	return nil
}

// GetActiveSessions returns all active sessions
func (m *Manager) GetActiveSessions() []*ActiveSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*ActiveSession, 0, len(m.activeSessions))
	for _, active := range m.activeSessions {
		sessions = append(sessions, active)
	}

	return sessions
}

// GetSessionThinking returns thinking state
func (m *Manager) GetSessionThinking(sessionID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active, ok := m.activeSessions[sessionID]
	if !ok {
		return false, fmt.Errorf("session not active: %s", sessionID)
	}

	if active.Tracker == nil {
		return false, nil
	}

	return active.Tracker.IsThinking(), nil
}

// StartThinkingTracking starts thinking tracking for a session
func (m *Manager) StartThinkingTracking(sessionID string, pid int, reader io.Reader) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	active, ok := m.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session not active: %s", sessionID)
	}

	// Stop existing tracker if any
	if active.Tracker != nil {
		active.Tracker.Stop()
	}

	// Create new tracker
	tracker := NewThinkingTracker(pid)
	tracker.OnThinkingChange(func(thinking bool) {
		m.mu.Lock()
		if active, ok := m.activeSessions[sessionID]; ok {
			active.Thinking = thinking
		}
		m.mu.Unlock()
	})

	active.Tracker = tracker

	// Start tracking in a goroutine
	go func() {
		if err := tracker.Start(context.Background(), reader); err != nil {
			m.logger.Warn("thinking tracker stopped", "session_id", sessionID, "error", err)
		}
	}()

	m.logger.Info("started thinking tracking", "session_id", sessionID, "pid", pid)

	return nil
}

// StopThinkingTracking stops thinking tracking for a session
func (m *Manager) StopThinkingTracking(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	active, ok := m.activeSessions[sessionID]
	if !ok {
		return fmt.Errorf("session not active: %s", sessionID)
	}

	if active.Tracker == nil {
		return nil
	}

	active.Tracker.Stop()
	active.Tracker = nil
	active.Thinking = false

	m.logger.Info("stopped thinking tracking", "session_id", sessionID)

	return nil
}

// AppendMessage appends a message to a session
func (m *Manager) AppendMessage(sessionID string, message *store.StoredMessage) error {
	if err := m.store.AppendMessage(sessionID, message); err != nil {
		return err
	}

	// Update index
	if session, err := m.store.Get(sessionID); err == nil {
		m.index.Update(session)
	}

	return nil
}

// GetMessages retrieves messages from a session
func (m *Manager) GetMessages(sessionID string, limit, offset int) ([]*store.StoredMessage, error) {
	return m.store.GetMessages(sessionID, limit, offset)
}

// AutoArchive archives old inactive sessions
func (m *Manager) AutoArchive(olderThan time.Duration) error {
	return store.ArchiveOldSessions(m.store, olderThan)
}

// AutoCleanup permanently deletes old archived sessions
func (m *Manager) AutoCleanup(olderThan time.Duration) error {
	_, err := store.CleanupArchivedSessions(m.store, olderThan)
	return err
}

// Shutdown gracefully stops all sessions
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("shutting down session manager")

	// Stop all thinking trackers
	for sessionID, active := range m.activeSessions {
		if active.Tracker != nil {
			active.Tracker.Stop()
		}

		// Update session status
		active.Session.Status = store.StatusPaused
		active.Session.UpdatedAt = time.Now()
		if err := m.store.Update(active.Session); err != nil {
			m.logger.Warn("failed to update session on shutdown", "session_id", sessionID, "error", err)
		}

		m.index.Update(active.Session)
	}

	// Clear active sessions
	m.activeSessions = make(map[string]*ActiveSession)

	// Close store
	if err := m.store.Close(); err != nil {
		return err
	}

	return nil
}

// GetStore returns the underlying store (for advanced operations)
func (m *Manager) GetStore() store.SessionStore {
	return m.store
}

// GetIndex returns the session index
func (m *Manager) GetIndex() *store.SessionIndex {
	return m.index
}

// UpdateSessionRuntimeID updates the runtime session ID for a session
func (m *Manager) UpdateSessionRuntimeID(sessionID, runtimeSessionID string) error {
	session, err := m.store.Get(sessionID)
	if err != nil {
		return err
	}

	session.RuntimeSessionID = runtimeSessionID
	session.UpdatedAt = time.Now()

	if err := m.store.Update(session); err != nil {
		return err
	}

	m.index.Update(session)

	// Update active session if exists
	m.mu.Lock()
	if active, ok := m.activeSessions[sessionID]; ok {
		active.Session.RuntimeSessionID = runtimeSessionID
	}
	m.mu.Unlock()

	return nil
}

// SetSessionError sets an error message on a session
func (m *Manager) SetSessionError(sessionID, errorMsg string) error {
	session, err := m.store.Get(sessionID)
	if err != nil {
		return err
	}

	session.Status = store.StatusError
	session.ErrorMsg = errorMsg
	session.UpdatedAt = time.Now()

	if err := m.store.Update(session); err != nil {
		return err
	}

	m.index.Update(session)

	return nil
}

// GetSessionByRuntimeID gets a session by runtime session ID
func (m *Manager) GetSessionByRuntimeID(runtimeID string) (*store.Session, error) {
	// Try index first
	if session, ok := m.index.GetByRuntimeID(runtimeID); ok {
		return session, nil
	}

	// Fall back to store
	return m.store.GetByRuntimeSessionID(runtimeID)
}

// InitializeScanner initializes the session scanner
func (m *Manager) InitializeScanner(ctx context.Context) error {
	sessionDir := m.getStoreBasePath()
	m.scanner = NewScanner(sessionDir)

	if err := m.scanner.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scanner: %w", err)
	}

	m.scanner.OnSessionFound(func(sessionID string) {
		m.handleSessionDiscovered(sessionID)
	})

	return nil
}

// InitializeHookServer initializes the hook server
func (m *Manager) InitializeHookServer() error {
	m.hookServer = NewHookServer(0) // Let OS assign port

	if err := m.hookServer.Start(); err != nil {
		return fmt.Errorf("failed to start hook server: %w", err)
	}

	m.hookServer.OnSessionHook(func(sessionID string, data SessionHookData) {
		m.handleSessionHook(sessionID, data)
	})

	return nil
}

// GetHookServer returns the hook server
func (m *Manager) GetHookServer() *HookServer {
	return m.hookServer
}

// GetScanner returns the scanner
func (m *Manager) GetScanner() *Scanner {
	return m.scanner
}

// handleSessionDiscovered handles a session discovered by the scanner
func (m *Manager) handleSessionDiscovered(sessionID string) {
	m.logger.Info("session discovered by scanner", "session_id", sessionID)

	m.mu.Lock()
	defer m.mu.Unlock()

	if active, ok := m.activeSessions[sessionID]; ok {
		active.Session.Status = store.StatusActive
		m.index.Update(active.Session)
	}
}

// handleSessionHook handles a session hook from the hook server
func (m *Manager) handleSessionHook(sessionID string, data SessionHookData) {
	m.logger.Info("session hook received",
		"session_id", sessionID,
		"event", data.Event,
		"path", data.Path,
	)

	m.mu.Lock()
	defer m.mu.Unlock()

	switch data.Event {
	case HookEventSessionStart:
		if _, ok := m.activeSessions[sessionID]; !ok {
			// Create new active session
			m.activeSessions[sessionID] = &ActiveSession{
				Session: &store.Session{
					ID:        sessionID,
					Path:      data.Path,
					Status:    store.StatusActive,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				StartTime: time.Now(),
			}
		}

	case HookEventSessionEnd:
		if active, ok := m.activeSessions[sessionID]; ok {
			active.Session.Status = store.StatusPaused
		}
	}
}

// getStoreBasePath gets the base path from the store
func (m *Manager) getStoreBasePath() string {
	// We know the underlying store is FileSessionStore
	if fs, ok := m.store.(*store.FileSessionStore); ok {
		return fs.GetBasePath()
	}
	return ""
}
