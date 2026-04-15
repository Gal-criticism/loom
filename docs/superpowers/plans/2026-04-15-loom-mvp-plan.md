# Loom MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建 Loom MVP 版本，包含核心对话能力和基础 lofi 音乐播放功能

**Architecture:** MVP 分为三个核心组件：Daemon（Go CLI）、Backend（Bun + TanStack Start）、Client（TanStack Start 前端）。Daemon 通过 WebSocket 与 Backend 通信，Client 通过 HTTP + WebSocket 访问 Backend。

**Tech Stack:**
- 前端：TanStack Start + TanStack Query
- 后端：Bun + TanStack Start
- Daemon：Go
- 数据库：PostgreSQL
- WebSocket：Centrifugo

---

## 架构概览

```
┌─────────────────────────────────────────────────────────────────────┐
│                     MVP 阶段一：基础设施                              │
├─────────────────────────────────────────────────────────────────────┤
│ Task 1: 项目初始化与目录结构                                         │
│ Task 2: Docker Compose 环境搭建                                     │
│ Task 3: PostgreSQL 数据库 schema                                     │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                     MVP 阶段二：Daemon (Go)                         │
├─────────────────────────────────────────────────────────────────────┤
│ Task 4: Go 项目初始化与依赖                                         │
│ Task 5: Runtime 抽象层实现                                          │
│ Task 6: WebSocket 客户端实现                                        │
│ Task 7: HTTP/gRPC API 实现                                          │
│ Task 8: CLI 命令行工具                                              │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                     MVP 阶段三：Backend (Bun)                       │
├─────────────────────────────────────────────────────────────────────┤
│ Task 9: TanStack Start 项目初始化                                   │
│ Task 10: 基础 API 结构搭建                                          │
│ Task 11: WebSocket 消息路由                                         │
│ Task 12: 设备指纹认证                                               │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                     MVP 阶段四：Client (前端)                       │
├─────────────────────────────────────────────────────────────────────┤
│ Task 13: TanStack Start 前端初始化                                  │
│ Task 14: 基础 UI 布局与分层结构                                     │
│ Task 15: lofi 音乐播放组件                                          │
│ Task 16: 对话 UI 组件                                               │
│ Task 17: WebSocket 集成                                             │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 阶段一：基础设施

### Task 1: 项目初始化与目录结构

**Files:**
- Create: `docker-compose.yml`
- Create: `Makefile`
- Create: `README.md`
- Modify: `.gitignore`

- [ ] **Step 1: 创建项目目录结构**

```bash
mkdir -p loom/{cmd/{daemon,backend,client},internal/{daemon,backend,common},pkg/{runtime,ws,api},migrations}
```

- [ ] **Step 2: 创建 docker-compose.yml**

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: loom
      POSTGRES_PASSWORD: loom_dev
      POSTGRES_DB: loom
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  centrigugo:
    image: centrifugo/centrifugo:v5
    ports:
      - "8000:8000"
    volumes:
      - ./centrifugo_config.json:/etc/centrifugo/config.json
    command: centrifugo -c /etc/centrifugo/config.json

volumes:
  postgres_data:
```

- [ ] **Step 3: 创建 Makefile**

```makefile
.PHONY: dev setup build

setup:
	docker-compose up -d

dev:
	docker-compose up

build-daemon:
	cd cmd/daemon && go build -o ../../bin/loomd .

build-backend:
	bun run backend/src/index.ts

clean:
	docker-compose down -v
```

- [ ] **Step 4: 创建 .gitignore**

```gitignore
# Binaries
bin/
dist/

# Environment
.env
.env.local

# IDE
.vscode/
.idea/

# OS
.DS_Store

# Logs
*.log
```

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "chore: initial project structure with docker-compose"
```

---

### Task 2: Docker Compose 环境搭建

**Files:**
- Modify: `docker-compose.yml`
- Create: `centrifugo_config.json`
- Create: `.env`

- [ ] **Step 1: 创建 Centrifugo 配置文件**

```json
{
  "token_secret": "dev-secret-change-in-production",
  "api_key": "dev-api-key-change-in-production",
  "admin_secret": "dev-admin-secret",
  "port": "8000",
  "engine": "memory",
  "allowed_origins": ["http://localhost:3000", "http://localhost:8080"]
}
```

- [ ] **Step 2: 创建环境变量文件**

```bash
# Database
DATABASE_URL=postgresql://loom:loom_dev@localhost:5432/loom

# Centrifugo
CENTRIFUGO_URL=ws://localhost:8000
CENTRIFUGO_API_KEY=dev-api-key-change-in-production

# App
APP_PORT=3000
LOG_LEVEL=debug
```

- [ ] **Step 3: 修改 docker-compose.yml 添加环境变量**

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: loom
      POSTGRES_PASSWORD: loom_dev
      POSTGRES_DB: loom
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d

  centrigugo:
    image: centrifugo/centrifugo:v5
    ports:
      - "8000:8000"
    volumes:
      - ./centrifugo_config.json:/etc/centrifugo/config.json
    command: centrifugo -c /etc/centrifugo/config.json

  backend:
    build:
      context: .
      dockerfile: Dockerfile.backend
    ports:
      - "3000:3000"
    environment:
      DATABASE_URL: postgresql://loom:loom_dev@postgres:5432/loom
      CENTRIFUGO_URL: ws://centrigugo:8000
    depends_on:
      - postgres
      - centrigugo

volumes:
  postgres_data:
```

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "chore: add docker-compose environment config"
```

---

### Task 3: PostgreSQL 数据库 Schema

**Files:**
- Create: `migrations/001_initial_schema.sql`

- [ ] **Step 1: 创建初始 schema**

```sql
-- Users table (device fingerprint + email auth)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    password_hash VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login TIMESTAMP WITH TIME ZONE
);

