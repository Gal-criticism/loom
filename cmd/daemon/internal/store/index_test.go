package store

import (
	"fmt"
	"testing"
	"time"
)

func TestSessionIndex_Load(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create sessions
	sessions := []*Session{
		{ID: "session-1", Path: "/path/1", Runtime: "claude", Status: StatusActive},
		{ID: "session-2", Path: "/path/2", Runtime: "opencode", Status: StatusPaused},
		{ID: "session-3", Path: "/path/3", Runtime: "claude", Status: StatusArchived},
	}

	for _, s := range sessions {
		if err := store.Create(s); err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
	}

	// Create index and load
	index := NewSessionIndex(store)
	if err := index.Load(); err != nil {
		t.Errorf("Load failed: %v", err)
	}

	// Verify all sessions loaded
	if count := index.Count(); count != 3 {
		t.Errorf("Expected 3 sessions in index, got %d", count)
	}

	// Verify each session
	for _, s := range sessions {
		if _, ok := index.Get(s.ID); !ok {
			t.Errorf("Session %s not found in index", s.ID)
		}
	}
}

func TestSessionIndex_Add(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	session := &Session{
		ID:               "test-session",
		Path:             "/home/user/project",
		Runtime:          "claude",
		Status:           StatusActive,
		RuntimeSessionID: "runtime-uuid",
	}

	index.Add(session)

	// Verify added
	if _, ok := index.Get("test-session"); !ok {
		t.Error("Session not found after Add")
	}

	// Verify runtime ID mapping
	if _, ok := index.GetByRuntimeID("runtime-uuid"); !ok {
		t.Error("Session not found by runtime ID")
	}
}

func TestSessionIndex_Update(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	session := &Session{
		ID:               "test-session",
		Path:             "/home/user/project",
		Runtime:          "claude",
		Status:           StatusActive,
		RuntimeSessionID: "old-uuid",
	}

	index.Add(session)

	// Update session with new runtime ID
	session.RuntimeSessionID = "new-uuid"
	session.Status = StatusPaused
	index.Update(session)

	// Verify old mapping removed
	if _, ok := index.GetByRuntimeID("old-uuid"); ok {
		t.Error("Old runtime ID mapping should have been removed")
	}

	// Verify new mapping exists
	if _, ok := index.GetByRuntimeID("new-uuid"); !ok {
		t.Error("New runtime ID mapping not found")
	}

	// Verify status updated
	if s, _ := index.Get("test-session"); s.Status != StatusPaused {
		t.Errorf("Expected status %s, got %s", StatusPaused, s.Status)
	}
}

func TestSessionIndex_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	session := &Session{
		ID:               "test-session",
		Path:             "/home/user/project",
		Runtime:          "claude",
		Status:           StatusActive,
		RuntimeSessionID: "runtime-uuid",
	}

	index.Add(session)
	index.Remove("test-session")

	// Verify removed
	if _, ok := index.Get("test-session"); ok {
		t.Error("Session should have been removed")
	}

	// Verify runtime ID mapping removed
	if _, ok := index.GetByRuntimeID("runtime-uuid"); ok {
		t.Error("Runtime ID mapping should have been removed")
	}
}

func TestSessionIndex_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	// Add sessions with different timestamps
	now := time.Now()
	sessions := []*Session{
		{ID: "session-1", Path: "/path/1", Runtime: "claude", Status: StatusActive, UpdatedAt: now},
		{ID: "session-2", Path: "/path/2", Runtime: "opencode", Status: StatusPaused, UpdatedAt: now.Add(-1 * time.Hour)},
		{ID: "session-3", Path: "/path/3", Runtime: "claude", Status: StatusArchived, UpdatedAt: now.Add(-2 * time.Hour)},
	}

	for _, s := range sessions {
		index.Add(s)
	}

	// Test list all
	all := index.List(ListOptions{})
	if len(all) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(all))
	}

	// Test filter by status
	active := index.List(ListOptions{Status: StatusActive})
	if len(active) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(active))
	}

	// Test filter by runtime
	claudeSessions := index.List(ListOptions{Runtime: "claude"})
	if len(claudeSessions) != 2 {
		t.Errorf("Expected 2 claude sessions, got %d", len(claudeSessions))
	}

	// Test limit
	limited := index.List(ListOptions{Limit: 2})
	if len(limited) != 2 {
		t.Errorf("Expected 2 sessions with limit, got %d", len(limited))
	}

	// Test offset
	offset := index.List(ListOptions{Offset: 1})
	if len(offset) != 2 {
		t.Errorf("Expected 2 sessions with offset, got %d", len(offset))
	}

	// Test combined filters
	filtered := index.List(ListOptions{
		Status: StatusActive,
		Limit:  1,
	})
	if len(filtered) != 1 {
		t.Errorf("Expected 1 session with combined filters, got %d", len(filtered))
	}
}

