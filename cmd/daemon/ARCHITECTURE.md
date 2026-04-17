# Loom Daemon 架构文档

## 概述

Loom Daemon 是一个生产级别的 Go 服务，负责在用户本地机器上管理 AI 运行时（Claude Code / OpenCode），并通过 WebSocket 与云端后端通信。

## 核心组件

### 1. Runtime 层 (`internal/runtime/`)

**职责**: 抽象和管理不同的 AI 运行时

- **接口** (`runtime.go`): 定义了统一的操作接口
  - `Chat()`: 发送聊天消息并接收流式响应
  - `ExecuteTool()`: 执行特定工具
  - `ListTools()`: 列出可用工具
  - `HealthCheck()`: 健康检查

- **Claude Runtime** (`claude.go`): Claude Code CLI 的封装
  - 解析 MCP (Model Context Protocol) 格式的工具调用
  - 支持 6 种工具: Bash, Read, Write, Edit, Glob, Grep
  - 流式输出解析 (text, thinking, tool_call, tool_result, done, error)

- **OpenCode Runtime** (`opencode.go`): OpenCode CLI 的封装
  - 预留结构，待实现

### 2. Session 管理层 (`internal/session/`)

**职责**: 管理运行时会话的生命周期

- **Session 结构**:
  ```go
  type Session struct {
      ID           string
      PID          int
      RuntimeType  string
      Status       Status  // starting/running/thinking/tool_call/stopping/stopped/error
      runtime      runtime.Runtime
      process      *os.Process
      cancel       context.CancelFunc
  }
  ```

- **Manager 功能**:
  - `Spawn()`: 创建新会话
  - `Stop()`: 停止会话
  - `Chat()`: 向会话发送消息
  - `Get()/List()`: 查询会话
  - 心跳检测和超时管理

### 3. HTTP 控制服务器 (`internal/daemon/`)

**职责**: 提供本地 HTTP API 供外部调用

**端点**:
- `GET /health`: 健康检查
- `GET /v1/status`: 获取守护进程状态
- `GET /v1/sessions`: 列出所有会话
- `POST /v1/sessions`: 创建新会话
- `GET /v1/sessions/:id`: 获取会话详情
- `DELETE /v1/sessions/:id`: 停止会话
- `POST /v1/chat`: 发送聊天消息 (SSE 流式)
- `POST /v1/sessions/:id/tools`: 执行工具

### 4. WebSocket 客户端 (`internal/ws/`)

**职责**: 通过 Centrifugo 与云端后端通信

**功能**:
- 使用 `centrifuge-go` SDK
- 自动重连机制 (指数退避)
- 心跳保活 (30秒间隔)
- 频道订阅管理
- 消息处理器注册

**频道**:
- `daemon:{deviceId}`: 接收后端命令
- `user:{deviceId}`: 发送事件到后端

### 5. 消息路由层 (`internal/messaging/`)

**职责**: 处理前后端消息格式转换和路由

**Router** (`router.go`):
- 注册消息处理器
- 路由到相应的处理函数
- 异步工具执行

**Formatter** (`formatter.go`):
- `ToBackendMessage()`: Runtime 事件 → Backend 消息
- `FromBackendMessage()`: Backend 消息 → Runtime 请求

## 数据流

### 1. 聊天请求流

```
User (Browser)
    ↓ HTTP/WebSocket
Backend (Bun)
    ↓ WebSocket (Centrifugo)
Daemon (Go)
    ↓ Function Call
Runtime (Claude Code)
    ↓ Stream
Claude API
    ↓ Stream
Runtime
    ↓ StreamEvent
Daemon
    ↓ WebSocket
Backend
    ↓ WebSocket
Client
```

### 2. 工具执行流

```
Claude API 决定使用工具
    ↓ MCP 格式输出
Runtime (解析 MCP)
    ↓ ToolCall StreamEvent
Daemon
    ↓ ExecuteTool()
本地执行 (Bash/Read/Write/etc)
    ↓ ToolResult StreamEvent
Daemon
    ↓ WebSocket
Backend
```

## 并发模型

### Goroutine 结构

```
main
├── Start Command
│   ├── Session Manager
│   │   └── heartbeatLoop (goroutine)
│   ├── Control Server
│   │   └── http.Serve (goroutine)
│   └── WebSocket Client
│       ├── centrifuge.Client (内部 goroutines)
│       ├── heartbeatLoop (goroutine)
│       └── message handlers
└── Signal Handler
```

### 同步机制

- **Session Manager**: `sync.RWMutex` 保护 sessions map
- **WebSocket Client**: `sync.RWMutex` 保护连接状态和 handlers
- **Session**: `sync.RWMutex` 保护状态变更

## 错误处理策略

1. **连接错误**: 自动重连，指数退避
2. **运行时错误**: 返回 error StreamEvent，标记会话状态为 error
3. **超时**: Context 取消，优雅关闭
4. **进程终止**: SIGTERM -> 等待 5s -> SIGKILL (强制模式)

## 安全考虑

1. **本地绑定**: HTTP 服务器默认绑定 127.0.0.1
2. **设备认证**: 通过 deviceId 和 apiKey 验证
3. **命令注入**: 工具参数使用结构化数据，避免 shell 注入
4. **文件访问**: 工具执行限制在 WorkingDir 内

## 扩展性

### 添加新的 Runtime

1. 实现 `runtime.Runtime` 接口
2. 在 `runtime.Factory()` 中注册
3. 添加相应的配置选项

### 添加新的工具

1. 在 `ClaudeRuntime` 的 `ListTools()` 中添加定义
2. 在 `ExecuteTool()` 中实现处理逻辑
3. 更新文档

### 添加新的消息处理器

1. 在 `Router.registerHandlers()` 中注册
2. 实现处理函数
3. 前后端约定消息格式

## 监控指标

可通过 `/v1/status` 获取:
- 守护进程版本和运行时间
- 活跃会话数
- 运行时可用性
- WebSocket 连接状态

## 配置

### 命令行参数

```bash
./loomd start \
  --centrifugo ws://localhost:8000 \
  --device-id my-device \
  --listen 127.0.0.1:9999 \
  --runtime claude
```

### 环境变量

- `LOOM_DEVICE_ID`: 设备ID
- `LOOM_BACKEND_URL`: 后端URL
- `LOOM_CENTRIFUGO_URL`: Centrifugo URL
- `LOOM_RUNTIME`: 默认运行时
- `LOOM_CONTROL_ADDR`: 控制地址

## 测试策略

### 单元测试
- Runtime 接口实现
- Session 状态管理
- 消息格式化

### 集成测试
- HTTP API 端点
- 会话生命周期
- WebSocket 连接

### 端到端测试
- 完整聊天流程
- 工具执行链
- 重连机制
