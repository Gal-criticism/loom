package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/loom/daemon/api"
	"github.com/loom/daemon/config"
	"github.com/loom/daemon/runtime"
	"github.com/loom/daemon/ws"
	"github.com/spf13/cobra"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Loom Daemon",
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config)
	if err != nil {
		log.Printf("Using default config: %v", err)
		cfg = config.DefaultConfig()
	}

	// 创建 Runtime
	rt, err := runtime.NewRuntime(cfg.Runtime)
	if err != nil {
		return err
	}

	log.Printf("Using runtime: %s", rt.Name())

	// 启动 HTTP API 服务器
	apiServer := api.NewServer(rt, cfg.Listen)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	// 连接 Backend WebSocket
	deviceID := getDeviceID()
	client := ws.NewClient(cfg.BackendWS, deviceID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		log.Printf("Failed to connect to backend: %v", err)
	}

	// 处理来自 Backend 的消息
	client.On("chat_request", handleChatRequest(rt))

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	client.Close()

	return nil
}

func handleChatRequest(rt runtime.Runtime) func(json.RawMessage) error {
	return func(payload json.RawMessage) error {
		// 处理来自 Backend 的对话请求
		log.Printf("Received chat request")
		return nil
	}
}

func getDeviceID() string {
	// TODO: 实现设备指纹生成
	return "device-" + "default"
}
