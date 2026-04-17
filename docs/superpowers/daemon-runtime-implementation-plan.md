# Loom Daemon + Runtime 生产级实现方案

## 1. 架构总览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Loom Daemon + Runtime 完整架构                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                      Daemon (Go CLI) - 常驻进程                       │   │
│   │                                                                     │   │
│   │   ┌─────────────────────────────────────────────────────────────┐   │   │
│   │   │              Control Server (HTTP localhost)                │   │   │
│   │   │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐   │   │   │
│   │   │  │ /spawn      │ │ /stop       │ │ /status             │   │   │   │
│   │   │  │ /list       │ │ /resume     │ │ /health             │   │   │   │
│   │   │  └─────────────┘ └─────────────┘ └─────────────────────┘   │   │   │
│   │   └─────────────────────────────────────────────────────────────┘   │   │
│   │                                                                     │   │
│   │   ┌─────────────────────────────────────────────────────────────┐   │   │
│   │   │                Session Manager (会话管理)                    │   │   │
│   │   │  • PID 跟踪                    • 生命周期管理               │   │   │
│   │   │  • 心跳检测                    • 状态同步                   │   │   │
│   │   │  • 自动恢复                    • 优雅关闭                   │   │   │
│   │   └─────────────────────────────────────────────────────────────┘   │   │
│   │                                                                     │   │
│   │   ┌─────────────────────────────────────────────────────────────┐   │   │
│   │   │              WebSocket Client → Backend (Centrifugo)        │   │   │
│   │   │  • 设备认证                    • 消息路由                   │   │   │
│   │   │  • 重连机制                    • RPC 处理                   │   │   │
│   │   └─────────────────────────────────────────────────────────────┘   │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    │ 启动/控制                              │
│                                    ▼                                        │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                    Runtime Process (子进程)                          │   │
│   │                                                                     │   │
│   │   ┌─────────────────────┐         ┌─────────────────────┐          │   │
│   │   │   Claude Runtime    │         │  OpenCode Runtime   │          │   │
│   │   │   ┌─────────────┐   │         │   ┌─────────────┐   │          │   │
│   │   │   │ Claude Code │   │         │   │  OpenCode   │   │          │   │
│   │   │   │ CLI / SDK   │   │         │   │   CLI/SDK   │   │          │   │
│   │   │   └─────────────┘   │         │   └─────────────┘   │          │   │
│   │   │                     │         │                     │          │   │
│   │   │  ┌───────────────┐  │         │  ┌───────────────┐  │          │   │
│   │   │  │ Agent Loop    │  │         │  │  Agent Loop   │  │          │   │
│   │   │  │ • Chat        │  │         │  │  • Chat       │  │          │   │
│   │   │  │ • Tool Call   │  │         │  │  • Tool Call  │  │          │   │
│   │   │  │ • Execute     │  │         │  │  • Execute    │  │          │   │
│   │   │  │ • Stream      │  │         │  │  • Stream     │  │          │   │
│   │   │  └───────────────┘  │         │  └───────────────┘  │          │   │
│   │   └─────────────────────┘         └─────────────────────┘          │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. 核心组件设计

### 2.1 Runtime 接口设计

```go
// internal/runtime/runtime.go
package runtime

import "context"

// Message 聊天消息
type Message struct {
    Role    string `json:"role"`    // user | assistant | system
    Content string `json:"content"`
}

// ToolCall 工具调用
type ToolCall struct {
    ID        string                 `json:"id"`
    Name      string                 `json:"name"`
    Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult 工具执行结果
type ToolResult struct {
    ToolCallID string `json:"tool_call_id"`
    Output     string `json:"output"`
    Error      string `json:"error,omitempty"`
}

// StreamEvent 流式事件
type StreamEvent struct {
    Type string `json:"type"` // text | tool_call | tool_result | thinking | done | error
    
    // Type=text
    Text string `json:"text,omitempty"`
    
    // Type=tool_call
    ToolCall *ToolCall `json:"tool_call,omitempty"`
    
    // Type=tool_result
    ToolResult *ToolResult `json:"tool_result,omitempty"`
    
    // Type=thinking
    Thinking bool `json:"thinking,omitempty"`
    
    // Type=done
    Done bool `json:"done,omitempty"`
    
    // Type=error
    Error string `json:"error,omitempty"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
    SessionID string    `json:"session_id"`
    Messages  []Message `json:"messages"`
    Tools     []Tool    `json:"tools,omitempty"`
    Stream    bool      `json:"stream"`
}

