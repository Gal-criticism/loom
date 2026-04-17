/**
 * Session Manager
 * 管理 AI 会话的生命周期
 */

package session

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/loom/daemon/internal/runtime"
)

// Status 会话状态
type Status string

const (
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusThinking Status = "thinking"
	StatusToolCall Status = "tool_call"
	StatusStopping Status = "stopping"
	StatusStopped  Status = "stopped"
	StatusError    Status = "error"
)

// Event 会话事件
type Event struct {
	SessionID string
	Type      string // started | message | tool_call | thinking | stopped | error
	Data      interface{}
	Timestamp time.Time
}

// Session 表示一个运行时会话
type Session struct {
	ID           string
	PID          int
	RuntimeType  string
	WorkingDir   string
	Status       Status
	StartedAt    time.Time
	LastActivity time.Time
	Metadata     map[string]string

	// 运行时实例
	runtime runtime.Runtime

	// 进程引用
	process *os.Process

	// 取消函数
	cancel context.CancelFunc

	// 事件回调
	onEvent func(Event)

	// 互斥锁保护状态变更
	mu sync.RWMutex
}

// SetStatus 设置会话状态（线程安全）
func (s *Session) SetStatus(status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.LastActivity = time.Now()
}

// GetStatus 获取会话状态（线程安全）
func (s *Session) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

// GetRuntime 获取 Runtime 实例（线程安全）
func (s *Session) GetRuntime() runtime.Runtime {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtime
}

// emit 发送事件
func (s *Session) emit(event Event) {
	if s.onEvent != nil {
		s.onEvent(event)
	}
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	// 心跳间隔
	HeartbeatInterval time.Duration

	// 会话超时
	SessionTimeout time.Duration

	// 最大会话数
	MaxSessions int
}

// DefaultManagerConfig 返回默认配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		HeartbeatInterval: 30 * time.Second,
		SessionTimeout:    10 * time.Minute,
		MaxSessions:       10,
	}
}

// Manager 会话管理器
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	config   ManagerConfig

	// 后台任务控制
	heartbeatStop chan struct{}
	wg            sync.WaitGroup
}

// NewManager 创建会话管理器
func NewManager(config ManagerConfig) *Manager {
	m := &Manager{
		sessions:      make(map[string]*Session),
		config:        config,
		heartbeatStop: make(chan struct{}),
	}

	// 启动心跳检测
	m.wg.Add(1)
	go m.heartbeatLoop()

	return m
}

// SpawnOptions 启动会话选项
type SpawnOptions struct {
	RuntimeType string
	WorkingDir  string
	Resume      bool
	EnvVars     map[string]string
	Metadata    map[string]string
	OnEvent     func(Event)

	// Runtime 配置
	RuntimeConfig runtime.Config
}

// Spawn 启动新会话
func (m *Manager) Spawn(ctx context.Context, opts SpawnOptions) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查最大会话数
	if len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("max sessions reached: %d", m.config.MaxSessions)
	}

	// 创建 Runtime 实例
	rtConfig := opts.RuntimeConfig
	rtConfig.WorkingDir = opts.WorkingDir
	rtConfig.EnvVars = opts.EnvVars

	rt, err := runtime.Factory(opts.RuntimeType, rtConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime: %w", err)
	}

	// 创建会话
	session := &Session{
		ID:           generateSessionID(),
		RuntimeType:  opts.RuntimeType,
		WorkingDir:   opts.WorkingDir,
		Status:       StatusStarting,
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
		Metadata:     opts.Metadata,
		runtime:      rt,
		onEvent:      opts.OnEvent,
	}

	// 创建带取消的上下文
	ctx, cancel := context.WithCancel(ctx)
	session.cancel = cancel

	// 启动会话进程
	if err := m.startSession(ctx, session, opts); err != nil {
		cancel()
		return nil, err
	}

	m.sessions[session.ID] = session

	// 发送 started 事件
	session.emit(Event{
		SessionID: session.ID,
		Type:      "started",
		Timestamp: time.Now(),
	})

	return session, nil
}

