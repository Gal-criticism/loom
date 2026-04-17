package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loom/daemon/internal/store"
)

func TestManager_CreateSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	opts := CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
		Title:   "Test Session",
	}

	session, err := mgr.CreateSession(opts)
	if err != nil {
		t.Errorf("CreateSession failed: %v", err)
	}

	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}

	if session.Path != opts.Path {
		t.Errorf("Expected path %s, got %s", opts.Path, session.Path)
	}

	if session.Runtime != opts.Runtime {
		t.Errorf("Expected runtime %s, got %s", opts.Runtime, session.Runtime)
	}

	if session.Status != store.StatusActive {
		t.Errorf("Expected status %s, got %s", store.StatusActive, session.Status)
	}
}

func TestManager_StartStopSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	// Create session
	session, err := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Start session
	startOpts := StartOptions{
		PID:              12345,
		RuntimeSessionID: "claude-uuid-123",
	}

	if err := mgr.StartSession(session.ID, startOpts); err != nil {
		t.Errorf("StartSession failed: %v", err)
	}

	// Verify active
	activeSessions := mgr.GetActiveSessions()
	if len(activeSessions) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(activeSessions))
	}

	// Stop session
	if err := mgr.StopSession(session.ID); err != nil {
		t.Errorf("StopSession failed: %v", err)
	}

	// Verify not active
	activeSessions = mgr.GetActiveSessions()
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions, got %d", len(activeSessions))
	}

	// Verify status changed
	updated, _ := mgr.GetSession(session.ID)
	if updated.Status != store.StatusPaused {
		t.Errorf("Expected status %s, got %s", store.StatusPaused, updated.Status)
	}
}

func TestManager_ResumeSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	// Create and start session
	session, _ := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})

	mgr.StartSession(session.ID, StartOptions{PID: 12345})
	mgr.StopSession(session.ID)

	// Resume
	if err := mgr.ResumeSession(session.ID); err != nil {
		t.Errorf("ResumeSession failed: %v", err)
	}

	// Verify active
	activeSessions := mgr.GetActiveSessions()
	if len(activeSessions) != 1 {
		t.Errorf("Expected 1 active session after resume, got %d", len(activeSessions))
	}
}

func TestManager_ArchiveUnarchive(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	// Create session
	session, _ := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})

	// Archive
	if err := mgr.ArchiveSession(session.ID); err != nil {
		t.Errorf("ArchiveSession failed: %v", err)
	}

	// Verify archived
	archived, _ := mgr.GetSession(session.ID)
	if archived.Status != store.StatusArchived {
		t.Errorf("Expected status %s, got %s", store.StatusArchived, archived.Status)
	}

	// Unarchive
	if err := mgr.UnarchiveSession(session.ID); err != nil {
		t.Errorf("UnarchiveSession failed: %v", err)
	}

	// Verify unarchived
	unarchived, _ := mgr.GetSession(session.ID)
	if unarchived.Status != store.StatusActive {
		t.Errorf("Expected status %s, got %s", store.StatusActive, unarchived.Status)
	}
}

func TestManager_ListSessions(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	// Create sessions
	sessions := []CreateOptions{
		{Path: "/path/1", Runtime: "claude", Title: "Session 1"},
		{Path: "/path/2", Runtime: "opencode", Title: "Session 2"},
		{Path: "/path/3", Runtime: "claude", Title: "Session 3"},
	}

	for _, opts := range sessions {
		mgr.CreateSession(opts)
		time.Sleep(10 * time.Millisecond)
	}

	// List all
	all, err := mgr.ListSessions(store.ListOptions{})
	if err != nil {
		t.Errorf("ListSessions failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(all))
	}

	// Filter by runtime
	claudeSessions, _ := mgr.ListSessions(store.ListOptions{Runtime: "claude"})
	if len(claudeSessions) != 2 {
		t.Errorf("Expected 2 claude sessions, got %d", len(claudeSessions))
	}
}

func TestManager_Messages(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	// Create session
	session, _ := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})

	// Append messages
	messages := []*store.StoredMessage{
		{ID: "msg-1", Time: time.Now().Unix(), Role: "user", Type: "text", Content: "Hello"},
		{ID: "msg-2", Time: time.Now().Unix(), Role: "agent", Type: "text", Content: "Hi!"},
	}

	for _, msg := range messages {
		if err := mgr.AppendMessage(session.ID, msg); err != nil {
			t.Errorf("AppendMessage failed: %v", err)
		}
	}

	// Get messages
	retrieved, err := mgr.GetMessages(session.ID, 10, 0)
	if err != nil {
		t.Errorf("GetMessages failed: %v", err)
	}
	if len(retrieved) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(retrieved))
	}
}