// Tool 工具定义
type Tool struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    InputSchema Schema `json:"input_schema"`
}

type Schema struct {
    Type       string                 `json:"type"`
    Properties map[string]interface{} `json:"properties"`
    Required   []string               `json:"required"`
}

// Runtime AI运行时接口
type Runtime interface {
    // Name 返回运行时名称
    Name() string
    
    // Chat 发送聊天请求，通过回调接收流式响应
    Chat(ctx context.Context, req ChatRequest, onEvent func(StreamEvent)) error
    
    // ListTools 列出可用工具
    ListTools() []Tool
    
    // ExecuteTool 执行特定工具
    ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error)
    
    // HealthCheck 健康检查
    HealthCheck() error
}

// Factory 创建 Runtime 的工厂
type Factory interface {
    Create(runtimeType string, config Config) (Runtime, error)
}
```

### 2.2 Claude Runtime 实现

```go
// internal/runtime/claude.go
package runtime

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
    "strings"
    "time"
)

// ClaudeRuntime Claude Code 运行时
type ClaudeRuntime struct {
    config    Config
    cliPath   string
    tools     []Tool
}

// Config Claude 运行时配置
type Config struct {
    // Claude CLI 路径，默认使用系统 PATH 中的 claude
    CLIPath string
    
    // 工作目录
    WorkingDir string
    
    // 环境变量
    EnvVars map[string]string
    
    // MCP 服务器配置
    MCPServers map[string]MCPServer
    
    // 允许的工具列表
    AllowedTools []string
    
    // 权限模式
    PermissionMode string // default | auto | yolo
}

type MCPServer struct {
    Type string `json:"type"` // http | stdio
    URL  string `json:"url,omitempty"`
    Command string `json:"command,omitempty"`
}

func NewClaudeRuntime(config Config) (*ClaudeRuntime, error) {
    // 查找 Claude CLI
    cliPath := config.CLIPath
    if cliPath == "" {
        var err error
        cliPath, err = exec.LookPath("claude")
        if err != nil {
            return nil, fmt.Errorf("claude CLI not found in PATH: %w", err)
        }
    }
    
    rt := &ClaudeRuntime{
        config:  config,
        cliPath: cliPath,
        tools:   defaultClaudeTools(),
    }
    
    return rt, nil
}

func (r *ClaudeRuntime) Name() string {
    return "claude-code"
}

func (r *ClaudeRuntime) ListTools() []Tool {
    return r.tools
}

