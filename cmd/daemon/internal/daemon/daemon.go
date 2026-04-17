// Package daemon provides the resident daemon service for Loom.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/loom/daemon/internal/session"
	"github.com/loom/daemon/internal/store"
)

const (
	// Version is the daemon version
	Version = "0.3.0"

	// StatusRunning indicates a session is active
	StatusRunning = "running"
	// StatusStopped indicates a session has stopped
	StatusStopped = "stopped"
	// StatusError indicates a session encountered an error
	StatusError = "error"
)

// TrackedSession represents a tracked runtime session
type TrackedSession struct {
	SessionID string    `json:"session_id"`
	PID       int       `json:"pid"`
	Path      string    `json:"path"`
	Runtime   string    `json:"runtime"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}

// Daemon is the resident daemon service
type Daemon struct {
	pidToSession map[int]*TrackedSession
	sessionMu    sync.RWMutex

	controlServer *ControlServer
	wsClient      WebSocketClient
	machineID     string
	version       string
	startTime     time.Time

	configDir string
	state     *State
	stateMu   sync.RWMutex

	shutdownCh   chan struct{}
	shutdownOnce sync.Once
	logger       *slog.Logger

	// Callbacks for session lifecycle
	onSessionStart func(session *TrackedSession)
	onSessionStop  func(sessionID string)

	// Phase 4: Session Management
	sessionManager *session.Manager
}

// WebSocketClient interface for backend communication
type WebSocketClient interface {
	Connect(ctx context.Context) error
	Close() error
	Send(msgType string, payload interface{}) error
	On(event string, handler func(json.RawMessage) error)
	IsConnected() bool
}

// New creates a new Daemon instance
func New(logger *slog.Logger) (*Daemon, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	machineID, err := getOrCreateMachineID(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get machine ID: %w", err)
	}

	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	// Initialize session manager
	sessionDir := filepath.Join(configDir, "sessions")
	sessionMgr, err := session.NewManager(sessionDir, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	d := &Daemon{
		pidToSession:   make(map[int]*TrackedSession),
		machineID:      machineID,
		version:        Version,
		startTime:      time.Now(),
		configDir:      configDir,
		state:          NewState(),
		shutdownCh:     make(chan struct{}),
		logger:         logger,
		sessionManager: sessionMgr,
	}

	return d, nil
}

// GetMachineID returns the daemon's machine ID
func (d *Daemon) GetMachineID() string {
	return d.machineID
}

// GetVersion returns the daemon version
func (d *Daemon) GetVersion() string {
	return d.version
}

// GetStartTime returns when the daemon started
func (d *Daemon) GetStartTime() time.Time {
	return d.startTime
}

// GetUptime returns how long the daemon has been running
func (d *Daemon) GetUptime() time.Duration {
	return time.Since(d.startTime)
}

// GetStatus returns the current daemon status
func (d *Daemon) GetStatus() Status {
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()

	return Status{
		Version:       d.version,
		MachineID:     d.machineID,
		StartTime:     d.startTime,
		Uptime:        d.GetUptime().String(),
		Sessions:      d.getSessionsLocked(),
		SessionCount:  len(d.pidToSession),
		WebSocketConn: d.wsClient != nil && d.wsClient.IsConnected(),
	}
}

// Status represents the daemon's current status
type Status struct {
	Version       string            `json:"version"`
	MachineID     string            `json:"machine_id"`
	StartTime     time.Time         `json:"start_time"`
	Uptime        string            `json:"uptime"`
	Sessions      []*TrackedSession `json:"sessions"`
	SessionCount  int               `json:"session_count"`
	WebSocketConn bool              `json:"websocket_connected"`
}

// TrackSession adds a new session to track
func (d *Daemon) TrackSession(pid int, path, runtime string) *TrackedSession {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	session := &TrackedSession{
		SessionID: uuid.New().String(),
		PID:       pid,
		Path:      path,
		Runtime:   runtime,
		StartedAt: time.Now(),
		Status:    StatusRunning,
	}

	d.pidToSession[pid] = session
	d.logger.Info("session tracked",
		"session_id", session.SessionID,
		"pid", pid,
		"path", path,
		"runtime", runtime,
	)

	// Also create in session manager
	if d.sessionManager != nil {
		_, err := d.sessionManager.CreateSession(session.CreateOptions{
			Path:    path,
			Runtime: runtime,
		})
		if err != nil {
			d.logger.Warn("failed to create session in manager", "error", err)
		}
	}

	if d.onSessionStart != nil {
		go d.onSessionStart(session)
	}

	return session
}

// UpdateSessionID updates the session ID for a tracked session (e.g., after webhook confirmation)
func (d *Daemon) UpdateSessionID(pid int, sessionID string) error {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	session, ok := d.pidToSession[pid]
	if !ok {
		return fmt.Errorf("no session found for pid %d", pid)
	}

	oldID := session.SessionID
	session.SessionID = sessionID
	d.logger.Info("session ID updated",
		"pid", pid,
		"old_id", oldID,
		"new_id", sessionID,
	)

	return nil
}

// GetSessionByPID returns a session by its PID
func (d *Daemon) GetSessionByPID(pid int) (*TrackedSession, bool) {
	d.sessionMu.RLock()
	defer d.sessionMu.RUnlock()

	session, ok := d.pidToSession[pid]
	return session, ok
}

// GetSessionByID returns a session by its session ID
func (d *Daemon) GetSessionByID(sessionID string) (*TrackedSession, bool) {
	d.sessionMu.RLock()
	defer d.sessionMu.RUnlock()

	for _, session := range d.pidToSession {
		if session.SessionID == sessionID {
			return session, true
		}
	}
	return nil, false
}

// GetSessions returns all tracked sessions
func (d *Daemon) GetSessions() []*TrackedSession {
	d.sessionMu.RLock()
	defer d.sessionMu.RUnlock()

	return d.getSessionsLocked()
}

func (d *Daemon) getSessionsLocked() []*TrackedSession {
	sessions := make([]*TrackedSession, 0, len(d.pidToSession))
	for _, session := range d.pidToSession {
		sessions = append(sessions, session)
	}
	return sessions
}

// StopSession stops a tracked session by session ID
func (d *Daemon) StopSession(sessionID string) error {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	for pid, session := range d.pidToSession {
		if session.SessionID == sessionID {
			if err := d.stopSessionLocked(pid, session, ""); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("session not found: %s", sessionID)
}

// StopSessionByPID stops a tracked session by PID
func (d *Daemon) StopSessionByPID(pid int) error {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	session, ok := d.pidToSession[pid]
	if !ok {
		return fmt.Errorf("no session found for pid %d", pid)
	}

	return d.stopSessionLocked(pid, session, "")
}

func (d *Daemon) stopSessionLocked(pid int, session *TrackedSession, errorMsg string) error {
	// Send signal to process
	process, err := os.FindProcess(pid)
	if err == nil {
		if err := process.Signal(os.Interrupt); err != nil {
			d.logger.Warn("failed to send interrupt signal, trying kill",
				"pid", pid,
				"error", err,
			)
			process.Kill()
		}
	}

	session.Status = StatusStopped
	if errorMsg != "" {
		session.Status = StatusError
		session.ErrorMsg = errorMsg
	}

	delete(d.pidToSession, pid)
	d.logger.Info("session stopped",
		"session_id", session.SessionID,
		"pid", pid,
		"status", session.Status,
	)

	if d.onSessionStop != nil {
		go d.onSessionStop(session.SessionID)
	}

	return nil
}

// CleanupDeadSessions removes sessions for processes that no longer exist
func (d *Daemon) CleanupDeadSessions() {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	for pid, session := range d.pidToSession {
		process, err := os.FindProcess(pid)
		if err != nil {
			d.logger.Info("cleaning up dead session (process not found)",
				"session_id", session.SessionID,
				"pid", pid,
			)
			session.Status = StatusStopped
			delete(d.pidToSession, pid)
			continue
		}

		// On Unix systems, FindProcess always succeeds, so we need to send signal 0
		// to check if process exists
		if err := process.Signal(syscall.Signal(0)); err != nil {
			d.logger.Info("cleaning up dead session (process dead)",
				"session_id", session.SessionID,
				"pid", pid,
			)
			session.Status = StatusStopped
			delete(d.pidToSession, pid)
		}
	}
}

// StopAllSessions stops all tracked sessions
func (d *Daemon) StopAllSessions() {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	for pid, session := range d.pidToSession {
		d.stopSessionLocked(pid, session, "daemon shutting down")
	}
}

// SetWebSocketClient sets the WebSocket client
func (d *Daemon) SetWebSocketClient(client WebSocketClient) {
	d.wsClient = client
}

// GetWebSocketClient returns the WebSocket client
func (d *Daemon) GetWebSocketClient() WebSocketClient {
	return d.wsClient
}

// Shutdown initiates daemon shutdown
func (d *Daemon) Shutdown() {
	d.shutdownOnce.Do(func() {
		d.logger.Info("shutdown initiated")

		// Shutdown session manager
		if d.sessionManager != nil {
			if err := d.sessionManager.Shutdown(); err != nil {
				d.logger.Warn("session manager shutdown error", "error", err)
			}
		}

		close(d.shutdownCh)
	})
}

// WaitForShutdown blocks until shutdown is initiated
func (d *Daemon) WaitForShutdown() <-chan struct{} {
	return d.shutdownCh
}

// SetOnSessionStart sets the callback for session start
func (d *Daemon) SetOnSessionStart(cb func(session *TrackedSession)) {
	d.onSessionStart = cb
}

// SetOnSessionStop sets the callback for session stop
func (d *Daemon) SetOnSessionStop(cb func(sessionID string)) {
	d.onSessionStop = cb
}

// getConfigDir returns the configuration directory
func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(home, ".loom")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	return configDir, nil
}

// getOrCreateMachineID returns the machine ID, creating one if it doesn't exist
func getOrCreateMachineID(configDir string) (string, error) {
	machineIDPath := filepath.Join(configDir, "machine_id")

	// Try to read existing machine ID
	data, err := os.ReadFile(machineIDPath)
	if err == nil && len(data) > 0 {
		return string(data), nil
	}

	// Generate new machine ID
	machineID := uuid.New().String()
	if err := os.WriteFile(machineIDPath, []byte(machineID), 0644); err != nil {
		return "", err
	}

	return machineID, nil
}

// GetSessionManager returns the session manager
func (d *Daemon) GetSessionManager() *session.Manager {
	return d.sessionManager
}

// CreateSession creates a new session via the session manager
func (d *Daemon) CreateSession(opts session.CreateOptions) (*store.Session, error) {
	return d.sessionManager.CreateSession(opts)
}

// ResumeSession resumes a session via the session manager
func (d *Daemon) ResumeSession(sessionID string) error {
	return d.sessionManager.ResumeSession(sessionID)
}

// ArchiveSession archives a session via the session manager
func (d *Daemon) ArchiveSession(sessionID string) error {
	return d.sessionManager.ArchiveSession(sessionID)
}

// ListSessions lists sessions via the session manager
func (d *Daemon) ListSessions(opts store.ListOptions) ([]*store.Session, error) {
	return d.sessionManager.ListSessions(opts)
}

// GetSessionThinking returns thinking state for a session
func (d *Daemon) GetSessionThinking(sessionID string) (bool, error) {
	return d.sessionManager.GetSessionThinking(sessionID)
}

// StartThinkingTracking starts thinking tracking for a session
func (d *Daemon) StartThinkingTracking(sessionID string, pid int, reader io.Reader) error {
	return d.sessionManager.StartThinkingTracking(sessionID, pid, reader)
}

// StopThinkingTracking stops thinking tracking for a session
func (d *Daemon) StopThinkingTracking(sessionID string) error {
	return d.sessionManager.StopThinkingTracking(sessionID)
}

// InitializeScanner initializes the session scanner
func (d *Daemon) InitializeScanner(ctx context.Context) error {
	return d.sessionManager.InitializeScanner(ctx)
}

// InitializeHookServer initializes the hook server
func (d *Daemon) InitializeHookServer() error {
	return d.sessionManager.InitializeHookServer()
}

// GetHookServer returns the hook server
func (d *Daemon) GetHookServer() *session.HookServer {
	return d.sessionManager.GetHookServer()
}

// GetScanner returns the scanner
func (d *Daemon) GetScanner() *session.Scanner {
	return d.sessionManager.GetScanner()
}

// AutoArchiveSessions archives old inactive sessions
func (d *Daemon) AutoArchiveSessions(olderThan time.Duration) error {
	return d.sessionManager.AutoArchive(olderThan)
}

// AutoCleanupSessions permanently deletes old archived sessions
func (d *Daemon) AutoCleanupSessions(olderThan time.Duration) error {
	return d.sessionManager.AutoCleanup(olderThan)
}
