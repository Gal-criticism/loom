package session

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// HookEvent types
const (
	HookEventSessionStart = "session_start"
	HookEventSessionEnd   = "session_end"
	HookEventMessage      = "message"
	HookEventThinking     = "thinking"
)

// SessionHookData represents data from a session hook
type SessionHookData struct {
	Event     string          `json:"event"`
	SessionID string          `json:"session_id"`
	Path      string          `json:"path,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// HookServer receives webhooks from runtime sessions
type HookServer struct {
	port     int
	server   *http.Server
	mu       sync.RWMutex
	callback func(string, SessionHookData)
	stopCh   chan struct{}
}

// NewHookServer creates a new hook server
func NewHookServer(port int) *HookServer {
	return &HookServer{
		port:   port,
		stopCh: make(chan struct{}),
	}
}

// Start starts the hook server
func (h *HookServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", h.handleHook)
	mux.HandleFunc("/health", h.handleHealth)

	// Let OS assign port if port is 0
	addr := fmt.Sprintf(":%d", h.port)
	if h.port == 0 {
		addr = ":0"
	}

	h.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start listener to get actual port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	h.port = listener.Addr().(*net.TCPAddr).Port

	go func() {
		if err := h.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Hook server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the hook server
func (h *HookServer) Stop() error {
	if h.server != nil {
		return h.server.Close()
	}
	return nil
}

// Port returns the port the server is listening on
func (h *HookServer) Port() int {
	return h.port
}

// OnSessionHook sets the callback for session hooks
func (h *HookServer) OnSessionHook(callback func(string, SessionHookData)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callback = callback
}

// GenerateSettingsFile generates a settings file for Claude Code
func (h *HookServer) GenerateSettingsFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	settingsDir := filepath.Join(home, ".config", "claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return "", err
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")

	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"session_start": fmt.Sprintf("http://localhost:%d/hook", h.port),
			"session_end":   fmt.Sprintf("http://localhost:%d/hook", h.port),
		},
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return "", err
	}

	return settingsPath, nil
}

func (h *HookServer) handleHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var data SessionHookData
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get session ID from data or generate one
	sessionID := data.SessionID
	if sessionID == "" {
		sessionID = "unknown"
	}

	h.mu.RLock()
	callback := h.callback
	h.mu.RUnlock()

	if callback != nil {
		go callback(sessionID, data)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *HookServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