func (r *ClaudeRuntime) Chat(ctx context.Context, req ChatRequest, onEvent func(StreamEvent)) error {
    // 构建 Claude CLI 参数
    args := []string{"--print"}
    
    // 添加系统提示词
    args = append(args, "--system-prompt", buildSystemPrompt(req.SessionID))
    
    // 添加 MCP 服务器配置
    if len(r.config.MCPServers) > 0 {
        mcpConfig := map[string]interface{}{
            "mcpServers": r.config.MCPServers,
        }
        mcpJSON, _ := json.Marshal(mcpConfig)
        args = append(args, "--mcp-config", string(mcpJSON))
    }
    
    // 添加允许的工具
    if len(r.config.AllowedTools) > 0 {
        args = append(args, "--allowed-tools", strings.Join(r.config.AllowedTools, ","))
    }
    
    // 添加权限模式
    if r.config.PermissionMode == "yolo" {
        args = append(args, "--dangerously-skip-permissions")
    }
    
    // 构建消息历史
    prompt := buildPrompt(req.Messages)
    args = append(args, prompt)
    
    // 启动 Claude 进程
    cmd := exec.CommandContext(ctx, r.cliPath, args...)
    cmd.Dir = r.config.WorkingDir
    
    // 设置环境变量
    cmd.Env = buildEnv(r.config.EnvVars)
    
    // 获取 stdout 和 stderr
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return fmt.Errorf("failed to create stdout pipe: %w", err)
    }
    
    stderr, err := cmd.StderrPipe()
    if err != nil {
        return fmt.Errorf("failed to create stderr pipe: %w", err)
    }
    
    // 启动进程
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start claude: %w", err)
    }
    
    // 使用 scanner 读取流式输出
    scanner := bufio.NewScanner(stdout)
    scanner.Split(bufio.ScanLines)
    
    // 解析 Claude 的流式输出
    // Claude Code 的输出格式：
    // 1. 思考状态：通过 stderr 输出
    // 2. 工具调用：$ mcp__<server>__<tool> <json_args>
    // 3. 最终结果：直接文本输出
    
    go func() {
        defer stdout.Close()
        
        for scanner.Scan() {
            line := scanner.Text()
            
            // 解析工具调用
            if strings.HasPrefix(line, "$ mcp__") {
                toolCall, err := parseToolCall(line)
                if err == nil {
                    onEvent(StreamEvent{
                        Type:     "tool_call",
                        ToolCall: toolCall,
                    })
                    
                    // 执行工具调用
                    result, err := r.ExecuteTool(ctx, toolCall.Name, toolCall.Arguments)
                    
                    var toolResult ToolResult
                    if err != nil {
                        toolResult = ToolResult{
                            ToolCallID: toolCall.ID,
                            Error:      err.Error(),
                        }
                    } else {
                        toolResult = ToolResult{
                            ToolCallID: toolCall.ID,
                            Output:     result,
                        }
                    }
                    
                    onEvent(StreamEvent{
                        Type:       "tool_result",
                        ToolResult: &toolResult,
                    })
                }
            } else {
                // 普通文本输出
                onEvent(StreamEvent{
                    Type: "text",
                    Text: line,
                })
            }
        }
    }()
    
    // 监控 stderr 获取思考状态
    go func() {
        scanner := bufio.NewScanner(stderr)
        for scanner.Scan() {
            line := scanner.Text()
            // Claude 在 stderr 输出思考状态
            if strings.Contains(line, "Thinking") {
                onEvent(StreamEvent{
                    Type:     "thinking",
                    Thinking: true,
                })
            }
        }
    }()
    
    // 等待进程结束
    if err := cmd.Wait(); err != nil {
        if ctx.Err() == context.Canceled {
            return nil // 正常取消
        }
        return fmt.Errorf("claude process error: %w", err)
    }
    
    onEvent(StreamEvent{Type: "done", Done: true})
    return nil
}

func (r *ClaudeRuntime) ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error) {
    // 工具执行通过 MCP 服务器或直接执行
    // 对于内置工具（Bash, Read, Edit等），直接执行
    
    switch name {
    case "Bash":
        return executeBashTool(ctx, input)
    case "Read":
        return executeReadTool(ctx, input)
    case "Write":
        return executeWriteTool(ctx, input)
    case "Edit":
        return executeEditTool(ctx, input)
    case "Glob":
        return executeGlobTool(ctx, input)
    case "Grep":
        return executeGrepTool(ctx, input)
    default:
        return "", fmt.Errorf("unknown tool: %s", name)
    }
}

func (r *ClaudeRuntime) HealthCheck() error {
    cmd := exec.Command(r.cliPath, "--version")
    return cmd.Run()
}

// 辅助函数

func defaultClaudeTools() []Tool {
    return []Tool{
        {
            Name:        "Bash",
            Description: "Execute bash commands",
            InputSchema: Schema{
                Type: "object",
                Properties: map[string]interface{}{
                    "command": map[string]string{"type": "string"},
                    "timeout": map[string]string{"type": "number"},
                },
                Required: []string{"command"},
            },
        },
        {
            Name:        "Read",
            Description: "Read file contents",
            InputSchema: Schema{
                Type: "object",
                Properties: map[string]interface{}{
                    "file_path": map[string]string{"type": "string"},
                    "offset":    map[string]string{"type": "number"},
                    "limit":     map[string]string{"type": "number"},
                },
                Required: []string{"file_path"},
            },
        },
        // ... 其他工具
    }
}