func TestManager_UpdateSessionRuntimeID(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	session, _ := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})

	// Update runtime session ID
	if err := mgr.UpdateSessionRuntimeID(session.ID, "new-runtime-id"); err != nil {
		t.Errorf("UpdateSessionRuntimeID failed: %v", err)
	}

	// Verify
	updated, _ := mgr.GetSession(session.ID)
	if updated.RuntimeSessionID != "new-runtime-id" {
		t.Errorf("Expected runtime session ID 'new-runtime-id', got %s", updated.RuntimeSessionID)
	}

	// Get by runtime ID
	byRuntimeID, err := mgr.GetSessionByRuntimeID("new-runtime-id")
	if err != nil {
		t.Errorf("GetSessionByRuntimeID failed: %v", err)
	}
	if byRuntimeID.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, byRuntimeID.ID)
	}
}

func TestManager_SetSessionError(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	session, _ := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})

	// Set error
	if err := mgr.SetSessionError(session.ID, "Something went wrong"); err != nil {
		t.Errorf("SetSessionError failed: %v", err)
	}

	// Verify
	updated, _ := mgr.GetSession(session.ID)
	if updated.Status != store.StatusError {
		t.Errorf("Expected status %s, got %s", store.StatusError, updated.Status)
	}
	if updated.ErrorMsg != "Something went wrong" {
		t.Errorf("Expected error message 'Something went wrong', got %s", updated.ErrorMsg)
	}
}

func TestManager_AutoArchive(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	// Create session with old timestamp by manipulating store directly
	session, _ := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})

	// Update session to be old
	session.UpdatedAt = time.Now().Add(-8 * 24 * time.Hour)
	mgr.GetStore().Update(session)

	// Auto archive sessions older than 7 days
	if err := mgr.AutoArchive(7 * 24 * time.Hour); err != nil {
		t.Errorf("AutoArchive failed: %v", err)
	}

	// Verify archived
	archived, _ := mgr.GetSession(session.ID)
	if archived.Status != store.StatusArchived {
		t.Errorf("Expected status %s, got %s", store.StatusArchived, archived.Status)
	}
}

func TestManager_AutoCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	// Create and archive session with old timestamp
	session, _ := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})

	// Archive and make old
	mgr.ArchiveSession(session.ID)
	session, _ = mgr.GetSession(session.ID)
	session.UpdatedAt = time.Now().Add(-31 * 24 * time.Hour)
	mgr.GetStore().Update(session)

	// Cleanup sessions older than 30 days
	if err := mgr.AutoCleanup(30 * 24 * time.Hour); err != nil {
		t.Errorf("AutoCleanup failed: %v", err)
	}

	// Verify deleted
	_, err = mgr.GetSession(session.ID)
	if err != store.ErrSessionNotFound {
		t.Error("Expected session to be deleted")
	}
}

func TestManager_Shutdown(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Create and start session
	session, _ := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})
	mgr.StartSession(session.ID, StartOptions{PID: 12345})

	// Shutdown
	if err := mgr.Shutdown(); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify no active sessions
	activeSessions := mgr.GetActiveSessions()
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions after shutdown, got %d", len(activeSessions))
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Shutdown()

	// Create session
	session, _ := mgr.CreateSession(CreateOptions{
		Path:    "/home/user/project",
		Runtime: "claude",
	})

	done := make(chan bool, 30)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			mgr.GetSession(session.ID)
			mgr.ListSessions(store.ListOptions{})
			mgr.GetActiveSessions()
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(i int) {
			mgr.CreateSession(CreateOptions{
				Path:    fmt.Sprintf("/path/%d", i),
				Runtime: "claude",
			})
			done <- true
		}(i)
	}

	// Concurrent message appends
	for i := 0; i < 10; i++ {
		go func(i int) {
			msg := &store.StoredMessage{
				ID:      fmt.Sprintf("msg-%d", i),
				Time:    time.Now().Unix(),
				Role:    "user",
				Type:    "text",
				Content: fmt.Sprintf("Message %d", i),
			}
			mgr.AppendMessage(session.ID, msg)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 30; i++ {
		<-done
	}
}
