package store

import (
	"fmt"
	"time"
)

// ArchiveOldSessions archives sessions inactive for specified duration
func ArchiveOldSessions(store SessionStore, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	sessions, err := store.List(ListOptions{
		Before: cutoff,
	})
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	for _, session := range sessions {
		// Only archive active or paused sessions
		if session.Status == StatusActive || session.Status == StatusPaused {
			if err := store.Archive(session.ID); err != nil {
				// Log but continue
				continue
			}
		}
	}

	return nil
}

// CleanupArchivedSessions permanently deletes old archived sessions
func CleanupArchivedSessions(store SessionStore, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)

	return store.Cleanup(cutoff)
}

// RunMaintenance runs both archive and cleanup
func RunMaintenance(store SessionStore, archiveAfter, deleteAfter time.Duration) error {
	// First archive old inactive sessions
	if err := ArchiveOldSessions(store, archiveAfter); err != nil {
		return fmt.Errorf("archive failed: %w", err)
	}

	// Then cleanup old archived sessions
	if _, err := CleanupArchivedSessions(store, deleteAfter); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	return nil
}

// MaintenanceScheduler runs maintenance at intervals
type MaintenanceScheduler struct {
	store        SessionStore
	archiveAfter time.Duration
	deleteAfter  time.Duration
	interval     time.Duration
	stopCh       chan struct{}
	running      bool
}

// NewMaintenanceScheduler creates a new scheduler
func NewMaintenanceScheduler(store SessionStore, archiveAfter, deleteAfter, interval time.Duration) *MaintenanceScheduler {
	return &MaintenanceScheduler{
		store:        store,
		archiveAfter: archiveAfter,
		deleteAfter:  deleteAfter,
		interval:     interval,
		stopCh:       make(chan struct{}),
	}
}

// Start starts the maintenance scheduler
func (s *MaintenanceScheduler) Start() {
	if s.running {
		return
	}
	s.running = true

	go s.run()
}

// Stop stops the maintenance scheduler
func (s *MaintenanceScheduler) Stop() {
	if !s.running {
		return
	}
	close(s.stopCh)
	s.running = false
}

func (s *MaintenanceScheduler) run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run immediately on start
	_ = RunMaintenance(s.store, s.archiveAfter, s.deleteAfter)

	for {
		select {
		case <-ticker.C:
			_ = RunMaintenance(s.store, s.archiveAfter, s.deleteAfter)
		case <-s.stopCh:
			return
		}
	}
}

// DefaultMaintenanceInterval is the default interval for maintenance
const DefaultMaintenanceInterval = 1 * time.Hour

// DefaultArchiveAfter is the default duration after which to archive inactive sessions
const DefaultArchiveAfter = 7 * 24 * time.Hour // 7 days

// DefaultDeleteAfter is the default duration after which to delete archived sessions
const DefaultDeleteAfter = 30 * 24 * time.Hour // 30 days