func buildPrompt(messages []Message) string {
    var parts []string
    for _, msg := range messages {
        switch msg.Role {
        case "user":
            parts = append(parts, fmt.Sprintf("User: %s", msg.Content))
        case "assistant":
            parts = append(parts, fmt.Sprintf("Assistant: %s", msg.Content))
        case "system":
            parts = append(parts, fmt.Sprintf("System: %s", msg.Content))
        }
    }
    return strings.Join(parts, "\n\n")
}

func buildSystemPrompt(sessionID string) string {
    return fmt.Sprintf(`You are Claude Code, an AI assistant integrated into Loom.
Current session: %s
Use tools when appropriate. Always confirm destructive actions.`, sessionID)
}

func parseToolCall(line string) (*ToolCall, error) {
    // 解析 $ mcp__<server>__<tool> <json>
    parts := strings.SplitN(line, " ", 3)
    if len(parts) < 3 {
        return nil, fmt.Errorf("invalid tool call format")
    }
    
    toolParts := strings.Split(parts[1], "__")
    if len(toolParts) < 2 {
        return nil, fmt.Errorf("invalid tool name format")
    }
    
    var args map[string]interface{}
    if err := json.Unmarshal([]byte(parts[2]), &args); err != nil {
        return nil, err
    }
    
    return &ToolCall{
        ID:        generateToolCallID(),
        Name:      toolParts[len(toolParts)-1],
        Arguments: args,
    }, nil
}

func generateToolCallID() string {
    return fmt.Sprintf("call_%d", time.Now().UnixNano())
}
```

### 2.3 Session Manager 设计

```go
// internal/session/manager.go
package session

import (
    "context"
    "fmt"
    "os"
    "sync"
    "syscall"
    "time"
)

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
    
    // 进程引用
    process *os.Process
    
    // 取消函数
    cancel context.CancelFunc
    
    // 事件回调
    onEvent func(Event)
}

type Status string

const (
    StatusStarting   Status = "starting"
    StatusRunning    Status = "running"
    StatusThinking   Status = "thinking"
    StatusToolCall   Status = "tool_call"
    StatusStopping   Status = "stopping"
    StatusStopped    Status = "stopped"
    StatusError      Status = "error"
)

type Event struct {
    SessionID string
    Type      string // started | message | tool_call | thinking | stopped | error
    Data      interface{}
    Timestamp time.Time
}

// Manager 会话管理器
type Manager struct {
    sessions map[string]*Session
    mu       sync.RWMutex
    
    // 配置
    config ManagerConfig
    
    // 后台任务
    heartbeatStop chan struct{}
}

type ManagerConfig struct {
    // 心跳间隔
    HeartbeatInterval time.Duration
    
    // 会话超时
    SessionTimeout time.Duration
    
    // 最大会话数
    MaxSessions int
}

func NewManager(config ManagerConfig) *Manager {
    if config.HeartbeatInterval == 0 {
        config.HeartbeatInterval = 30 * time.Second
    }
    if config.SessionTimeout == 0 {
        config.SessionTimeout = 10 * time.Minute
    }
    if config.MaxSessions == 0 {
        config.MaxSessions = 10
    }
    
    m := &Manager{
        sessions:      make(map[string]*Session),
        config:        config,
        heartbeatStop: make(chan struct{}),
    }
    
    // 启动心跳检测
    go m.heartbeatLoop()
    
    return m
}