// startSession 启动会话进程
func (m *Manager) startSession(ctx context.Context, session *Session, opts SpawnOptions) error {
	// 这里我们实际上不需要启动一个长期运行的进程
	// 因为每次 Chat 调用都会启动一个 Claude 进程
	// 但为了管理方便，我们创建一个占位进程或记录

	session.SetStatus(StatusRunning)
	session.LastActivity = time.Now()

	return nil
}

// Chat 发送聊天消息到会话
func (m *Manager) Chat(ctx context.Context, sessionID string, req runtime.ChatRequest, onEvent func(runtime.StreamEvent)) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.runtime == nil {
		return fmt.Errorf("session runtime not initialized")
	}

	// 更新状态为思考中
	session.SetStatus(StatusThinking)
	session.emit(Event{
		SessionID: sessionID,
		Type:      "thinking",
		Data:      map[string]bool{"thinking": true},
		Timestamp: time.Now(),
	})

	// 包装回调以跟踪状态
	wrappedOnEvent := func(event runtime.StreamEvent) {
		// 更新最后活动时间
		session.LastActivity = time.Now()

		// 根据事件类型更新状态
		switch event.Type {
		case "tool_call":
			session.SetStatus(StatusToolCall)
			session.emit(Event{
				SessionID: sessionID,
				Type:      "tool_call",
				Data:      event.ToolCall,
				Timestamp: time.Now(),
			})

		case "done":
			session.SetStatus(StatusRunning)
			session.emit(Event{
				SessionID: sessionID,
				Type:      "message",
				Data:      "completed",
				Timestamp: time.Now(),
			})

		case "error":
			session.SetStatus(StatusError)
			session.emit(Event{
				SessionID: sessionID,
				Type:      "error",
				Data:      event.Error,
				Timestamp: time.Now(),
			})
		}

		// 调用原始回调
		onEvent(event)
	}

	// 调用 Runtime.Chat
	err := session.runtime.Chat(ctx, req, wrappedOnEvent)

	// 恢复状态
	if session.GetStatus() != StatusError {
		session.SetStatus(StatusRunning)
	}

	return err
}

// Stop 停止会话
func (m *Manager) Stop(sessionID string, force bool) error {
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.SetStatus(StatusStopping)

	// 取消上下文
	if session.cancel != nil {
		session.cancel()
	}

	// 终止进程
	if session.process != nil {
		// 发送 SIGTERM
		if err := session.process.Signal(syscall.SIGTERM); err != nil {
			// 进程可能已经退出
		}

		// 等待进程退出
		done := make(chan error, 1)
		go func() {
			_, err := session.process.Wait()
			done <- err
		}()

		select {
		case <-done:
			// 正常退出
		case <-time.After(5 * time.Second):
			// 超时，强制终止
			if force {
				if err := session.process.Kill(); err != nil {
					return fmt.Errorf("failed to kill process: %w", err)
				}
			}
		}
	}

	session.SetStatus(StatusStopped)
	session.emit(Event{
		SessionID: sessionID,
		Type:      "stopped",
		Timestamp: time.Now(),
	})

	// 从管理器中移除
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	return nil
}

// Get 获取会话
func (m *Manager) Get(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// List 列出所有会话
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}

	return sessions
}

// heartbeatLoop 心跳检测循环
func (m *Manager) heartbeatLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkSessions()
		case <-m.heartbeatStop:
			return
		}
	}
}

// checkSessions 检查会话健康状态
func (m *Manager) checkSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for id, session := range m.sessions {
		// 检查是否超时
		if now.Sub(session.LastActivity) > m.config.SessionTimeout {
			// 超时，标记为空闲，可选：自动停止
			// TODO: 实现自动停止策略
			_ = id
		}

		// 检查进程是否存活（如果有进程）
		if session.process != nil {
			if err := session.process.Signal(syscall.Signal(0)); err != nil {
				// 进程已死
				session.SetStatus(StatusError)
				session.emit(Event{
					SessionID: id,
					Type:      "error",
					Data:      "process died unexpectedly",
					Timestamp: time.Now(),
				})
			}
		}
	}
}

// Close 关闭管理器
func (m *Manager) Close() error {
	close(m.heartbeatStop)

	// 等待心跳循环退出
	m.wg.Wait()

	// 停止所有会话
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()

	for _, session := range sessions {
		m.Stop(session.ID, true)
	}

	return nil
}

// generateSessionID 生成会话 ID
func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}