func TestSessionIndex_GetActive(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	sessions := []*Session{
		{ID: "session-1", Path: "/path/1", Runtime: "claude", Status: StatusActive},
		{ID: "session-2", Path: "/path/2", Runtime: "opencode", Status: StatusPaused},
		{ID: "session-3", Path: "/path/3", Runtime: "claude", Status: StatusActive},
	}

	for _, s := range sessions {
		index.Add(s)
	}

	active := index.GetActive()
	if len(active) != 2 {
		t.Errorf("Expected 2 active sessions, got %d", len(active))
	}
}

func TestSessionIndex_GetByPath(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	sessions := []*Session{
		{ID: "session-1", Path: "/path/same", Runtime: "claude", Status: StatusActive},
		{ID: "session-2", Path: "/path/same", Runtime: "opencode", Status: StatusPaused},
		{ID: "session-3", Path: "/path/different", Runtime: "claude", Status: StatusActive},
	}

	for _, s := range sessions {
		index.Add(s)
	}

	results := index.GetByPath("/path/same")
	if len(results) != 2 {
		t.Errorf("Expected 2 sessions with path '/path/same', got %d", len(results))
	}
}

func TestSessionIndex_GetByRuntime(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	sessions := []*Session{
		{ID: "session-1", Path: "/path/1", Runtime: "claude", Status: StatusActive},
		{ID: "session-2", Path: "/path/2", Runtime: "opencode", Status: StatusPaused},
		{ID: "session-3", Path: "/path/3", Runtime: "claude", Status: StatusArchived},
	}

	for _, s := range sessions {
		index.Add(s)
	}

	claudeSessions := index.GetByRuntime("claude")
	if len(claudeSessions) != 2 {
		t.Errorf("Expected 2 claude sessions, got %d", len(claudeSessions))
	}
}

func TestSessionIndex_CountByStatus(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	sessions := []*Session{
		{ID: "session-1", Path: "/path/1", Runtime: "claude", Status: StatusActive},
		{ID: "session-2", Path: "/path/2", Runtime: "opencode", Status: StatusActive},
		{ID: "session-3", Path: "/path/3", Runtime: "claude", Status: StatusArchived},
	}

	for _, s := range sessions {
		index.Add(s)
	}

	if count := index.CountByStatus(StatusActive); count != 2 {
		t.Errorf("Expected 2 active sessions, got %d", count)
	}

	if count := index.CountByStatus(StatusArchived); count != 1 {
		t.Errorf("Expected 1 archived session, got %d", count)
	}

	if count := index.CountByStatus(StatusPaused); count != 0 {
		t.Errorf("Expected 0 paused sessions, got %d", count)
	}
}

func TestSessionIndex_GetRecent(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	now := time.Now()
	sessions := []*Session{
		{ID: "session-1", Path: "/path/1", Runtime: "claude", Status: StatusActive, UpdatedAt: now},
		{ID: "session-2", Path: "/path/2", Runtime: "opencode", Status: StatusPaused, UpdatedAt: now.Add(-1 * time.Hour)},
		{ID: "session-3", Path: "/path/3", Runtime: "claude", Status: StatusArchived, UpdatedAt: now.Add(-2 * time.Hour)},
	}

	for _, s := range sessions {
		index.Add(s)
	}

	recent := index.GetRecent(2)
	if len(recent) != 2 {
		t.Errorf("Expected 2 recent sessions, got %d", len(recent))
	}

	// Should be ordered by UpdatedAt descending
	if recent[0].ID != "session-1" {
		t.Errorf("Expected first session to be session-1, got %s", recent[0].ID)
	}
}

func TestSessionIndex_GetInactive(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	now := time.Now()
	sessions := []*Session{
		{ID: "session-1", Path: "/path/1", Runtime: "claude", Status: StatusActive, LastMessageAt: now},
		{ID: "session-2", Path: "/path/2", Runtime: "opencode", Status: StatusPaused, LastMessageAt: now.Add(-8 * 24 * time.Hour)},
		{ID: "session-3", Path: "/path/3", Runtime: "claude", Status: StatusArchived, LastMessageAt: now.Add(-10 * 24 * time.Hour)},
	}

	for _, s := range sessions {
		index.Add(s)
	}

	inactive := index.GetInactive(now.Add(-7 * 24 * time.Hour))
	if len(inactive) != 2 {
		t.Errorf("Expected 2 inactive sessions, got %d", len(inactive))
	}
}

func TestSessionIndex_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileSessionStore(tmpDir)
	defer store.Close()

	index := NewSessionIndex(store)

	// Add initial session
	index.Add(&Session{ID: "session-1", Path: "/path/1", Runtime: "claude", Status: StatusActive})

	// Concurrent reads and writes
	done := make(chan bool, 20)

	// Readers
	for i := 0; i < 10; i++ {
		go func() {
			index.Get("session-1")
			index.Count()
			index.GetActive()
			done <- true
		}()
	}

	// Writers
	for i := 0; i < 10; i++ {
		go func(i int) {
			session := &Session{
				ID:      fmt.Sprintf("session-%d", i+2),
				Path:    fmt.Sprintf("/path/%d", i+2),
				Runtime: "claude",
				Status:  StatusActive,
			}
			index.Add(session)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify final count
	if count := index.Count(); count != 11 {
		t.Errorf("Expected 11 sessions, got %d", count)
	}
}
