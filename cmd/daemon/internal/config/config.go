package config

import (
	"os"
)

// Config 是 Daemon 的配置
type Config struct {
	// 设备 ID
	DeviceID string

	// Backend WebSocket URL
	BackendURL string

	// Centrifugo WebSocket URL
	CentrifugoURL string

	// 默认运行时类型
	DefaultRuntime string

	// 控制服务器地址
	ControlAddr string
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		DeviceID:       getEnv("LOOM_DEVICE_ID", ""),
		BackendURL:     getEnv("LOOM_BACKEND_URL", "ws://localhost:8000"),
		CentrifugoURL:  getEnv("LOOM_CENTRIFUGO_URL", "ws://localhost:8000"),
		DefaultRuntime: getEnv("LOOM_RUNTIME", "claude"),
		ControlAddr:    getEnv("LOOM_CONTROL_ADDR", "127.0.0.1:0"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