// Spawn 启动新会话
func (m *Manager) Spawn(ctx context.Context, opts SpawnOptions) (*Session, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // 检查最大会话数
    if len(m.sessions) >= m.config.MaxSessions {
        return nil, fmt.Errorf("max sessions reached: %d", m.config.MaxSessions)
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
        onEvent:      opts.OnEvent,
    }
    
    // 创建带取消的上下文
    ctx, cancel := context.WithCancel(ctx)
    session.cancel = cancel
    
    // 根据 RuntimeType 启动进程
    switch opts.RuntimeType {
    case "claude":
        if err := m.spawnClaude(ctx, session, opts); err != nil {
            return nil, err
        }
    case "opencode":
        if err := m.spawnOpenCode(ctx, session, opts); err != nil {
            return nil, err
        }
    default:
        return nil, fmt.Errorf("unknown runtime type: %s", opts.RuntimeType)
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

func (m *Manager) spawnClaude(ctx context.Context, session *Session, opts SpawnOptions) error {
    // 构建 Claude 命令
    args := []string{
        "--working-dir", opts.WorkingDir,
        "--session-id", session.ID,
    }
    
    if opts.Resume {
        args = append(args, "--resume")
    }
    
    cmd := exec.CommandContext(ctx, "claude", args...)
    cmd.Dir = opts.WorkingDir
    cmd.Env = opts.EnvVars
    
    // 设置进程组，便于整体终止
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,
    }
    
    // 启动进程
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start claude: %w", err)
    }
    
    session.PID = cmd.Process.Pid
    session.process = cmd.Process
    session.Status = StatusRunning
    
    // 监控进程退出
    go func() {
        err := cmd.Wait()
        
        m.mu.Lock()
        defer m.mu.Unlock()
        
        if err != nil {
            session.Status = StatusError
            session.emit(Event{
                SessionID: session.ID,
                Type:      "error",
                Data:      err.Error(),
                Timestamp: time.Now(),
            })
        } else {
            session.Status = StatusStopped
            session.emit(Event{
                SessionID: session.ID,
                Type:      "stopped",
                Timestamp: time.Now(),
            })
        }
        
        delete(m.sessions, session.ID)
    }()
    
    return nil
}

// Stop 停止会话
func (m *Manager) Stop(sessionID string, force bool) error {
    m.mu.Lock()
    session, exists := m.sessions[sessionID]
    m.mu.Unlock()
    
    if !exists {
        return fmt.Errorf("session not found: %s", sessionID)
    }
    
    session.Status = StatusStopping
    
    // 先尝试优雅终止
    if session.cancel != nil {
        session.cancel()
    }
    
    if session.process != nil {
        // 发送 SIGTERM
        if err := session.process.Signal(syscall.SIGTERM); err != nil {
            return fmt.Errorf("failed to signal process: %w", err)
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
            } else {
                return fmt.Errorf("timeout waiting for session to stop")
            }
        }
    }
    
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
        // 检查进程是否存活
        if session.process != nil {
            if err := session.process.Signal(syscall.Signal(0)); err != nil {
                // 进程已死
                session.Status = StatusError
                delete(m.sessions, id)
                continue
            }
        }
        
        // 检查超时
        if now.Sub(session.LastActivity) > m.config.SessionTimeout {
            // 超时，标记为空闲
            // 可以选择自动停止
        }
    }
}

