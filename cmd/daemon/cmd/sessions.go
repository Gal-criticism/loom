package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/loom/daemon/internal/session"
	"github.com/loom/daemon/internal/store"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage sessions",
	Long:  "List, show, resume, archive, and cleanup sessions",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	RunE:  runSessionsList,
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show session details",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsShow,
}

var sessionsResumeCmd = &cobra.Command{
	Use:   "resume [id]",
	Short: "Resume a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsResume,
}

var sessionsArchiveCmd = &cobra.Command{
	Use:   "archive [id]",
	Short: "Archive a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsArchive,
}

var sessionsUnarchiveCmd = &cobra.Command{
	Use:   "unarchive [id]",
	Short: "Unarchive a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsUnarchive,
}

var sessionsCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup old archived sessions",
	RunE:  runSessionsCleanup,
}

var sessionsMessagesCmd = &cobra.Command{
	Use:   "messages [id]",
	Short: "Show session messages",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsMessages,
}

func init() {
	// Add flags to list command
	sessionsListCmd.Flags().String("status", "", "Filter by status (active, paused, archived, error)")
	sessionsListCmd.Flags().String("runtime", "", "Filter by runtime (claude, opencode)")
	sessionsListCmd.Flags().Int("limit", 50, "Limit number of results")
	sessionsListCmd.Flags().Int("offset", 0, "Offset for pagination")

	// Add flags to cleanup command
	sessionsCleanupCmd.Flags().Int("days", 30, "Delete sessions archived more than N days ago")

	// Add flags to messages command
	sessionsMessagesCmd.Flags().Int("limit", 20, "Limit number of messages")
	sessionsMessagesCmd.Flags().Int("offset", 0, "Offset for pagination")

	// Add subcommands
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsResumeCmd)
	sessionsCmd.AddCommand(sessionsArchiveCmd)
	sessionsCmd.AddCommand(sessionsUnarchiveCmd)
	sessionsCmd.AddCommand(sessionsCleanupCmd)
	sessionsCmd.AddCommand(sessionsMessagesCmd)

	// Add to root
	Root.AddCommand(sessionsCmd)
}

func getSessionManager() (*session.Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storePath := filepath.Join(home, ".loom", "sessions")
	mgr, err := session.NewManager(storePath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return mgr, nil
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	mgr, err := getSessionManager()
	if err != nil {
		return err
	}
	defer mgr.Shutdown()

	status, _ := cmd.Flags().GetString("status")
	runtime, _ := cmd.Flags().GetString("runtime")
	limit, _ := cmd.Flags().GetInt("limit")
	offset, _ := cmd.Flags().GetInt("offset")

	opts := store.ListOptions{
		Limit:  limit,
		Offset: offset,
	}

	if status != "" {
		opts.Status = store.SessionStatus(status)
	}
	if runtime != "" {
		opts.Runtime = runtime
	}

	sessions, err := mgr.ListSessions(opts)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	// Print header
	fmt.Printf("%-36s %-10s %-10s %-20s %s\n", "ID", "STATUS", "RUNTIME", "UPDATED", "PATH")
	fmt.Println(strings.Repeat("-", 100))

	for _, s := range sessions {
		updated := s.UpdatedAt.Format("2006-01-02 15:04")
		path := s.Path
		if len(path) > 40 {
			path = "..." + path[len(path)-37:]
		}
		fmt.Printf("%-36s %-10s %-10s %-20s %s\n",
			s.ID,
			s.Status,
			s.Runtime,
			updated,
			path,
		)
	}

	return nil
}

func runSessionsShow(cmd *cobra.Command, args []string) error {
	mgr, err := getSessionManager()
	if err != nil {
		return err
	}
	defer mgr.Shutdown()

	sessionID := args[0]
	s, err := mgr.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	fmt.Printf("Session: %s\n", s.ID)
	fmt.Printf("  Status:   %s\n", s.Status)
	fmt.Printf("  Runtime:  %s\n", s.Runtime)
	fmt.Printf("  Path:     %s\n", s.Path)
	fmt.Printf("  Title:    %s\n", s.Title)
	fmt.Printf("  Created:  %s\n", s.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated:  %s\n", s.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("  Messages: %d\n", s.MessageCount)

	if s.RuntimeSessionID != "" {
		fmt.Printf("  Runtime Session ID: %s\n", s.RuntimeSessionID)
	}

	if s.ErrorMsg != "" {
		fmt.Printf("  Error: %s\n", s.ErrorMsg)
	}

	// Check if active
	activeSessions := mgr.GetActiveSessions()
	for _, active := range activeSessions {
		if active.Session.ID == sessionID {
			fmt.Printf("  Active:   yes\n")
			fmt.Printf("  PID:      %d\n", active.PID)
			fmt.Printf("  Thinking: %v\n", active.Thinking)
			break
		}
	}

	return nil
}

func runSessionsResume(cmd *cobra.Command, args []string) error {
	mgr, err := getSessionManager()
	if err != nil {
		return err
	}
	defer mgr.Shutdown()

	sessionID := args[0]

	if err := mgr.ResumeSession(sessionID); err != nil {
		return fmt.Errorf("failed to resume session: %w", err)
	}

	fmt.Printf("Session %s resumed\n", sessionID)
	return nil
}

func runSessionsArchive(cmd *cobra.Command, args []string) error {
	mgr, err := getSessionManager()
	if err != nil {
		return err
	}
	defer mgr.Shutdown()

	sessionID := args[0]

	if err := mgr.ArchiveSession(sessionID); err != nil {
		return fmt.Errorf("failed to archive session: %w", err)
	}

	fmt.Printf("Session %s archived\n", sessionID)
	return nil
}

func runSessionsUnarchive(cmd *cobra.Command, args []string) error {
	mgr, err := getSessionManager()
	if err != nil {
		return err
	}
	defer mgr.Shutdown()

	sessionID := args[0]

	if err := mgr.UnarchiveSession(sessionID); err != nil {
		return fmt.Errorf("failed to unarchive session: %w", err)
	}

	fmt.Printf("Session %s unarchived\n", sessionID)
	return nil
}

func runSessionsCleanup(cmd *cobra.Command, args []string) error {
	mgr, err := getSessionManager()
	if err != nil {
		return err
	}
	defer mgr.Shutdown()

	days, _ := cmd.Flags().GetInt("days")
	olderThan := time.Duration(days) * 24 * time.Hour

	deleted, err := store.CleanupArchivedSessions(mgr.GetStore(), olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup sessions: %w", err)
	}

	fmt.Printf("Deleted %d archived sessions\n", deleted)
	return nil
}

func runSessionsMessages(cmd *cobra.Command, args []string) error {
	mgr, err := getSessionManager()
	if err != nil {
		return err
	}
	defer mgr.Shutdown()

	sessionID := args[0]
	limit, _ := cmd.Flags().GetInt("limit")
	offset, _ := cmd.Flags().GetInt("offset")

	messages, err := mgr.GetMessages(sessionID, limit, offset)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) == 0 {
		fmt.Println("No messages found")
		return nil
	}

	for _, msg := range messages {
		timestamp := time.Unix(msg.Time, 0).Format("15:04:05")
		fmt.Printf("[%s] %s (%s): ", timestamp, msg.Role, msg.Type)

		content := msg.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		content = strings.ReplaceAll(content, "\n", " ")
		fmt.Println(content)
	}

	return nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// parseSessionID extracts session ID from input (handles full IDs or short prefixes)
func parseSessionID(input string) string {
	// If it's already a valid UUID length, return as-is
	if len(input) == 36 {
		return input
	}
	return input
}
