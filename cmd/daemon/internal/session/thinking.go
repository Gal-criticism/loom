package session

import (
	"bufio"
	"context"
	"io"
	"regexp"
	"sync"
	"time"
)

// ThinkingTracker tracks when AI is thinking
type ThinkingTracker struct {
	pid            int
	isThinking     bool
	mu             sync.RWMutex
	changeCallback func(bool)
	stopCh         chan struct{}
	patterns       []*regexp.Regexp
}

// NewThinkingTracker creates a new thinking tracker
func NewThinkingTracker(pid int) *ThinkingTracker {
	return &ThinkingTracker{
		pid:      pid,
		stopCh:   make(chan struct{}),
		patterns: getDefaultPatterns(),
	}
}

// Start starts tracking thinking state from the reader
func (t *ThinkingTracker) Start(ctx context.Context, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.stopCh:
			return nil
		default:
		}

		line := scanner.Text()
		t.processLine(line)
	}

	return scanner.Err()
}

// Stop stops the thinking tracker
func (t *ThinkingTracker) Stop() {
	close(t.stopCh)
}

// IsThinking returns whether the AI is currently thinking
func (t *ThinkingTracker) IsThinking() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.isThinking
}

// OnThinkingChange sets the callback for thinking state changes
func (t *ThinkingTracker) OnThinkingChange(callback func(bool)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.changeCallback = callback
}

// PID returns the tracked process ID
func (t *ThinkingTracker) PID() int {
	return t.pid
}

func (t *ThinkingTracker) processLine(line string) {
	// Check for thinking start patterns
	thinkingStarted := t.matchesPattern(line, "start")
	thinkingEnded := t.matchesPattern(line, "end")

	t.mu.Lock()
	defer t.mu.Unlock()

	if thinkingStarted && !t.isThinking {
		t.isThinking = true
		if t.changeCallback != nil {
			go t.changeCallback(true)
		}
	} else if thinkingEnded && t.isThinking {
		t.isThinking = false
		if t.changeCallback != nil {
			go t.changeCallback(false)
		}
	}
}

func (t *ThinkingTracker) matchesPattern(line, patternType string) bool {
	for _, pattern := range t.patterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

func getDefaultPatterns() []*regexp.Regexp {
	patterns := []string{
		// Claude patterns
		`(?i)thinking\.\.\.`,
		`(?i)analyzing`,
		`(?i)processing`,
		`(?i)working on it`,
		// Tool use patterns
		`(?i)using tool`,
		`(?i)calling tool`,
		`(?i)executing`,
	}

	var compiled []*regexp.Regexp
	for _, p := range patterns {
		if r, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, r)
		}
	}

	return compiled
}

// ThinkingState represents the current thinking state
type ThinkingState struct {
	SessionID  string    `json:"session_id"`
	IsThinking bool      `json:"is_thinking"`
	Since      time.Time `json:"since,omitempty"`
}

// ThinkingTrackerManager manages multiple thinking trackers
type ThinkingTrackerManager struct {
	trackers map[string]*ThinkingTracker
	mu       sync.RWMutex
}

// NewThinkingTrackerManager creates a new manager
func NewThinkingTrackerManager() *ThinkingTrackerManager {
	return &ThinkingTrackerManager{
		trackers: make(map[string]*ThinkingTracker),
	}
}

// StartTracker starts a new tracker for a session
func (m *ThinkingTrackerManager) StartTracker(sessionID string, pid int, reader io.Reader) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing tracker if any
	if existing, ok := m.trackers[sessionID]; ok {
		existing.Stop()
	}

	tracker := NewThinkingTracker(pid)
	m.trackers[sessionID] = tracker

	go func() {
		if err := tracker.Start(context.Background(), reader); err != nil {
			// Log error but don't fail
		}
	}()

	return nil
}

// StopTracker stops a tracker for a session
func (m *ThinkingTrackerManager) StopTracker(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tracker, ok := m.trackers[sessionID]; ok {
		tracker.Stop()
		delete(m.trackers, sessionID)
	}
}

// GetState returns the thinking state for a session
func (m *ThinkingTrackerManager) GetState(sessionID string) (ThinkingState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tracker, ok := m.trackers[sessionID]
	if !ok {
		return ThinkingState{}, false
	}

	return ThinkingState{
		SessionID:  sessionID,
		IsThinking: tracker.IsThinking(),
	}, true
}

// GetAllStates returns thinking states for all sessions
func (m *ThinkingTrackerManager) GetAllStates() []ThinkingState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]ThinkingState, 0, len(m.trackers))
	for sessionID, tracker := range m.trackers {
		states = append(states, ThinkingState{
			SessionID:  sessionID,
			IsThinking: tracker.IsThinking(),
		})
	}

	return states
}

// StopAll stops all trackers
func (m *ThinkingTrackerManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tracker := range m.trackers {
		tracker.Stop()
	}

	m.trackers = make(map[string]*ThinkingTracker)
}