// Close 关闭管理器
func (m *Manager) Close() error {
    close(m.heartbeatStop)
    
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

func (s *Session) emit(event Event) {
    if s.onEvent != nil {
        s.onEvent(event)
    }
}

func generateSessionID() string {
    return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

type SpawnOptions struct {
    RuntimeType string
    WorkingDir  string
    Resume      bool
    EnvVars     []string
    Metadata    map[string]string
    OnEvent     func(Event)
}
```

### 2.4 Control Server 设计

```go
// internal/daemon/server.go
package daemon

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// ControlServer Daemon 控制服务器
type ControlServer struct {
    server  *http.Server
    manager *session.Manager
    config  ControlConfig
}

type ControlConfig struct {
    Host string
    Port int
}

func NewControlServer(manager *session.Manager, config ControlConfig) *ControlServer {
    s := &ControlServer{
        manager: manager,
        config:  config,
    }
    
    mux := http.NewServeMux()
    
    // 会话管理端点
    mux.HandleFunc("/v1/sessions", s.handleSessions)
    mux.HandleFunc("/v1/sessions/", s.handleSession)
    
    // 健康检查
    mux.HandleFunc("/health", s.handleHealth)
    
    // 状态端点
    mux.HandleFunc("/v1/status", s.handleStatus)
    
    s.server = &http.Server{
        Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
        Handler:      mux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
    }
    
    return s
}

func (s *ControlServer) Start() error {
    return s.server.ListenAndServe()
}

func (s *ControlServer) Stop(ctx context.Context) error {
    return s.server.Shutdown(ctx)
}

// handleSessions 处理会话列表和创建
func (s *ControlServer) handleSessions(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        // 列出所有会话
        sessions := s.manager.List()
        
        var response []SessionInfo
        for _, sess := range sessions {
            response = append(response, SessionInfo{
                ID:           sess.ID,
                RuntimeType:  sess.RuntimeType,
                Status:       string(sess.Status),
                WorkingDir:   sess.WorkingDir,
                StartedAt:    sess.StartedAt,
                LastActivity: sess.LastActivity,
            })
        }
        
        respondJSON(w, http.StatusOK, response)
        
    case http.MethodPost:
        // 创建新会话
        var req SpawnRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            respondError(w, http.StatusBadRequest, "invalid request body")
            return
        }
        
        opts := session.SpawnOptions{
            RuntimeType: req.RuntimeType,
            WorkingDir:  req.WorkingDir,
            Resume:      req.Resume,
            EnvVars:     req.EnvVars,
            Metadata:    req.Metadata,
            OnEvent: func(evt session.Event) {
                // 通过 WebSocket 发送事件到 Backend
                // TODO: 实现事件转发
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
            Status:       string(sess.Status),
            WorkingDir:   sess.WorkingDir,
            StartedAt:    sess.StartedAt,
            LastActivity: sess.LastActivity,
        })
        
    default:
        respondError(w, http.StatusMethodNotAllowed, "method not allowed")
    }
}

// handleSession 处理单个会话操作
func (s *ControlServer) handleSession(w http.ResponseWriter, r *http.Request) {
    // 从路径提取 session ID
    sessionID := r.URL.Path[len("/v1/sessions/"):]
    
    switch r.Method {
    case http.MethodGet:
        // 获取会话详情
        sess, err := s.manager.Get(sessionID)
        if err != nil {
            respondError(w, http.StatusNotFound, err.Error())
            return
        }
        
        respondJSON(w, http.StatusOK, SessionInfo{
            ID:           sess.ID,
            RuntimeType:  sess.RuntimeType,
            Status:       string(sess.Status),
            WorkingDir:   sess.WorkingDir,
            StartedAt:    sess.StartedAt,
            LastActivity: sess.LastActivity,
        })
        
    case http.MethodDelete:
        // 停止会话
        var req StopRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            req.Force = false
        }
        
        if err := s.manager.Stop(sessionID, req.Force); err != nil {
            respondError(w, http.StatusInternalServerError, err.Error())
            return
        }
        
        respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
        
    default:
        respondError(w, http.StatusMethodNotAllowed, "method not allowed")
    }
}

func (s *ControlServer) handleHealth(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, HealthResponse{
        Status:    "healthy",
        Timestamp: time.Now(),
        Sessions:  len(s.manager.List()),
    })
}

func (s *ControlServer) handleStatus(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, http.StatusOK, StatusResponse{
        Version:   Version,
        StartedAt: time.Now(), // TODO: 记录实际启动时间
        Uptime:    time.Since(time.Now()).String(),
    })
}

// 辅助类型和函数

