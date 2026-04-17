/**
 * Integration Tests
 * Daemon 端到端集成测试
 */

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/loom/daemon/internal/daemon"
	"github.com/loom/daemon/internal/messaging"
	"github.com/loom/daemon/internal/session"
	"github.com/loom/daemon/internal/ws"
)

// TestDaemonStartup 测试 Daemon 启动
func TestDaemonStartup(t *testing.T) {
	// 创建 Session Manager
	sessionManager := session.NewManager(session.DefaultManagerConfig())
	defer sessionManager.Close()

	// 创建 Control Server
	controlConfig := daemon.DefaultControlConfig()
	controlConfig.Host = "127.0.0.1:0"

	controlServer := daemon.NewControlServer(sessionManager, controlConfig)
	if err := controlServer.Start(); err != nil {
		t.Fatalf("Failed to start control server: %v", err)
	}
	defer controlServer.Stop(context.Background())

	addr := controlServer.Addr()
	if addr == "" {
		t.Fatal("Control server address is empty")
	}

	t.Logf("Control server started on %s", addr)

	// 测试健康检查端点
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if health["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", health["status"])
	}
}

// TestSessionLifecycle 测试会话生命周期
func TestSessionLifecycle(t *testing.T) {
	sessionManager := session.NewManager(session.DefaultManagerConfig())
	defer sessionManager.Close()

	// 创建会话
	sess, err := sessionManager.Spawn(context.Background(), session.SpawnOptions{
		RuntimeType: "claude",
		WorkingDir:  ".",
		OnEvent: func(evt session.Event) {
			t.Logf("Event: %s", evt.Type)
		},
	})
	if err != nil {
		t.Fatalf("Failed to spawn session: %v", err)
	}

	if sess.ID == "" {
		t.Error("Session ID is empty")
	}

	if sess.Status != session.StatusRunning {
		t.Errorf("Expected status 'running', got %s", sess.Status)
	}

	// 获取会话
	retrieved, err := sessionManager.Get(sess.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved.ID != sess.ID {
		t.Error("Retrieved session ID doesn't match")
	}

	// 列出会话
	sessions := sessionManager.List()
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}

	// 停止会话
	if err := sessionManager.Stop(sess.ID, false); err != nil {
		t.Fatalf("Failed to stop session: %v", err)
	}

	// 验证会话已停止
	_, err = sessionManager.Get(sess.ID)
	if err == nil {
		t.Error("Expected session to be removed")
	}
}

// TestControlServerAPI 测试控制服务器 API
func TestControlServerAPI(t *testing.T) {
	sessionManager := session.NewManager(session.DefaultManagerConfig())
	defer sessionManager.Close()

	controlConfig := daemon.DefaultControlConfig()
	controlConfig.Host = "127.0.0.1:0"

	controlServer := daemon.NewControlServer(sessionManager, controlConfig)
	if err := controlServer.Start(); err != nil {
		t.Fatalf("Failed to start control server: %v", err)
	}
	defer controlServer.Stop(context.Background())

	addr := controlServer.Addr()
	baseURL := fmt.Sprintf("http://%s", addr)

	// 测试状态端点
	t.Run("Status", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/v1/status", baseURL))
		if err != nil {
			t.Fatalf("Status request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var status map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			t.Fatalf("Failed to decode status: %v", err)
		}

		if status["version"] == "" {
			t.Error("Version is empty")
		}
	})

	// 测试创建会话
	t.Run("CreateSession", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"runtime_type": "claude",
			"working_dir":  ".",
		}
		body, _ := json.Marshal(reqBody)

		resp, err := http.Post(
			fmt.Sprintf("%s/v1/sessions", baseURL),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("Create session failed: %v", err)
		}
		defer resp.Body.Close()

		// 可能会跳过（如果没有安装 claude）
		if resp.StatusCode == http.StatusServiceUnavailable {
			t.Skip("Runtime not available")
		}

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var session map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
			t.Fatalf("Failed to decode session: %v", err)
		}

		if session["id"] == "" {
			t.Error("Session ID is empty")
		}
	})

	// 测试列出会话
	t.Run("ListSessions", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/v1/sessions", baseURL))
		if err != nil {
			t.Fatalf("List sessions failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode sessions: %v", err)
		}

		sessions, ok := result["sessions"].([]interface{})
		if !ok {
			t.Fatal("Sessions is not an array")
		}

		t.Logf("Found %d sessions", len(sessions))
	})
}

// TestMessageFormatter 测试消息格式化
func TestMessageFormatter(t *testing.T) {
	formatter := messaging.NewFormatter()

	// 测试各种事件类型的转换
	testCases := []struct {
		name      string
		eventType string
		event     runtime.StreamEvent
	}{
		{
			name:      "TextEvent",
			eventType: "text",
			event: runtime.StreamEvent{
				Type: "text",
				Text: "Hello world",
			},
		},
		{
			name:      "ThinkingEvent",
			eventType: "thinking",
			event: runtime.StreamEvent{
				Type:     "thinking",
				Thinking: true,
			},
		},
		{
			name:      "ToolCallEvent",
			eventType: "tool_call",
			event: runtime.StreamEvent{
				Type: "tool_call",
				ToolCall: &runtime.ToolCall{
					Name:      "Bash",
					Arguments: map[string]interface{}{"command": "ls"},
				},
			},
		},
		{
			name:      "DoneEvent",
			eventType: "done",
			event: runtime.StreamEvent{
				Type: "done",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := formatter.ToBackendMessage("test-session", tc.event)
			if err != nil {
				t.Fatalf("Failed to format event: %v", err)
			}

			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				t.Fatalf("Failed to unmarshal message: %v", err)
			}

			if msg["session_id"] != "test-session" {
				t.Errorf("Expected session_id 'test-session', got %v", msg["session_id"])
			}

			if msg["type"] == "" {
				t.Error("Message type is empty")
			}
		})
	}
}

// TestWebSocketClient 测试 WebSocket 客户端
func TestWebSocketClient(t *testing.T) {
	sessionManager := session.NewManager(session.DefaultManagerConfig())
	defer sessionManager.Close()

	// 创建 WebSocket 客户端（不实际连接）
	client := ws.NewClient("ws://localhost:8000", "test-device", sessionManager)

	// 测试注册处理器
	handlerCalled := false
	client.RegisterHandler("test:message", func(payload []byte) error {
		handlerCalled = true
		return nil
	})

	// 测试统计信息
	stats := client.Stats()
	if stats["device_id"] != "test-device" {
		t.Errorf("Expected device_id 'test-device', got %v", stats["device_id"])
	}

	// 由于我们没有实际连接，测试基本功能
	if client.IsConnected() {
		t.Error("Client should not be connected initially")
	}
}

// BenchmarkSessionSpawn 基准测试：创建会话
func BenchmarkSessionSpawn(b *testing.B) {
	sessionManager := session.NewManager(session.DefaultManagerConfig())
	defer sessionManager.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sessionManager.Spawn(ctx, session.SpawnOptions{
			RuntimeType: "claude",
			WorkingDir:  ".",
		})
		if err != nil {
			// 可能会因为运行时不可用而失败，这是预期的
			continue
		}
	}
}

// 导入 runtime 包用于测试
var _ = runtime.StreamEvent{}
