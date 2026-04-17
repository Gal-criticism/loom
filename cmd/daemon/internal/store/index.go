package store

import (
	"sort"
	"sync"
	"time"
)

// SessionIndex provides in-memory indexing for fast lookups
type SessionIndex struct {
	sessions    map[string]*Session
	byRuntimeID map[string]string // runtimeSessionID -> sessionID
	mu          sync.RWMutex
	store       SessionStore
}

// NewSessionIndex creates a new index
func NewSessionIndex(store SessionStore) *SessionIndex {
	return &SessionIndex{
		sessions:    make(map[string]*Session),
		byRuntimeID: make(map[string]string),
		store:       store,
	}
}

// Load loads all sessions from store into memory
func (idx *SessionIndex) Load() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	sessions, err := idx.store.List(ListOptions{})
	if err != nil {
		return err
	}

	// Clear existing index
	idx.sessions = make(map[string]*Session)
	idx.byRuntimeID = make(map[string]string)

	// Index all sessions
	for _, session := range sessions {
		idx.sessions[session.ID] = session
		if session.RuntimeSessionID != "" {
			idx.byRuntimeID[session.RuntimeSessionID] = session.ID
		}
	}

	return nil
}

// Add adds a session to index
func (idx *SessionIndex) Add(session *Session) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.sessions[session.ID] = session
	if session.RuntimeSessionID != "" {
		idx.byRuntimeID[session.RuntimeSessionID] = session.ID
	}
}

// Update updates a session in index
func (idx *SessionIndex) Update(session *Session) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove old runtime ID mapping if changed
	if existing, ok := idx.sessions[session.ID]; ok {
		if existing.RuntimeSessionID != "" &&
			existing.RuntimeSessionID != session.RuntimeSessionID {
			delete(idx.byRuntimeID, existing.RuntimeSessionID)
		}
	}

	idx.sessions[session.ID] = session
	if session.RuntimeSessionID != "" {
		idx.byRuntimeID[session.RuntimeSessionID] = session.ID
	}
}

// Remove removes a session from index
func (idx *SessionIndex) Remove(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if session, ok := idx.sessions[id]; ok {
		if session.RuntimeSessionID != "" {
			delete(idx.byRuntimeID, session.RuntimeSessionID)
		}
		delete(idx.sessions, id)
	}
}

// Get retrieves from index (fast)
func (idx *SessionIndex) Get(id string) (*Session, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	session, ok := idx.sessions[id]
	return session, ok
}

// GetByRuntimeID finds by runtime session ID
func (idx *SessionIndex) GetByRuntimeID(runtimeID string) (*Session, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if sessionID, ok := idx.byRuntimeID[runtimeID]; ok {
		if session, ok := idx.sessions[sessionID]; ok {
			return session, true
		}
	}
	return nil, false
}

// List returns filtered sessions
func (idx *SessionIndex) List(opts ListOptions) []*Session {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var sessions []*Session
	for _, session := range idx.sessions {
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
			return []*Session{}
		}
		sessions = sessions[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(sessions) {
		sessions = sessions[:opts.Limit]
	}

	return sessions
}

// GetActive returns all active sessions
func (idx *SessionIndex) GetActive() []*Session {
	return idx.List(ListOptions{Status: StatusActive})
}

// GetByPath returns sessions by path
func (idx *SessionIndex) GetByPath(path string) []*Session {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var sessions []*Session
	for _, session := range idx.sessions {
		if session.Path == path {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// GetByRuntime returns sessions by runtime
func (idx *SessionIndex) GetByRuntime(runtime string) []*Session {
	return idx.List(ListOptions{Runtime: runtime})
}

// Count returns total number of sessions
func (idx *SessionIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.sessions)
}

// CountByStatus returns count of sessions by status
func (idx *SessionIndex) CountByStatus(status SessionStatus) int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	count := 0
	for _, session := range idx.sessions {
		if session.Status == status {
			count++
		}
	}
	return count
}

// GetRecent returns recently updated sessions
func (idx *SessionIndex) GetRecent(limit int) []*Session {
	return idx.List(ListOptions{
		Limit: limit,
	})
}

// GetInactive returns sessions inactive since the given time
func (idx *SessionIndex) GetInactive(since time.Time) []*Session {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var sessions []*Session
	for _, session := range idx.sessions {
		if session.LastMessageAt.Before(since) && session.UpdatedAt.Before(since) {
			sessions = append(sessions, session)
		}
	}
	return sessions
}