type SpawnRequest struct {
    RuntimeType string            `json:"runtime_type"`
    WorkingDir  string            `json:"working_dir"`
    Resume      bool              `json:"resume,omitempty"`
    EnvVars     []string          `json:"env_vars,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
}

type StopRequest struct {
    Force bool `json:"force,omitempty"`
}

type SessionInfo struct {
    ID           string    `json:"id"`
    RuntimeType  string    `json:"runtime_type"`
    Status       string    `json:"status"`
    WorkingDir   string    `json:"working_dir"`
    StartedAt    time.Time `json:"started_at"`
    LastActivity time.Time `json:"last_activity"`
}

type HealthResponse struct {
    Status    string    `json:"status"`
    Timestamp time.Time `json:"timestamp"`
    Sessions  int       `json:"sessions"`
}

type StatusResponse struct {
    Version   string `json:"version"`
    StartedAt time.Time `json:"started_at"`
    Uptime    string `json:"uptime"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
    respondJSON(w, status, map[string]string{"error": message})
}
```

---

## 3. 完整通信流程

### 3.1 消息处理流程

```
用户输入
    │
    ▼
┌─────────────────┐
│   Backend API   │  POST /api/messages
│   (Bun/TS)      │  {session_id, content}
└────────┬────────┘
         │
         │ 1. 保存到 DB
         │ 2. 发送到 Centrifugo
         ▼
┌─────────────────┐
│   Centrifugo    │  channel: daemon:{device_id}
│   (WS Broker)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     Daemon      │  WebSocket Client
│    (Go CLI)     │  接收消息
└────────┬────────┘
         │
         │ 1. 解析消息
         │ 2. 路由到 Runtime
         ▼
┌─────────────────┐
│  Session Manager│  找到/创建会话
│                 │  调用 Runtime.Chat()
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Claude Runtime │  启动 claude CLI
│                 │  流式响应
└────────┬────────┘
         │
         │ 流式事件：
         │ {type: "text", text: "..."}
         │ {type: "thinking", thinking: true}
         │ {type: "tool_call", tool_call: {...}}
         ▼
┌─────────────────┐
│     Daemon      │  转发到 Backend
│                 │  通过 Centrifugo
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Centrifugo    │  channel: user:{device_id}
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Backend      │  保存到 DB
│                 │  转发给 Client
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     Client      │  显示消息
│    (React)      │  更新 UI
└─────────────────┘
```

### 3.2 会话生命周期

```
┌──────────┐
│  Idle    │
└────┬─────┘
     │ spawn
     ▼
┌──────────┐     ┌──────────┐
│ Starting │────▶│  Error   │
└────┬─────┘     └──────────┘
     │ started
     ▼
┌──────────┐     ┌──────────┐
│ Running  │────▶│ Thinking │
└────┬─────┘     └────┬─────┘
     │                │
     │ tool_call      │ done
     ▼                ▼
┌──────────┐     ┌──────────┐
│ToolCall  │     │ Running  │
└────┬─────┘     └────┬─────┘
     │                │
     │ result         │ stop
     ▼                ▼
└──────────┐     ┌──────────┐
│ Running  │────▶│ Stopping │
└──────────┘     └────┬─────┘
                      │ stopped
                      ▼
                 ┌──────────┐
                 │ Stopped  │
                 └──────────┘
```

---

## 4. 实现阶段规划

### Phase 1: 基础框架（2-3周）

```
Week 1-2: Runtime 接口与实现
├── internal/runtime/
│   ├── runtime.go          # 接口定义
│   ├── claude.go           # Claude 实现
│   └── opencode.go         # OpenCode 实现
├── internal/tools/
│   ├── bash.go
│   ├── read.go
│   ├── write.go
│   ├── edit.go
│   ├── glob.go
│   └── grep.go
└── tests/
    └── runtime_test.go

Week 2-3: Session Manager
├── internal/session/
│   ├── manager.go          # 会话管理
│   ├── process.go          # 进程管理
│   └── event.go            # 事件处理
└── tests/
    └── session_test.go
```

### Phase 2: Control Server（1-2周）

```
Week 3-4: HTTP Control Server
├── internal/daemon/
│   ├── server.go           # HTTP 服务器
│   ├── handlers.go         # 请求处理器
│   └── middleware.go       # 中间件
├── internal/api/
│   └── types.go            # API 类型
└── tests/
    └── server_test.go
