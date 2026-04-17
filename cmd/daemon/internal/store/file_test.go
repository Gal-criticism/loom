package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileSessionStore_Create(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		ID:      "test-session-1",
		Path:    "/home/user/project",
		Runtime: "claude",
		Status:  StatusActive,
		Title:   "Test Session",
	}

	// Test create
	if err := store.Create(session); err != nil {
		t.Errorf("Create failed: %v", err)
	}

	// Test duplicate create should fail
	if err := store.Create(session); err != ErrSessionExists {
		t.Errorf("Expected ErrSessionExists, got: %v", err)
	}

	// Verify directory was created
	sessionDir := filepath.Join(tmpDir, "session-test-session-1")
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Error("Session directory was not created")
	}

	// Verify metadata file exists
	metadataPath := filepath.Join(sessionDir, "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("Metadata file was not created")
	}

	// Verify messages file exists
	messagesPath := filepath.Join(sessionDir, "messages.jsonl")
	if _, err := os.Stat(messagesPath); os.IsNotExist(err) {
		t.Error("Messages file was not created")
	}
}

func TestFileSessionStore_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a session
	session := &Session{
		ID:      "test-session-2",
		Path:    "/home/user/project2",
		Runtime: "opencode",
		Status:  StatusActive,
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test get existing session
	retrieved, err := store.Get("test-session-2")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if retrieved.ID != session.ID {
		t.Errorf("Expected ID %s, got %s", session.ID, retrieved.ID)
	}
	if retrieved.Path != session.Path {
		t.Errorf("Expected Path %s, got %s", session.Path, retrieved.Path)
	}

	// Test get non-existent session
	_, err = store.Get("non-existent")
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got: %v", err)
	}
}

func TestFileSessionStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a session
	session := &Session{
		ID:      "test-session-3",
		Path:    "/home/user/project3",
		Runtime: "claude",
		Status:  StatusActive,
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Update session
	session.Status = StatusPaused
	session.Title = "Updated Title"
	if err := store.Update(session); err != nil {
		t.Errorf("Update failed: %v", err)
	}

	// Verify update
	retrieved, err := store.Get("test-session-3")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if retrieved.Status != StatusPaused {
		t.Errorf("Expected status %s, got %s", StatusPaused, retrieved.Status)
	}
	if retrieved.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %s", retrieved.Title)
	}
}

func TestFileSessionStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a session
	session := &Session{
		ID:      "test-session-4",
		Path:    "/home/user/project4",
		Runtime: "claude",
		Status:  StatusActive,
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Delete session
	if err := store.Delete("test-session-4"); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify deletion
	_, err = store.Get("test-session-4")
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound after delete, got: %v", err)
	}

	// Verify directory was removed
	sessionDir := filepath.Join(tmpDir, "session-test-session-4")
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("Session directory should have been removed")
	}
}

func TestFileSessionStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create multiple sessions
	sessions := []*Session{
		{ID: "session-1", Path: "/path/1", Runtime: "claude", Status: StatusActive},
		{ID: "session-2", Path: "/path/2", Runtime: "opencode", Status: StatusPaused},
		{ID: "session-3", Path: "/path/3", Runtime: "claude", Status: StatusArchived},
	}

	for _, s := range sessions {
		if err := store.Create(s); err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Test list all
	all, err := store.List(ListOptions{})
	if err != nil {
		t.Errorf("List failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(all))
	}

	// Test filter by status
	active, err := store.List(ListOptions{Status: StatusActive})
	if err != nil {
		t.Errorf("List with status filter failed: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(active))
	}

	// Test filter by runtime
	claudeSessions, err := store.List(ListOptions{Runtime: "claude"})
	if err != nil {
		t.Errorf("List with runtime filter failed: %v", err)
	}
	if len(claudeSessions) != 2 {
		t.Errorf("Expected 2 claude sessions, got %d", len(claudeSessions))
	}

	// Test limit
	limited, err := store.List(ListOptions{Limit: 2})
	if err != nil {
		t.Errorf("List with limit failed: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 sessions with limit, got %d", len(limited))
	}
}

func TestFileSessionStore_Archive(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		ID:      "test-archive",
		Path:    "/home/user/project",
		Runtime: "claude",
		Status:  StatusActive,
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Archive
	if err := store.Archive("test-archive"); err != nil {
		t.Errorf("Archive failed: %v", err)
	}

	// Verify
	retrieved, _ := store.Get("test-archive")
	if retrieved.Status != StatusArchived {
		t.Errorf("Expected status %s, got %s", StatusArchived, retrieved.Status)
	}

	// Unarchive
	if err := store.Unarchive("test-archive"); err != nil {
		t.Errorf("Unarchive failed: %v", err)
	}

	// Verify
	retrieved, _ = store.Get("test-archive")
	if retrieved.Status != StatusActive {
		t.Errorf("Expected status %s, got %s", StatusActive, retrieved.Status)
	}
}

func TestFileSessionStore_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create an archived session
	session := &Session{
		ID:      "test-cleanup",
		Path:    "/home/user/project",
		Runtime: "claude",
		Status:  StatusArchived,
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Update to make it old
	session.UpdatedAt = time.Now().Add(-31 * 24 * time.Hour)
	store.Update(session)

	// Cleanup sessions older than 30 days
	deleted, err := store.Cleanup(time.Now().Add(-30 * 24 * time.Hour))
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("Expected 1 deleted session, got %d", deleted)
	}

	// Verify deletion
	_, err = store.Get("test-cleanup")
	if err != ErrSessionNotFound {
		t.Errorf("Expected session to be deleted")
	}
}

func TestFileSessionStore_Messages(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		ID:      "test-messages",
		Path:    "/home/user/project",
		Runtime: "claude",
		Status:  StatusActive,
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Append messages
	messages := []*StoredMessage{
		{ID: "msg-1", Time: time.Now().Unix(), Role: "user", Type: "text", Content: "Hello"},
		{ID: "msg-2", Time: time.Now().Unix(), Role: "agent", Type: "text", Content: "Hi there"},
		{ID: "msg-3", Time: time.Now().Unix(), Role: "user", Type: "text", Content: "How are you?"},
	}

	for _, msg := range messages {
		if err := store.AppendMessage("test-messages", msg); err != nil {
			t.Errorf("AppendMessage failed: %v", err)
		}
	}

	// Retrieve messages
	retrieved, err := store.GetMessages("test-messages", 10, 0)
	if err != nil {
		t.Errorf("GetMessages failed: %v", err)
	}
	if len(retrieved) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(retrieved))
	}

	// Test limit
	limited, err := store.GetMessages("test-messages", 2, 0)
	if err != nil {
		t.Errorf("GetMessages with limit failed: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 messages with limit, got %d", len(limited))
	}

	// Test offset
	offset, err := store.GetMessages("test-messages", 10, 1)
	if err != nil {
		t.Errorf("GetMessages with offset failed: %v", err)
	}
	if len(offset) != 2 {
		t.Errorf("Expected 2 messages with offset, got %d", len(offset))
	}

	// Verify session message count was updated
	updated, _ := store.Get("test-messages")
	if updated.MessageCount != 3 {
		t.Errorf("Expected message count 3, got %d", updated.MessageCount)
	}
}

func TestFileSessionStore_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test atomic write
	path := filepath.Join(tmpDir, "test-file")
	data := []byte("test data content")

	fs := store.(*FileSessionStore)
	if err := fs.writeAtomic(path, data); err != nil {
		t.Errorf("writeAtomic failed: %v", err)
	}

	// Verify file exists and has correct content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("Expected %s, got %s", data, content)
	}

	// Verify permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestFileSessionStore_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a session
	session := &Session{
		ID:      "concurrent-test",
		Path:    "/home/user/project",
		Runtime: "claude",
		Status:  StatusActive,
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := store.Get("concurrent-test")
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestFileSessionStore_GetByRuntimeSessionID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		ID:               "test-runtime-id",
		Path:             "/home/user/project",
		Runtime:          "claude",
		Status:           StatusActive,
		RuntimeSessionID: "claude-uuid-12345",
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Get by runtime session ID
	retrieved, err := store.GetByRuntimeSessionID("claude-uuid-12345")
	if err != nil {
		t.Errorf("GetByRuntimeSessionID failed: %v", err)
	}
	if retrieved.ID != "test-runtime-id" {
		t.Errorf("Expected ID test-runtime-id, got %s", retrieved.ID)
	}

	// Non-existent runtime ID
	_, err = store.GetByRuntimeSessionID("non-existent-uuid")
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got: %v", err)
	}
}

func TestFileSessionStore_Closed(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	store.Close()

	// Operations on closed store should fail
	session := &Session{
		ID:      "closed-test",
		Path:    "/home/user/project",
		Runtime: "claude",
		Status:  StatusActive,
	}

	if err := store.Create(session); err != ErrStoreClosed {
		t.Errorf("Expected ErrStoreClosed, got: %v", err)
	}

	if _, err := store.Get("any-id"); err != ErrStoreClosed {
		t.Errorf("Expected ErrStoreClosed, got: %v", err)
	}
}