-- Characters table (user's custom characters)
CREATE TABLE characters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    prompt TEXT,
    avatar_url VARCHAR(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Sessions table
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    character_id UUID REFERENCES characters(id) ON DELETE SET NULL,
    title VARCHAR(200),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Messages table
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content TEXT NOT NULL,
    tools JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_messages_session_id ON messages(session_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);
```

- [ ] **Step 2: Commit**

```bash
git add migrations/001_initial_schema.sql
git commit -m "db: add initial PostgreSQL schema"
```

---

## 阶段二：Daemon (Go)

### Task 4: Go 项目初始化与依赖

**Files:**
- Create: `cmd/daemon/go.mod`
- Create: `cmd/daemon/main.go`
- Create: `cmd/daemon/config/config.go`

- [ ] **Step 1: 初始化 Go module**

```bash
cd cmd/daemon
go mod init github.com/loom/daemon
```

- [ ] **Step 2: 添加依赖**

```bash
go get github.com/gorilla/websocket
go get github.com/spf13/cobra
go get gopkg.in/yaml.v3
go get github.com/google/uuid
go get github.com/sashabaranov/go-openai
```

- [ ] **Step 3: 创建配置文件结构**

```go
// config/config.go
package config

import (
    "os"
)

type Config struct {
    Runtime   string `yaml:"runtime"`
    BackendWS string `yaml:"backend_ws"`
    Listen    string `yaml:"listen"`
    LogLevel  string `yaml:"log_level"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return defaultConfig(), nil
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}

func defaultConfig() *Config {
    return &Config{
        Runtime:   "claude-code",
        BackendWS: "ws://localhost:3000/ws/daemon",
        Listen:    "localhost:3456",
        LogLevel:  "info",
    }
}
```

- [ ] **Step 4: 创建主入口**

```go
// main.go
package main

import (
    "log"
    "os"

    "github.com/loom/daemon/cmd"
    "github.com/loom/daemon/config"
)

func main() {
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    if err := cmd.Root.Execute(); err != nil {
        os.Exit(1)
    }

    _ = cfg
}
```

- [ ] **Step 5: 创建基础 CLI 框架**

```go
// cmd/root.go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var Root = &cobra.Command{
    Use:   "loomd",
    Short: "Loom Daemon - Local AI Runtime Manager",
    Long:  "Loom Daemon connects local AI Runtime (Claude Code, OpenCode) to Loom Cloud",
}

func Execute() error {
    return Root.Execute()
}
```

- [ ] **Step 6: Commit**

```bash
cd cmd/daemon
git add .
cd ../..
git add cmd/daemon
git commit -m "feat(daemon): initialize Go project with cobra CLI"
```

---

### Task 5: Runtime 抽象层实现

**Files:**
- Create: `cmd/daemon/runtime/adapter.go`
- Create: `cmd/daemon/runtime/claude.go`
- Create: `cmd/daemon/runtime/opencode.go`

- [ ] **Step 1: 定义 Runtime 接口**

```go
// runtime/adapter.go
package runtime

import "context"

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type Tool struct {
    Name      string                 `json:"name"`
    Input     map[string]interface{} `json:"input"`
}

type ChatRequest struct {
    Messages []Message `json:"messages"`
    Tools    []Tool    `json:"tools,omitempty"`
    Stream   bool      `json:"stream"`
}

type ChatResponse struct {
    Content  string `json:"content"`
    ToolCall *Tool  `json:"tool_call,omitempty"`
    Done     bool   `json:"done"`
}

type Runtime interface {
    // Chat sends a chat request and returns responses via callback
    Chat(ctx context.Context, req ChatRequest, onResponse func(ChatResponse)) error
    
    // ExecuteTool executes a specific tool
    ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error)
    
    // ListCapabilities returns available tools/skills
    ListCapabilities() ([]string, []string)
    
    // Name returns the runtime name
    Name() string
}
```

- [ ] **Step 2: 创建 Claude Code 适配器**

```go
// runtime/claude.go
package runtime

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
)

type ClaudeRuntime struct{}

func NewClaudeRuntime() Runtime {
    return &ClaudeRuntime{}
}

func (r *ClaudeRuntime) Name() string {
    return "claude-code"
}

func (r *ClaudeRuntime) ListCapabilities() ([]string, []string) {
    // Claude Code 内置能力需要通过 --print 和 --print 标记查询
    // 简化版本：返回基础工具
    return []string{"Bash", "Read", "Edit", "Write", "Glob", "Grep"}, nil
}

func (r *ClaudeRuntime) Chat(ctx context.Context, req ChatRequest, onResponse func(ChatResponse)) error {
    // 实现 Claude Code CLI 调用
    // 使用 claude-code -p 或类似接口
    return r.invokeClaude(ctx, req, onResponse)
}

func (r *ClaudeRuntime) invokeClaude(ctx context.Context, req ChatRequest, onResponse func(ChatResponse)) error {
    cmd := exec.CommandContext(ctx, "claude-code", []string{}...)
    
    stdin, err := cmd.StdinPipe()
    if err != nil {
        return err
    }
    
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return err
    }
    
    if err := cmd.Start(); err != nil {
        return err
    }
    
    // 发送请求
    encoder := json.NewEncoder(stdin)
    if err := encoder.Encode(req); err != nil {
        return err
    }
    stdin.Close()
    
    // 读取响应
    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        var resp ChatResponse
        if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
            continue
        }
        onResponse(resp)
        if resp.Done {
            break
        }
    }
    
    return cmd.Wait()
}

func (r *ClaudeRuntime) ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error) {
    // 执行工具
    req := ChatRequest{
        Messages: []Message{
            {Role: "user", Content: fmt.Sprintf("Execute tool %s with input: %v", name, input)},
        },
        Tools: []Tool{{Name: name, Input: input}},
    }
    
    var result string
    err := r.Chat(ctx, req, func(resp ChatResponse) {
        result += resp.Content
    })
    
    return result, err
}
```

- [ ] **Step 3: 创建 OpenCode 适配器**

```go
// runtime/opencode.go
package runtime

type OpenCodeRuntime struct{}

func NewOpenCodeRuntime() Runtime {
    return &OpenCodeRuntime{}
}

func (r *OpenCodeRuntime) Name() string {
    return "opencode"
}

func (r *OpenCodeRuntime) ListCapabilities() ([]string, []string) {
    // OpenCode 能力列表
    return []string{"Bash", "Read", "Edit", "Write", "Glob", "Grep"}, nil
}

func (r *OpenCodeRuntime) Chat(ctx context.Context, req ChatRequest, onResponse func(ChatResponse)) error {
    // OpenCode 实现
    return r.invokeOpenCode(ctx, req, onResponse)
}

func (r *OpenCodeRuntime) invokeOpenCode(ctx context.Context, req ChatRequest, onResponse func(ChatResponse)) error {
    // TODO: 实现 OpenCode CLI 调用
    return nil
}

func (r *OpenCodeRuntime) ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error) {
    req := ChatRequest{
        Messages: []Message{
            {Role: "user", Content: fmt.Sprintf("Execute tool %s with input: %v", name, input)},
        },
        Tools: []Tool{{Name: name, Input: input}},
    }
    
    var result string
    err := r.Chat(ctx, req, func(resp ChatResponse) {
        result += resp.Content
    })
    
    return result, err
}
```

- [ ] **Step 4: 创建适配器工厂**

```go
// runtime/factory.go
package runtime

func NewRuntime(runtimeType string) (Runtime, error) {
    switch runtimeType {
    case "claude-code", "claude":
        return NewClaudeRuntime(), nil
    case "opencode", "open-code":
        return NewOpenCodeRuntime(), nil
    default:
        return nil, fmt.Errorf("unknown runtime: %s", runtimeType)
    }
}
```

- [ ] **Step 5: Commit**

```bash
git add cmd/daemon/runtime/
git commit -m "feat(daemon): add runtime abstraction layer"
```

---

### Task 6: WebSocket 客户端实现

**Files:**
- Create: `cmd/daemon/ws/client.go`

- [ ] **Step 1: 创建 WebSocket 客户端**

```go
// ws/client.go
package ws

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

type Message struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

type Client struct {
    conn     *websocket.Conn
    url      string
    deviceID string
    mu       sync.Mutex
    handlers map[string]Handler
    ctx      context.Context
    cancel   context.CancelFunc
}

type Handler func(payload json.RawMessage) error

func NewClient(url, deviceID string) *Client {
    return &Client{
        url:      url,
        deviceID: deviceID,
        handlers: make(map[string]Handler),
    }
}

func (c *Client) Connect(ctx context.Context) error {
    c.ctx, c.cancel = context.WithCancel(ctx)
    
    conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
    if err != nil {
        return fmt.Errorf("failed to connect to backend: %w", err)
    }
    
    c.conn = conn
    
    // 发送注册消息
    registerMsg := Message{
        Type: "register",
        Payload: mustMarshal(map[string]string{
            "device_id": c.deviceID,
            "runtime":   "claude-code",
        }),
    }
    
    if err := c.conn.WriteJSON(registerMsg); err != nil {
        return err
    }
    
    go c.readLoop()
    return nil
}

func (c *Client) readLoop() {
    for {
        select {
        case <-c.ctx.Done():
            c.conn.Close()
            return
        default:
            _, data, err := c.conn.ReadMessage()
            if err != nil {
                log.Printf("Error reading message: %v", err)
                continue
            }
            
            var msg Message
            if err := json.Unmarshal(data, &msg); err != nil {
                log.Printf("Error unmarshaling message: %v", err)
                continue
            }
            
            if handler, ok := c.handlers[msg.Type]; ok {
                if err := handler(msg.Payload); err != nil {
                    log.Printf("Error handling message: %v", err)
                }
            }
        }
    }
}

func (c *Client) On(event string, handler Handler) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.handlers[event] = handler
}

func (c *Client) Send(typ string, payload interface{}) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    data, err := json.Marshal(Message{
        Type:    typ,
        Payload: mustMarshal(payload),
    })
    if err != nil {
        return err
    }
    
    return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) Close() error {
    if c.cancel != nil {
        c.cancel()
    }
    if c.conn != nil {
        return c.conn.Close()
    }
    return nil
}

func mustMarshal(v interface{}) json.RawMessage {
    data, _ := json.Marshal(v)
    return data
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/daemon/ws/
git commit -m "feat(daemon): add WebSocket client"
```

---

### Task 7: HTTP/gRPC API 实现

**Files:**
- Create: `cmd/daemon/api/server.go`

- [ ] **Step 1: 创建 HTTP 服务器**

```go
// api/server.go
package api

import (
    "encoding/json"
    "log"
    "net/http"

    "github.com/loom/daemon/runtime"
)

type Server struct {
    runtime runtime.Runtime
    addr    string
}

type ChatRequest struct {
    Messages []runtime.Message `json:"messages"`
    Tools    []runtime.Tool    `json:"tools"`
    Stream   bool              `json:"stream"`
}

type ChatResponse struct {
    Content  string `json:"content"`
    ToolCall  *runtime.Tool `json:"tool_call,omitempty"`
    Done      bool   `json:"done"`
}

type ToolRequest struct {
    Name  string                 `json:"name"`
    Input map[string]interface{} `json:"input"`
}

type ToolResponse struct {
    Output string `json:"output"`
    Error  string `json:"error,omitempty"`
}

type CapabilitiesResponse struct {
    Tools  []string `json:"tools"`
    Skills []string `json:"skills"`
}

func NewServer(rt runtime.Runtime, addr string) *Server {
    return &Server{
        runtime: rt,
        addr:    addr,
    }
}

func (s *Server) Start() error {
    http.HandleFunc("/api/chat", s.handleChat)
    http.HandleFunc("/api/tools", s.handleTools)
    http.HandleFunc("/api/tools/execute", s.handleToolExecute)
    http.HandleFunc("/api/capabilities", s.handleCapabilities)
    http.HandleFunc("/health", s.handleHealth)
    
    log.Printf("Starting API server on %s", s.addr)
    return http.ListenAndServe(s.addr, nil)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req ChatRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    encoder := json.NewEncoder(w)
    
    runtimeReq := runtime.ChatRequest{
        Messages: req.Messages,
        Tools:    req.Tools,
        Stream:   req.Stream,
    }
    
    err := s.runtime.Chat(r.Context(), runtimeReq, func(resp runtime.ChatResponse) {
        encoder.Encode(ChatResponse{
            Content:  resp.Content,
            ToolCall:  resp.ToolCall,
            Done:      resp.Done,
        })
    })
    
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    tools, _ := s.runtime.ListCapabilities()
    json.NewEncoder(w).Encode(ToolsResponse{Tools: tools})
}

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req ToolRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    output, err := s.runtime.ExecuteTool(r.Context(), req.Name, req.Input)
    if err != nil {
        json.NewEncoder(w).Encode(ToolResponse{Error: err.Error()})
        return
    }
    
    json.NewEncoder(w).Encode(ToolResponse{Output: output})
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
    tools, skills := s.runtime.ListCapabilities()
    json.NewEncoder(w).Encode(CapabilitiesResponse{
        Tools:  tools,
        Skills: skills,
    })
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type ToolsResponse struct {
    Tools []string `json:"tools"`
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/daemon/api/
git commit -m "feat(daemon): add HTTP API server"
```

---

### Task 8: CLI 命令行工具

**Files:**
- Modify: `cmd/daemon/cmd/root.go`
- Create: `cmd/daemon/cmd/start.go`
- Create: `cmd/daemon/cmd/version.go`

- [ ] **Step 1: 更新 root 命令**

```go
// cmd/root.go
package cmd

import (
    "github.com/spf13/cobra"
)

var (
    verbose bool
    config  string
)

var Root = &cobra.Command{
    Use:   "loomd",
    Short: "Loom Daemon - Local AI Runtime Manager",
    Long:  `Loom Daemon connects local AI Runtime (Claude Code, OpenCode) to Loom Cloud`,
}

func init() {
    Root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
    Root.PersistentFlags().StringVar(&config, "config", "config.yaml", "Config file path")
    Root.AddCommand(StartCmd)
    Root.AddCommand(VersionCmd)
}
```

- [ ] **Step 2: 创建 start 命令**

```go
// cmd/start.go
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
```

- [ ] **Step 3: 创建 version 命令**

```go
// cmd/version.go
package cmd

import (
    "fmt"

    "github.com/spf13/cobra"
)

var VersionCmd = &cobra.Command{
    Use:   "version",
    Short: "Show version",
    Run:   runVersion,
}

func runVersion(cmd *cobra.Command, args []string) {
    fmt.Println("Loom Daemon v0.1.0")
}
```

- [ ] **Step 4: Commit**

```bash
git add cmd/daemon/cmd/
git commit -m "feat(daemon): add CLI commands (start, version)"
```

---

## 阶段三：Backend (Bun)

### Task 9: TanStack Start 项目初始化

**Files:**
- Create: `backend/package.json`
- Create: `backend/tsconfig.json`
- Create: `backend/src/index.ts`

- [ ] **Step 1: 初始化 Bun 项目**

```bash
mkdir -p backend
cd backend
bun init -y
```

- [ ] **Step 2: 添加 TanStack Start 和依赖**

```bash
bun add @tanstack/start @tanstack/react-query react react-dom
bun add -d @types/react @types/react-dom typescript vite
```

- [ ] **Step 3: 创建 TanStack Start 配置**

```typescript
// vite.config.ts
import { defineConfig } from 'vite'
import tsconfigPaths from 'vite-tsconfig-paths'

export default defineConfig({
  plugins: [tsconfigPaths()],
  server: {
    port: 3000,
  },
})
```

```json
// tsconfig.json
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "paths": {
      "~/*": ["./src/*"]
    }
  }
}
```

- [ ] **Step 4: 创建入口文件**

```typescript
// src/index.ts
import { createStartHandler } from "@tanstack/start";
import { getRouter } from "./router";

export default createStartHandler({
  createRouter: () => getRouter(),
});
```

- [ ] **Step 5: Commit**

```bash
git add backend/
git commit -m "feat(backend): initialize TanStack Start project"
```

---

### Task 10: 基础 API 结构搭建

**Files:**
- Create: `backend/src/router.ts`
- Create: `backend/src/routes/api/auth.ts`
- Create: `backend/src/lib/db.ts`

- [ ] **Step 1: 创建路由和 API 结构**

```typescript
// src/router.ts
import { createRouter as createTanStackRouter } from "@tanstack/start/router";
import { routeTree } from "./routeTree.gen";

export function getRouter() {
  return createTanStackRouter({ routeTree });
}

export function createRouter() {
  return getRouter();
}
```

```typescript
// src/routeTree.gen.ts
import { Route as rootRoute } from "./routes/__root";
import { Route as apiRoute } from "./routes/api";
import { Route as indexRoute } from "./routes/index";

declare module "@tanstack/start" {
  interface FileRoutesByPath {
    "/": {
      preLoaderRoute: typeof indexRoute;
      parentRoute: typeof rootRoute;
    };
    "/api/auth": {
      preLoaderRoute: typeof apiRoute;
      parentRoute: typeof rootRoute;
    };
  }
}

export const routeTree = rootRoute.addChildren([indexRoute, apiRoute.addChildren([])]);
```

```typescript
// src/routes/index.ts
import { Route } from "@tanstack/start";

export const indexRoute = new Route({
  getStaticProps: async () => {
    return {
      head: {
        title: "Loom",
        meta: [
          { name: "description", content: "AI Companion with Vibe" },
        ],
      },
    };
  },
  component: () => <div>Loom MVP</div>,
});
```

- [ ] **Step 2: 创建数据库连接**

```typescript
// src/lib/db.ts
import { Pool } from "pg";

export const pool = new Pool({
  connectionString: process.env.DATABASE_URL || "postgresql://loom:loom_dev@localhost:5432/loom",
});

export const db = {
  query: async (text: string, params?: any[]) => {
    const result = await pool.query(text, params);
    return result;
  },
};
```

- [ ] **Step 3: 创建认证 API**

```typescript
// src/routes/api/auth/register.ts
import { Route, json } from "@tanstack/start";
import { db } from "~/lib/db";

export const registerRoute = new Route({
  path: "/api/auth/register",
  method: "POST",
  handler: async (req: Request) => {
    const { email, password } = await req.json();
    
    // TODO: 实现密码哈希
    const passwordHash = "hashed_" + password;
    
    try {
      const result = await db.query(
        "INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id, email",
        [email, passwordHash]
      );
      
      return json({ user: result.rows[0] });
    } catch (error) {
      return json({ error: "User already exists" }, { status: 400 });
    }
  },
});
```

- [ ] **Step 4: Commit**

```bash
git add backend/src/
git commit -m "feat(backend): add basic API structure"
```

---

### Task 11: WebSocket 消息路由

**Files:**
- Create: `backend/src/lib/ws.ts`

- [ ] **Step 1: 创建 WebSocket 处理**

```typescript
// src/lib/ws.ts
import { Centrifuge } from "centrifuge";

export interface WSServer {
  handleConnection(ws: WebSocket, deviceId: string): void;
  broadcast(userId: string, message: any): void;
  sendToDaemon(deviceId: string, message: any): void;
}

export function createWSServer(): WSServer {
  const centrifuge = new Centrifuge("ws://localhost:8000", {
    token: process.env.CENTRIFUGO_TOKEN || "dev-token",
  });

  return {
    handleConnection(ws: WebSocket, deviceId: string) {
      // 注册设备到 Daemon 频道
      const channel = `daemon:${deviceId}`;
      centrifuge.subscribe(channel, {
        message: (ctx) => {
          ws.send(JSON.stringify(ctx.data));
        },
      });
    },
    
    broadcast(userId: string, message: any) {
      centrifuge.publish(`user:${userId}`, message);
    },
    
    sendToDaemon(deviceId: string, message: any) {
      centrifuge.publish(`daemon:${deviceId}`, message);
    },
  };
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/src/lib/ws.ts
git commit -m "feat(backend): add WebSocket message routing"
```

---

### Task 12: 设备指纹认证

**Files:**
- Modify: `backend/src/routes/api/auth.ts`
- Create: `backend/src/routes/api/auth/device.ts`

- [ ] **Step 1: 创建设备指纹认证**

```typescript
// src/routes/api/auth/device.ts
import { Route, json } from "@tanstack/start";
import { db } from "~/lib/db";
import { v4 as uuidv4 } from "uuid";

export const deviceAuthRoute = new Route({
  path: "/api/auth/device",
  method: "POST",
  handler: async (req: Request) => {
    const { device_id } = await req.json();
    
    if (!device_id) {
      // 生成新设备 ID
      const newDeviceId = uuidv4();
      
      const result = await db.query(
        "INSERT INTO users (device_id) VALUES ($1) RETURNING id, device_id",
        [newDeviceId]
      );
      
      return json({ 
        user: result.rows[0],
        is_new: true 
      });
    }
    
    // 查找现有用户
    const result = await db.query(
      "SELECT id, device_id FROM users WHERE device_id = $1",
      [device_id]
    );
    
    if (result.rows.length === 0) {
      return json({ error: "Device not found" }, { status: 404 });
    }
    
    // 更新最后登录时间
    await db.query(
      "UPDATE users SET last_login = NOW() WHERE device_id = $1",
      [device_id]
    );
    
    return json({ 
      user: result.rows[0],
      is_new: false 
    });
  },
});
```

- [ ] **Step 2: Commit**

```bash
git add backend/src/routes/api/auth/
git commit -m "feat(backend): add device fingerprint auth"
```

---

## 阶段四：Client (前端)

### Task 13: TanStack Start 前端初始化

**Files:**
- Create: `client/src/main.tsx`
- Create: `client/src/App.tsx`
- Create: `client/src/routes/__root.tsx`
- Modify: `client/vite.config.ts`

- [ ] **Step 1: 初始化前端项目**

```bash
mkdir -p client
cd client
bun init -y
bun add @tanstack/start @tanstack/react-query react react-dom
bun add -d @types/react @types/react-dom typescript vite
```

- [ ] **Step 2: 创建入口文件**

```typescript
// src/main.tsx
import { createRoot } from "react-dom/client";
import { StartServer } from "./StartServer";
import "./styles.css";

createRoot(document.getElementById("root")!).render(<StartServer />);
```

```typescript
// src/StartServer.tsx
import { createStartHandler } from "@tanstack/start";
import { getRouter } from "./router";

export default createStartHandler({
  createRouter: () => getRouter(),
});
```

- [ ] **Step 3: 创建基础样式**

```css
/* styles.css */
:root {
  --bg-primary: #1a1a2e;
  --bg-secondary: #16213e;
  --accent: #e94560;
  --text: #eaeaea;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: var(--bg-primary);
  color: var(--text);
  min-height: 100vh;
}
```

- [ ] **Step 4: Commit**

```bash
git add client/
git commit -m "feat(client): initialize TanStack Start frontend"
```

---

### Task 14: 基础 UI 布局与分层结构

**Files:**
- Create: `client/src/components/Layout.tsx`
- Create: `client/src/components/Layer.tsx`

- [ ] **Step 1: 创建分层组件**

```typescript
// src/components/Layer.tsx
import { ReactNode } from "react";

interface LayerProps {
  children: ReactNode;
  zIndex: number;
  visible?: boolean;
}

export function Layer({ children, zIndex, visible = true }: LayerProps) {
  if (!visible) return null;
  
  return (
    <div
      style={{
        position: "absolute",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        zIndex,
        pointerEvents: visible ? "auto" : "none",
      }}
    >
      {children}
    </div>
  );
}
```

- [ ] **Step 2: 创建主布局**

```typescript
// src/components/Layout.tsx
import { ReactNode } from "react";
import { BackgroundLayer } from "./BackgroundLayer";
import { CharacterLayer } from "./CharacterLayer";
import { ChatLayer } from "./ChatLayer";
import { ControlLayer } from "./ControlLayer";

interface LayoutProps {
  children?: ReactNode;
}

export function Layout({ children }: LayoutProps) {
  return (
    <div style={{ position: "relative", width: "100vw", height: "100vh", overflow: "hidden" }}>
      {/* Layer 1: 背景 */}
      <BackgroundLayer />
      
      {/* Layer 2: 角色 */}
      <CharacterLayer />
      
      {/* Layer 3: 对话 */}
      <ChatLayer />
      
      {/* Layer 4: 控制 */}
      <ControlLayer />
      
      {children}
    </div>
  );
}
```

- [ ] **Step 3: 创建各层占位组件**

```typescript
// src/components/BackgroundLayer.tsx
export function BackgroundLayer() {
  return (
    <div
      style={{
        background: "linear-gradient(to bottom, #1a1a2e, #16213e)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <div style={{ opacity: 0.3, fontSize: "14px", color: "#666" }}>
        [GIF/视频背景 - MVP 暂为占位]
      </div>
    </div>
  );
}
```

```typescript
// src/components/CharacterLayer.tsx
export function CharacterLayer() {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        flexDirection: "column",
      }}
    >
      <div
        style={{
          width: "120px",
          height: "120px",
          borderRadius: "50%",
          background: "#2a2a4e",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: "12px",
          color: "#666",
        }}
      >
        [角色占位]
      </div>
    </div>
  );
}
```

```typescript
// src/components/ControlLayer.tsx
export function ControlLayer() {
  return (
    <div
      style={{
        position: "fixed",
        top: "20px",
        right: "20px",
        display: "flex",
        gap: "12px",
      }}
    >
      <button style={{ padding: "8px 16px", borderRadius: "8px", border: "none", background: "#2a2a4e", color: "#fff", cursor: "pointer" }}>
        ⚙️ 设置
      </button>
    </div>
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add client/src/components/
git commit -m "feat(client): add layered UI components"
```

---

### Task 15: lofi 音乐播放组件

**Files:**
- Create: `client/src/components/Player.tsx`
- Create: `client/src/lib/audio.ts`

- [ ] **Step 1: 创建音频管理器**

```typescript
// src/lib/audio.ts
type PlayState = "playing" | "paused" | "stopped";

interface AudioManager {
  play(url: string): void;
  pause(): void;
  resume(): void;
  stop(): void;
  setVolume(volume: number): void;
  getState(): PlayState;
  onStateChange(callback: (state: PlayState) => void): void;
}

export function createAudioManager(): AudioManager {
  let audio: HTMLAudioElement | null = null;
  let state: PlayState = "stopped";
  let stateCallbacks: ((state: PlayState) => void)[] = [];

  const notifyState = () => {
    stateCallbacks.forEach((cb) => cb(state));
  };

  return {
    play(url: string) {
      if (audio) {
        audio.pause();
      }
      audio = new Audio(url);
      audio.loop = true;
      audio.play().then(() => {
        state = "playing";
        notifyState();
      });
    },
    pause() {
      audio?.pause();
      state = "paused";
      notifyState();
    },
    resume() {
      audio?.play();
      state = "playing";
      notifyState();
    },
    stop() {
      if (audio) {
        audio.pause();
        audio.currentTime = 0;
      }
      state = "stopped";
      notifyState();
    },
    setVolume(volume: number) {
      if (audio) {
        audio.volume = Math.max(0, Math.min(1, volume));
      }
    },
    getState() {
      return state;
    },
    onStateChange(callback: (state: PlayState) => void) {
      stateCallbacks.push(callback);
    },
  };
}

export const audioManager = createAudioManager();
```

- [ ] **Step 2: 创建播放器组件**

```typescript
// src/components/Player.tsx
import { useState, useEffect } from "react";
import { audioManager } from "~/lib/audio";

// MVP: 使用免费电台
const DEFAULT_STATIONS = [
  { name: "Lofi Girl", url: "https://play.streamafrica.net/lofiradio" },
  { name: "Chillhop", url: "https://streams.fluxfm.de/Chillhop/mp3-128" },
];

export function Player() {
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentStation, setCurrentStation] = useState(DEFAULT_STATIONS[0]);

  useEffect(() => {
    audioManager.onStateChange((state) => {
      setIsPlaying(state === "playing");
    });
  }, []);

  const togglePlay = () => {
    if (isPlaying) {
      audioManager.pause();
    } else {
      audioManager.play(currentStation.url);
    }
  };

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: "12px",
        padding: "8px 16px",
        background: "rgba(0,0,0,0.3)",
        borderRadius: "24px",
        backdropFilter: "blur(8px)",
      }}
    >
      <button
        onClick={togglePlay}
        style={{
          width: "32px",
          height: "32px",
          borderRadius: "50%",
          border: "none",
          background: "#e94560",
          color: "#fff",
          cursor: "pointer",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        {isPlaying ? "⏸" : "▶"}
      </button>
      
      <span style={{ fontSize: "14px", minWidth: "80px" }}>
        {currentStation.name}
      </span>
      
      <span style={{ fontSize: "12px", opacity: 0.6 }}>
        {isPlaying ? "正在播放" : "已暂停"}
      </span>
    </div>
  );
}
```

- [ ] **Step 3: 将 Player 添加到 ControlLayer**

```typescript
// src/components/ControlLayer.tsx
import { Player } from "./Player";

export function ControlLayer() {
  return (
    <div
      style={{
        position: "fixed",
        top: "20px",
        left: "20px",
        right: "20px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
      }}
    >
      <Player />
      
      <button style={{ padding: "8px 16px", borderRadius: "8px", border: "none", background: "#2a2a4e", color: "#fff", cursor: "pointer" }}>
        ⚙️ 设置
      </button>
    </div>
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add client/src/components/Player.tsx client/src/lib/audio.ts
git commit -m "feat(client): add lofi music player component"
```

---

### Task 16: 对话 UI 组件

**Files:**
- Create: `client/src/components/Chat.tsx`
- Create: `client/src/hooks/useChat.ts`

- [ ] **Step 1: 创建聊天 Hook**

```typescript
// src/hooks/useChat.ts
import { useState, useCallback } from "react";

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
}

interface UseChat {
  messages: Message[];
  isLoading: boolean;
  sendMessage: (content: string) => Promise<void>;
  clearMessages: () => void;
}

export function useChat(): UseChat {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  const sendMessage = useCallback(async (content: string) => {
    const userMessage: Message = {
      id: Date.now().toString(),
      role: "user",
      content,
    };
    
    setMessages((prev) => [...prev, userMessage]);
    setIsLoading(true);

    try {
      // TODO: 替换为实际的 WebSocket 调用
      const response = await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ messages: [...messages, userMessage] }),
      });
      
      const data = await response.json();
      
      const assistantMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: "assistant",
        content: data.content || "（MVP 暂未连接后端）",
      };
      
      setMessages((prev) => [...prev, assistantMessage]);
    } catch (error) {
      console.error("Chat error:", error);
    } finally {
      setIsLoading(false);
    }
  }, [messages]);

  const clearMessages = useCallback(() => {
    setMessages([]);
  }, []);

  return { messages, isLoading, sendMessage, clearMessages };
}
```

- [ ] **Step 2: 创建聊天组件**

```typescript
// src/components/Chat.tsx
import { useState } from "react";
import { useChat } from "~/hooks/useChat";

export function Chat() {
  const [input, setInput] = useState("");
  const { messages, isLoading, sendMessage } = useChat();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;
    
    const content = input;
    setInput("");
    await sendMessage(content);
  };

  return (
    <div
      style={{
        position: "fixed",
        bottom: "20px",
        left: "20px",
        right: "20px",
        maxWidth: "600px",
        margin: "0 auto",
        display: "flex",
        flexDirection: "column",
        gap: "12px",
      }}
    >
      {/* 消息列表 */}
      <div
        style={{
          maxHeight: "300px",
          overflowY: "auto",
          display: "flex",
          flexDirection: "column",
          gap: "8px",
          padding: "12px",
          background: "rgba(0,0,0,0.3)",
          borderRadius: "12px",
          backdropFilter: "blur(8px)",
        }}
      >
        {messages.length === 0 && (
          <div style={{ textAlign: "center", opacity: 0.5, fontSize: "14px" }}>
            开始对话吧...
          </div>
        )}
        
        {messages.map((msg) => (
          <div
            key={msg.id}
            style={{
              padding: "8px 12px",
              borderRadius: "8px",
              background: msg.role === "user" ? "#e94560" : "#2a2a4e",
              alignSelf: msg.role === "user" ? "flex-end" : "flex-start",
              maxWidth: "80%",
              fontSize: "14px",
            }}
          >
            {msg.content}
          </div>
        ))}
        
        {isLoading && (
          <div style={{ opacity: 0.5, fontSize: "14px" }}>
            正在输入...
          </div>
        )}
      </div>

      {/* 输入框 */}
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="发送消息..."
          disabled={isLoading}
          style={{
            width: "100%",
            padding: "12px 16px",
            borderRadius: "24px",
            border: "none",
            background: "rgba(0,0,0,0.5)",
            color: "#fff",
            fontSize: "14px",
            outline: "none",
          }}
        />
      </form>
    </div>
  );
}
```

- [ ] **Step 3: 更新 Layout 添加 ChatLayer**

```typescript
// src/components/ChatLayer.tsx
import { Chat } from "./Chat";

export function ChatLayer() {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "flex-end",
        justifyContent: "center",
        paddingBottom: "20px",
      }}
    >
      <Chat />
    </div>
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add client/src/components/Chat.tsx client/src/hooks/useChat.ts
git commit -m "feat(client): add chat UI component"
```

---

### Task 17: WebSocket 集成

**Files:**
- Create: `client/src/lib/websocket.ts`
- Modify: `client/src/hooks/useChat.ts`

- [ ] **Step 1: 创建 WebSocket 连接管理器**

```typescript
// src/lib/websocket.ts
type MessageHandler = (data: any) => void;

interface WSConnection {
  connect(url: string, token?: string): void;
  disconnect(): void;
  send(type: string, payload: any): void;
  on(type: string, handler: MessageHandler): void;
  off(type: string, handler: MessageHandler): void;
}

export function createWSConnection(): WSConnection {
  let ws: WebSocket | null = null;
  let handlers: Map<string, MessageHandler[]> = new Map();
  let reconnectAttempts = 0;
  const maxReconnectAttempts = 5;

  const connect = (url: string, token?: string) => {
    ws = new WebSocket(url);

    ws.onopen = () => {
      console.log("WebSocket connected");
      reconnectAttempts = 0;
      
      // 发送认证
      if (token) {
        ws?.send(JSON.stringify({ type: "auth", payload: { token } }));
      }
    };

    ws.onmessage = (event) => {
      try {
        const { type, payload } = JSON.parse(event.data);
        const typeHandlers = handlers.get(type) || [];
        typeHandlers.forEach((handler) => handler(payload));
      } catch (error) {
        console.error("WS message error:", error);
      }
    };

    ws.onclose = () => {
      console.log("WebSocket closed");
      // 自动重连
      if (reconnectAttempts < maxReconnectAttempts) {
        reconnectAttempts++;
        setTimeout(() => connect(url, token), 1000 * reconnectAttempts);
      }
    };

    ws.onerror = (error) => {
      console.error("WebSocket error:", error);
    };
  };

  const disconnect = () => {
    if (ws) {
      ws.close();
      ws = null;
    }
  };

  const send = (type: string, payload: any) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type, payload }));
    }
  };

  const on = (type: string, handler: MessageHandler) => {
    const typeHandlers = handlers.get(type) || [];
    typeHandlers.push(handler);
    handlers.set(type, typeHandlers);
  };

  const off = (type: string, handler: MessageHandler) => {
    const typeHandlers = handlers.get(type) || [];
    const filtered = typeHandlers.filter((h) => h !== handler);
    handlers.set(type, filtered);
  };

  return { connect, disconnect, send, on, off };
}

export const ws = createWSConnection();
```

- [ ] **Step 2: 更新 useChat 使用 WebSocket**

```typescript
// src/hooks/useChat.ts
import { useState, useCallback, useEffect } from "react";
import { ws } from "~/lib/websocket";

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
}

interface UseChat {
  messages: Message[];
  isLoading: boolean;
  sendMessage: (content: string) => Promise<void>;
  clearMessages: () => void;
}

export function useChat(): UseChat {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  // 监听 AI 响应
  useEffect(() => {
    const handleResponse = (payload: any) => {
      const assistantMessage: Message = {
        id: Date.now().toString(),
        role: "assistant",
        content: payload.content,
      };
      setMessages((prev) => [...prev, assistantMessage]);
      setIsLoading(false);
    };

    ws.on("chat_response", handleResponse);

    return () => {
      ws.off("chat_response", handleResponse);
    };
  }, []);

  const sendMessage = useCallback(async (content: string) => {
    const userMessage: Message = {
      id: Date.now().toString(),
      role: "user",
      content,
    };

    setMessages((prev) => [...prev, userMessage]);
    setIsLoading(true);

    // 通过 WebSocket 发送
    ws.send("chat_request", {
      messages: [...messages, userMessage],
    });
  }, [messages]);

  const clearMessages = useCallback(() => {
    setMessages([]);
  }, []);

  return { messages, isLoading, sendMessage, clearMessages };
}
```

- [ ] **Step 3: Commit**

```bash
git add client/src/lib/websocket.ts
git commit -m "feat(client): add WebSocket integration"
```

---

## 执行方式选择

**Plan complete and saved to `docs/superpowers/plans/2026-04-15-loom-mvp-plan.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - 我为每个任务派遣一个子代理，在任务之间进行审查，快速迭代

**2. Inline Execution** - 在此会话中使用 executing-plans 执行任务，带有审查检查点

**你选择哪种方式？**