```

### Phase 3: WebSocket 集成（2周）

```
Week 4-5: Centrifugo 集成
├── internal/ws/
│   ├── client.go           # WebSocket 客户端
│   ├── handlers.go         # 消息处理器
│   └── rpc.go              # RPC 实现
├── internal/messaging/
│   ├── router.go           # 消息路由
│   └── formatter.go        # 消息格式化
└── tests/
    └── ws_test.go
```

### Phase 4: 集成测试与优化（2周）

```
Week 5-6: 端到端测试
├── tests/
│   ├── integration/
│   │   ├── chat_flow_test.go
│   │   ├── tool_call_test.go
│   │   └── session_lifecycle_test.go
│   └── e2e/
│       └── full_pipeline_test.go
├── docs/
│   └── api.md
└── scripts/
    └── test.sh
```

---

## 5. 关键技术决策

### 5.1 决策记录

| 决策 | 选择 | 理由 |
|------|------|------|
| Runtime 调用方式 | Claude CLI | 官方稳定，无需逆向 |
| 进程管理 | PID + 信号 | 跨平台，可控 |
| IPC 方式 | HTTP localhost | 简单，可调试 |
| 消息格式 | JSON | 通用，易调试 |
| 流式响应 | 回调函数 | Go 习惯用法 |
| 会话恢复 | 依赖 Claude --resume | 保持一致性 |

### 5.2 与 Happy 的主要区别

```
Happy                          Loom (我们的设计)
─────────────────────────────────────────────────────
Node.js + TypeScript           Go
Socket.IO 直连                 Centrifugo 中间件
远程优先设计                   本地优先 + 云端同步
完整 E2E 加密                  传输层 TLS (初期)
复杂的本地/远程切换            简化模式
```

---

## 6. 目录结构规划

```
cmd/daemon/
├── main.go                      # 入口
├── cmd/
│   ├── start.go                # start 命令
│   ├── stop.go                 # stop 命令
│   └── status.go               # status 命令
├── internal/
│   ├── runtime/
│   │   ├── runtime.go          # 接口
│   │   ├── claude.go           # Claude 实现
│   │   ├── opencode.go         # OpenCode 实现
│   │   └── factory.go          # 工厂
│   ├── session/
│   │   ├── manager.go          # 会话管理
│   │   ├── process.go          # 进程管理
│   │   └── event.go            # 事件
│   ├── daemon/
│   │   ├── server.go           # HTTP 服务器
│   │   ├── handlers.go         # 处理器
│   │   └── config.go           # 配置
│   ├── ws/
│   │   ├── client.go           # WebSocket 客户端
│   │   ├── handlers.go         # 消息处理
│   │   └── rpc.go              # RPC
│   ├── tools/
│   │   ├── bash.go
│   │   ├── read.go
│   │   ├── write.go
│   │   ├── edit.go
│   │   ├── glob.go
│   │   └── grep.go
│   └── config/
│       └── config.go           # 全局配置
├── pkg/
│   └── api/
│       └── types.go            # 共享类型
├── tests/
│   ├── runtime_test.go
│   ├── session_test.go
│   └── integration/
│       └── chat_test.go
├── go.mod
├── go.sum
└── README.md
```

---

## 7. 风险与缓解

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| Claude CLI 接口变更 | 中 | 高 | 封装抽象层，快速适配 |
| 进程管理复杂性 | 中 | 中 | 完善的测试，日志记录 |
| 跨平台兼容 | 中 | 中 | CI 多平台测试 |
| 性能瓶颈 | 低 | 中 | 基准测试，优化 |
| 内存泄漏 | 中 | 高 | pprof 分析，定期检查 |

---

## 8. 成功标准

- ✅ Runtime 能成功调用 Claude Code 并获取响应
- ✅ 工具调用循环正常工作
- ✅ 会话管理能启动/停止/监控进程
- ✅ Control Server 响应正确
- ✅ WebSocket 消息正确路由
- ✅ 端到端测试通过
- ✅ 内存使用稳定
