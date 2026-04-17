/**
 * Message Router
 * 处理从 Backend 接收的消息并路由到相应的处理器
 */

package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/loom/daemon/internal/runtime"
	"github.com/loom/daemon/internal/session"
	"github.com/loom/daemon/internal/ws"
)

// Router 消息路由器
type Router struct {
	wsClient       *ws.Client
	sessionManager *session.Manager
	formatter      *Formatter
}

// NewRouter 创建消息路由器
func NewRouter(wsClient *ws.Client, sessionManager *session.Manager) *Router {
	r := &Router{
		wsClient:       wsClient,
		sessionManager: sessionManager,
		formatter:      NewFormatter(),
	}

	// 注册消息处理器
	r.registerHandlers()

	return r
}

// registerHandlers 注册所有消息处理器
func (r *Router) registerHandlers() {
	// 聊天请求
	r.wsClient.RegisterHandler("chat:message", r.handleChatMessage)

	// 会话管理
	r.wsClient.RegisterHandler("session:create", r.handleSessionCreate)
	r.wsClient.RegisterHandler("session:stop", r.handleSessionStop)

	// 工具调用
	r.wsClient.RegisterHandler("tool:execute", r.handleToolExecute)

	// 系统消息
	r.wsClient.RegisterHandler("system:ping", r.handlePing)
}

// handleChatMessage 处理聊天消息
func (r *Router) handleChatMessage(payload []byte) error {
	var req ChatMessageRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("failed to unmarshal chat message: %w", err)
	}

	log.Printf("[ROUTER] Received chat message for session %s", req.SessionID)

	// 转换消息格式
	var messages []runtime.Message
	for _, m := range req.Messages {
		messages = append(messages, runtime.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// 使用 WebSocket 客户端处理
	if err := r.wsClient.HandleChatRequest(req.SessionID, messages); err != nil {
		return fmt.Errorf("failed to handle chat request: %w", err)
	}

	return nil
}

// handleSessionCreate 处理创建会话请求
func (r *Router) handleSessionCreate(payload []byte) error {
	var req SessionCreateRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("failed to unmarshal session create request: %w", err)
	}

	log.Printf("[ROUTER] Creating session with runtime %s", req.RuntimeType)

	// 创建会话
	sess, err := r.sessionManager.Spawn(context.Background(), session.SpawnOptions{
		RuntimeType: req.RuntimeType,
		WorkingDir:  req.WorkingDir,
		EnvVars:     req.EnvVars,
		Metadata:    req.Metadata,
		OnEvent: func(evt session.Event) {
			// 转发事件到 Backend
			r.forwardSessionEvent(evt)
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// 发送成功响应
	response := map[string]interface{}{
		"type":       "session:created",
		"session_id": sess.ID,
		"status":     sess.Status,
		"timestamp":  time.Now().Unix(),
	}

	channel := fmt.Sprintf("user:%s", r.wsClient.GetDeviceID())
	if err := r.wsClient.SendToBackend(channel, response); err != nil {
		log.Printf("[ROUTER] Failed to send session created response: %v", err)
	}

	return nil
}

// handleSessionStop 处理停止会话请求
func (r *Router) handleSessionStop(payload []byte) error {
	var req SessionStopRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("failed to unmarshal session stop request: %w", err)
	}

	log.Printf("[ROUTER] Stopping session %s", req.SessionID)

	if err := r.sessionManager.Stop(req.SessionID, req.Force); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}

	// 发送成功响应
	response := map[string]interface{}{
		"type":       "session:stopped",
		"session_id": req.SessionID,
		"timestamp":  time.Now().Unix(),
	}

	channel := fmt.Sprintf("user:%s", r.wsClient.GetDeviceID())
	if err := r.wsClient.SendToBackend(channel, response); err != nil {
		log.Printf("[ROUTER] Failed to send session stopped response: %v", err)
	}

	return nil
}

// handleToolExecute 处理工具执行请求
func (r *Router) handleToolExecute(payload []byte) error {
	var req ToolExecuteRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("failed to unmarshal tool execute request: %w", err)
	}

	log.Printf("[ROUTER] Executing tool %s for session %s", req.ToolName, req.SessionID)

	// 获取会话
	sess, err := r.sessionManager.Get(req.SessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// 获取 Runtime 并执行工具
	rt := sess.GetRuntime()
	if rt == nil {
		return fmt.Errorf("session has no runtime")
	}

	// 异步执行工具
	go func() {
		output, err := rt.ExecuteTool(context.Background(), req.ToolName, req.Input)

		response := map[string]interface{}{
			"type":       "tool:result",
			"session_id": req.SessionID,
			"tool_name":  req.ToolName,
			"timestamp":  time.Now().Unix(),
		}

		if err != nil {
			response["error"] = err.Error()
		} else {
			response["output"] = output
		}

		channel := fmt.Sprintf("user:%s", r.wsClient.GetDeviceID())
		if err := r.wsClient.SendToBackend(channel, response); err != nil {
			log.Printf("[ROUTER] Failed to send tool result: %v", err)
		}
	}()

	return nil
}

// handlePing 处理心跳 ping
func (r *Router) handlePing(payload []byte) error {
	// 发送 pong 响应
	response := map[string]interface{}{
		"type":      "pong",
		"timestamp": time.Now().Unix(),
	}

	channel := fmt.Sprintf("daemon:%s", r.wsClient.GetDeviceID())
	return r.wsClient.SendToBackend(channel, response)
}

// forwardSessionEvent 转发会话事件到 Backend
func (r *Router) forwardSessionEvent(evt session.Event) {
	// 使用 Formatter 转换事件格式
	data, err := r.formatter.ToBackendMessage(evt.SessionID, runtime.StreamEvent{
		Type: evt.Type,
	})
	if err != nil {
		log.Printf("[ROUTER] Failed to format event: %v", err)
		return
	}

	// 解析回 map 以便添加额外字段
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("[ROUTER] Failed to unmarshal formatted event: %v", err)
		return
	}

	// 添加事件数据
	msg["data"] = evt.Data

	channel := fmt.Sprintf("user:%s", r.wsClient.GetDeviceID())
	if err := r.wsClient.SendToBackend(channel, msg); err != nil {
		log.Printf("[ROUTER] Failed to forward event: %v", err)
	}
}

// 请求/响应类型

type ChatMessageRequest struct {
	SessionID string    `json:"session_id"`
	Messages  []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SessionCreateRequest struct {
	RuntimeType string            `json:"runtime_type"`
	WorkingDir  string            `json:"working_dir"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type SessionStopRequest struct {
	SessionID string `json:"session_id"`
	Force     bool   `json:"force,omitempty"`
}

type ToolExecuteRequest struct {
	SessionID string                 `json:"session_id"`
	ToolName  string                 `json:"tool_name"`
	Input     map[string]interface{} `json:"input"`
}
