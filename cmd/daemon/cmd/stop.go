package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Loom Daemon",
	Long:  `Stop the running Loom Daemon gracefully.`,
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	// 连接到控制服务器并发送停止命令
	// 实际实现需要获取 daemon 的控制端口

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post("http://127.0.0.1:0/v1/stop", "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("Daemon stopped successfully")
	} else {
		return fmt.Errorf("failed to stop daemon: %s", resp.Status)
	}

	return nil
}
