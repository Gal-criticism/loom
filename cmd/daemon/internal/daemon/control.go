package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/loom/daemon/internal/session"
	"github.com/loom/daemon/internal/store"
)

// ControlServer provides HTTP control interface for the daemon
type ControlServer struct {
	daemon   *Daemon
	server   *http.Server
	listener net.Listener
	logger   *slog.Logger
}

// NewControlServer creates a new control server
func NewControlServer(daemon *Daemon, logger *slog.Logger) *ControlServer {
	if logger == nil {
		logger = slog.Default()
	}

	return &ControlServer{
		daemon: daemon,
		logger: logger,
	}
}

// Start starts the control server on a Unix socket
func (s *ControlServer) Start() error {
	mux := http.NewServeMux()

	// Health and status
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)

	// Session management
	mux.HandleFunc("/sessions", s.handleSessions)
	mux.HandleFunc("/sessions/", s.handleSessionDetail)

	// Legacy endpoints for compatibility
	mux.HandleFunc("/session/start", s.handleSessionStart)
	mux.HandleFunc("/session/stop", s.handleSessionStop)
	mux.HandleFunc("/session/thinking", s.handleSessionThinking)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Create Unix socket
	socketPath := s.getSocketPath()
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket: %w", err)
	}

	s.listener = listener

	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("control server error", "error", err)
		}
	}()

	s.logger.Info("control server started", "socket", socketPath)
	return nil
}

// Stop stops the control server
func (s *ControlServer) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Port returns the socket path
func (s *ControlServer) SocketPath() string {
	return s.getSocketPath()
}

func (s *ControlServer) getSocketPath() string {
	return filepath.Join(s.daemon.configDir, "daemon.sock")
}

// Handler methods

func (s *ControlServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": s.daemon.GetVersion(),
		"uptime":  s.daemon.GetUptime().String(),
	})
}

func (s *ControlServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.daemon.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *ControlServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listSessions(w, r)
	case http.MethodPost:
		s.createSession(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *ControlServer) listSessions(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := store.ListOptions{}

	if status := r.URL.Query().Get("status"); status != "" {
		opts.Status = store.SessionStatus(status)
	}
	if runtime := r.URL.Query().Get("runtime"); runtime != "" {
		opts.Runtime = runtime
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			opts.Limit = n
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if n, err := strconv.Atoi(offset); err == nil {
			opts.Offset = n
		}
	}

	sessions, err := s.daemon.ListSessions(opts)
	if err != nil {
		s.logger.Error("failed to list sessions", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

func (s *ControlServer) createSession(w http.ResponseWriter, r *http.Request) {
	var opts session.CreateOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	session, err := s.daemon.CreateSession(opts)
	if err != nil {
		s.logger.Error("failed to create session", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session)
}

func (s *ControlServer) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from path
	path := strings.TrimPrefix(r.URL.Path, "/sessions/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]

	switch r.Method {
	case http.MethodGet:
		s.getSession(w, r, sessionID)
	case http.MethodPost:
		if len(parts) > 1 {
			switch parts[1] {
			case "resume":
				s.resumeSession(w, r, sessionID)
			case "archive":
				s.archiveSession(w, r, sessionID)
			case "unarchive":
				s.unarchiveSession(w, r, sessionID)
			case "stop":
				s.stopSession(w, r, sessionID)
			case "messages":
				s.getSessionMessages(w, r, sessionID)
			default:
				http.Error(w, "Unknown action", http.StatusNotFound)
			}
		} else {
			http.Error(w, "Action required", http.StatusBadRequest)
		}
	case http.MethodDelete:
		s.deleteSession(w, r, sessionID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *ControlServer) getSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, err := s.daemon.GetSessionManager().GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *ControlServer) resumeSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if err := s.daemon.ResumeSession(sessionID); err != nil {
		s.logger.Error("failed to resume session", "session_id", sessionID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

func (s *ControlServer) archiveSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if err := s.daemon.ArchiveSession(sessionID); err != nil {
		s.logger.Error("failed to archive session", "session_id", sessionID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "archived"})
}

func (s *ControlServer) unarchiveSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if err := s.daemon.GetSessionManager().UnarchiveSession(sessionID); err != nil {
		s.logger.Error("failed to unarchive session", "session_id", sessionID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "unarchived"})
}

func (s *ControlServer) stopSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if err := s.daemon.GetSessionManager().StopSession(sessionID); err != nil {
		s.logger.Error("failed to stop session", "session_id", sessionID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (s *ControlServer) deleteSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if err := s.daemon.GetSessionManager().GetStore().Delete(sessionID); err != nil {
		s.logger.Error("failed to delete session", "session_id", sessionID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *ControlServer) getSessionMessages(w http.ResponseWriter, r *http.Request, sessionID string) {
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil {
			offset = n
		}
	}

	messages, err := s.daemon.GetSessionManager().GetMessages(sessionID, limit, offset)
	if err != nil {
		s.logger.Error("failed to get messages", "session_id", sessionID, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
	})
}

// Legacy handlers

func (s *ControlServer) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path    string `json:"path"`
		Runtime string `json:"runtime"`
		PID     int    `json:"pid"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	session := s.daemon.TrackSession(req.PID, req.Path, req.Runtime)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session)
}

func (s *ControlServer) handleSessionStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := s.daemon.StopSession(req.SessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (s *ControlServer) handleSessionThinking(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	thinking, err := s.daemon.GetSessionThinking(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": sessionID,
		"thinking":   thinking,
	})
}
