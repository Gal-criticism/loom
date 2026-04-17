package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Loom Daemon status",
	Long:  `Show the current status of the Loom Daemon including active sessions.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	// 获取 daemon 状态
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:0/health")
	if err != nil {
		return fmt.Errorf("daemon is not running: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon health check failed: %s", resp.Status)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("Daemon Status: %s\n", health["status"])
	fmt.Printf("Version: %s\n", health["version"])
	fmt.Printf("Active Sessions: %v\n", health["sessions"])

	return nil
}
