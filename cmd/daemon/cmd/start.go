/**
 * Start command
 * 启动 Daemon
 */

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/loom/daemon/internal/config"
	"github.com/loom/daemon/internal/daemon"
	"github.com/loom/daemon/internal/messaging"
	"github.com/loom/daemon/internal/session"
	"github.com/loom/daemon/internal/ws"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Loom Daemon",
	Long:  `Start the Loom Daemon and connect to the Loom Backend.`,
	RunE:  runStart,
}

var (
	// 命令行参数
	backendURL    string
	deviceID      string
	centrifugoURL string
	listenAddr    string
	runtimeType   string
)

func init() {
	startCmd.Flags().StringVar(&backendURL, "backend", "ws://localhost:8000", "Backend WebSocket URL")
	startCmd.Flags().StringVar(&centrifugoURL, "centrifugo", "ws://localhost:8000", "Centrifugo WebSocket URL")
	startCmd.Flags().StringVar(&deviceID, "device-id", "", "Device ID (auto-generated if empty)")
	startCmd.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:0", "Control server listen address")
	startCmd.Flags().StringVar(&runtimeType, "runtime", "claude", "Default runtime type (claude or opencode)")
}

func runStart(cmd *cobra.Command, args []string) error {
	log.Println("[DAEMON] Starting Loom Daemon...")

	// 加载配置
	cfg := config.DefaultConfig()
	if backendURL != "" {
		cfg.BackendURL = backendURL
	}
	if centrifugoURL != "" {
		cfg.CentrifugoURL = centrifugoURL
	}
	if deviceID != "" {
		cfg.DeviceID = deviceID
	} else {
		// 生成设备 ID
		cfg.DeviceID = generateDeviceID()
	}

	log.Printf("[DAEMON] Device ID: %s", cfg.DeviceID)
	log.Printf("[DAEMON] Backend: %s", cfg.BackendURL)
	log.Printf("[DAEMON] Centrifugo: %s", cfg.CentrifugoURL)

	// 创建 Session Manager
	sessionManager := session.NewManager(session.DefaultManagerConfig())
	defer sessionManager.Close()

	// 创建 Control Server
	controlConfig := daemon.DefaultControlConfig()
	if listenAddr != "" {
		// 解析 listen 地址
		controlConfig.Host = listenAddr
	}

	controlServer := daemon.NewControlServer(sessionManager, controlConfig)
	if err := controlServer.Start(); err != nil {
		return fmt.Errorf("failed to start control server: %w", err)
	}
	defer controlServer.Stop(context.Background())

	log.Printf("[DAEMON] Control server listening on %s", controlServer.Addr())

	// 创建 WebSocket Client
	wsClient := ws.NewClient(cfg.CentrifugoURL, cfg.DeviceID, sessionManager)

	// 创建消息路由器（会自动注册处理器）
	_ = messaging.NewRouter(wsClient, sessionManager)

	// 连接到 Centrifugo
	if err := wsClient.Connect(); err != nil {
		log.Printf("[DAEMON] Warning: Failed to connect to Centrifugo: %v", err)
		log.Println("[DAEMON] Will retry in background...")
		// 继续运行，重连逻辑在 client 内部处理
	} else {
		defer wsClient.Disconnect()
		log.Println("[DAEMON] Connected to Centrifugo")
	}

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("[DAEMON] Daemon is running. Press Ctrl+C to stop.")

	<-sigChan

	log.Println("[DAEMON] Shutting down...")

	// 优雅关闭
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := controlServer.Stop(shutdownCtx); err != nil {
		log.Printf("[DAEMON] Error stopping control server: %v", err)
	}

	return nil
}

func generateDeviceID() string {
	// 简化的设备 ID 生成
	// 实际应该使用机器指纹（MAC 地址 + 主机名等）
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return fmt.Sprintf("%s-%d", hostname, time.Now().Unix())
}
