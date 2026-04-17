package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Scanner scans for active sessions
type Scanner struct {
	basePath      string
	sessions      map[string]bool
	mu            sync.RWMutex
	foundCallback func(string)
	stopCh        chan struct{}
	running       bool
}

// NewScanner creates a new session scanner
func NewScanner(basePath string) *Scanner {
	return &Scanner{
		basePath: basePath,
		sessions: make(map[string]bool),
		stopCh:   make(chan struct{}),
	}
}

// Start starts the scanner
func (s *Scanner) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.running = true

	// Do initial scan
	s.scan()

	// Start periodic scanning
	go s.run(ctx)

	return nil
}

// Stop stops the scanner
func (s *Scanner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.running = false
}

// OnSessionFound sets the callback for when a session is found
func (s *Scanner) OnSessionFound(callback func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.foundCallback = callback
}

// ReportSession reports a session to the scanner
func (s *Scanner) ReportSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.sessions[sessionID] {
		s.sessions[sessionID] = true
		if s.foundCallback != nil {
			go s.foundCallback(sessionID)
		}
	}
}

// GetSessions returns all discovered sessions
func (s *Scanner) GetSessions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]string, 0, len(s.sessions))
	for sessionID := range s.sessions {
		sessions = append(sessions, sessionID)
	}

	return sessions
}

func (s *Scanner) run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.scan()
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scanner) scan() {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "session-") {
			continue
		}

		sessionID := strings.TrimPrefix(name, "session-")

		s.mu.Lock()
		if !s.sessions[sessionID] {
			s.sessions[sessionID] = true
			if s.foundCallback != nil {
				go s.foundCallback(sessionID)
			}
		}
		s.mu.Unlock()
	}
}

// IsRunning returns whether the scanner is running
func (s *Scanner) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}
