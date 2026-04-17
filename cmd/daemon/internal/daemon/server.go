/**
 * Control Server
 * Daemon 的 HTTP 控制接口
 */

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/loom/daemon/internal/runtime"
	"github.com/loom/daemon/internal/session"
)

const (
	Version = "0.1.0"
)

// ControlConfig 控制服务器配置
type ControlConfig struct {
	Host string
	Port int
}

// DefaultControlConfig 返回默认配置
func DefaultControlConfig() ControlConfig {
	return ControlConfig{
		Host: "127.0.0.1",
		Port: 0, // 随机端口
	}
}

// ControlServer Daemon 控制服务器
type ControlServer struct {
	server  *http.Server
	manager *session.Manager
	config  ControlConfig
	addr    string
}

// NewControlServer 创建控制服务器
func NewControlServer(manager *session.Manager, config ControlConfig) *ControlServer {
	s := &ControlServer{
		manager: manager,
		config:  config,
	}

	mux := http.NewServeMux()

	// API 端点
	mux.HandleFunc("/v1/sessions", s.handleSessions)
	mux.HandleFunc("/v1/sessions/", s.handleSession)
	mux.HandleFunc("/v1/chat", s.handleChat)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/status", s.handleStatus)

	// 包装中间件
	handler := s.withLogging(mux)
	handler = s.withRecovery(handler)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s
}

// Start 启动服务器
func (s *ControlServer) Start() error {
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	s.addr = listener.Addr().String()

	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// 记录错误
		}
	}()

	return nil
}

// Stop 停止服务器
func (s *ControlServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Addr 返回服务器地址
func (s *ControlServer) Addr() string {
	return s.addr
}

// 处理函数

func (s *ControlServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listSessions(w, r)
	case http.MethodPost:
		s.createSession(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *ControlServer) handleSession(w http.ResponseWriter, r *http.Request) {
	// 提取 session ID
	path := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	parts := strings.Split(path, "/")
	sessionID := parts[0]

	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "missing session ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getSession(w, r, sessionID)
	case http.MethodDelete:
		s.stopSession(w, r, sessionID)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *ControlServer) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.manager.List()

	var response []SessionInfo
	for _, sess := range sessions {
		response = append(response, SessionInfo{
			ID:           sess.ID,
			RuntimeType:  sess.RuntimeType,
			Status:       string(sess.GetStatus()),
			WorkingDir:   sess.WorkingDir,
			StartedAt:    sess.StartedAt,
			LastActivity: sess.LastActivity,
		})
	}

	respondJSON(w, http.StatusOK, ListSessionsResponse{Sessions: response})
}

func (s *ControlServer) createSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// 验证请求
	if req.RuntimeType == "" {
		req.RuntimeType = "claude"
	}
	if req.WorkingDir == "" {
		req.WorkingDir = "."
	}

	opts := session.SpawnOptions{
		RuntimeType: req.RuntimeType,
		WorkingDir:  req.WorkingDir,
		Resume:      req.Resume,
		EnvVars:     req.EnvVars,
		Metadata:    req.Metadata,
		RuntimeConfig: runtime.Config{
			PermissionMode: req.PermissionMode,
		},
		OnEvent: func(evt session.Event) {
			// 事件处理（可以通过 WebSocket 发送）
			_ = evt
		},
	}

	sess, err := s.manager.Spawn(r.Context(), opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, SessionInfo{
		ID:           sess.ID,
		RuntimeType:  sess.RuntimeType,
		Status:       string(sess.GetStatus()),
		WorkingDir:   sess.WorkingDir,
		StartedAt:    sess.StartedAt,
		LastActivity: sess.LastActivity,
	})
}

func (s *ControlServer) getSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	sess, err := s.manager.Get(sessionID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SessionInfo{
		ID:           sess.ID,
		RuntimeType:  sess.RuntimeType,
		Status:       string(sess.GetStatus()),
		WorkingDir:   sess.WorkingDir,
		StartedAt:    sess.StartedAt,
		LastActivity: sess.LastActivity,
		Metadata:     sess.Metadata,
	})
}

func (s *ControlServer) stopSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req StopSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Force = false
	}

	if err := s.manager.Stop(sessionID, req.Force); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *ControlServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "missing session_id")
		return
	}

	// 设置 SSE 头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 创建流式响应
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// 发送聊天请求
	runtimeReq := runtime.ChatRequest{
		SessionID: req.SessionID,
		Messages:  convertMessages(req.Messages),
		Stream:    true,
	}

	onEvent := func(event runtime.StreamEvent) {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	if err := s.manager.Chat(r.Context(), req.SessionID, runtimeReq, onEvent); err != nil {
		event := runtime.StreamEvent{
			Type:  "error",
			Error: err.Error(),
		}
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

func (s *ControlServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, HealthResponse{
		Status:    "healthy",
		Version:   Version,
		Timestamp: time.Now(),
		Sessions:  len(s.manager.List()),
	})
}

func (s *ControlServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, StatusResponse{
		Version:   Version,
		StartedAt: time.Now(), // TODO: 记录实际启动时间
		Uptime:    "unknown",  // TODO: 计算实际运行时间
		Sessions:  len(s.manager.List()),
	})
}

// 中间件

func (s *ControlServer) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		// 记录请求日志
		_ = duration
	})
}

func (s *ControlServer) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				respondError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// 辅助函数

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}

func convertMessages(msgs []Message) []runtime.Message {
	var result []runtime.Message
	for _, m := range msgs {
		result = append(result, runtime.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return result
}

// 请求/响应类型

type CreateSessionRequest struct {
	RuntimeType    string            `json:"runtime_type"`
	WorkingDir     string            `json:"working_dir"`
	Resume         bool              `json:"resume,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	PermissionMode string            `json:"permission_mode,omitempty"`
}

type StopSessionRequest struct {
	Force bool `json:"force,omitempty"`
}

type ChatRequest struct {
	SessionID string    `json:"session_id"`
	Messages  []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SessionInfo struct {
	ID           string            `json:"id"`
	RuntimeType  string            `json:"runtime_type"`
	Status       string            `json:"status"`
	WorkingDir   string            `json:"working_dir"`
	StartedAt    time.Time         `json:"started_at"`
	LastActivity time.Time         `json:"last_activity"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type ListSessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Sessions  int       `json:"sessions"`
}

type StatusResponse struct {
	Version   string    `json:"version"`
	StartedAt time.Time `json:"started_at"`
	Uptime    string    `json:"uptime"`
	Sessions  int       `json:"sessions"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
