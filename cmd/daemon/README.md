# Loom Daemon

Loom Daemon 是运行在用户机器上的本地服务，负责管理 AI 运行时会话并与 Loom 后端通信。

## 功能特性

- **多运行时支持**: 支持 Claude Code 和 OpenCode 运行时
- **会话管理**: 完整的会话生命周期管理（创建、运行、停止）
- **工具执行**: 支持 Bash、Read、Write、Edit、Glob、Grep 等工具
- **WebSocket 通信**: 通过 Centrifugo 与后端实时通信
- **HTTP 控制接口**: 提供本地 HTTP API 用于管理和监控
- **流式响应**: 支持 SSE (Server-Sent Events) 流式输出

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Machine                            │
│                                                                 │
│  ┌─────────────┐      ┌──────────────┐      ┌─────────────┐    │
│  │   Runtime   │◀────▶│    Daemon    │◀────▶│ Centrifugo  │    │
│  │ Claude Code │      │  (Go CLI)    │      │   Client    │    │
│  │  OpenCode   │      │              │      │             │    │
│  └─────────────┘      └──────┬───────┘      └─────────────┘    │
│                              │                                  │
│                              ▼                                  │
│                       ┌──────────────┐                         │
│                       │ HTTP Control │                         │
│                       │   Server     │                         │
│                       └──────────────┘                         │
└─────────────────────────────────────────────────────────────────┘
                                        ▲
                                        │ WebSocket
                                        ▼
                               ┌─────────────────┐
                               │  Loom Backend   │
                               │  (Bun + TS)     │
                               └─────────────────┘
```

## 快速开始

### 构建

```bash
cd cmd/daemon
go build -o loomd .
```

### 运行

```bash
# 基本运行
./loomd start

# 指定配置
./loomd start \
  --centrifugo ws://localhost:8000 \
  --device-id my-device \
  --listen 127.0.0.1:9999 \
  --runtime claude

# 查看帮助
./loomd --help
./loomd start --help
```

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `LOOM_DEVICE_ID` | 设备唯一标识 | 自动生成 |
| `LOOM_BACKEND_URL` | 后端 WebSocket URL | `ws://localhost:8000` |
| `LOOM_CENTRIFUGO_URL` | Centrifugo WebSocket URL | `ws://localhost:8000` |
| `LOOM_RUNTIME` | 默认运行时类型 | `claude` |
| `LOOM_CONTROL_ADDR` | 控制服务器监听地址 | `127.0.0.1:0` |

## HTTP API

### 健康检查

```bash
GET /health
```

响应：
```json
{
  "status": "healthy",
  "version": "0.1.0",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### 获取状态

```bash
GET /v1/status
```

响应：
```json
{
  "version": "0.1.0",
  "started_at": "2024-01-15T10:00:00Z",
  "uptime": "30m0s",
  "sessions": 2,
  "runtime": {
    "type": "claude",
    "available": true
  }
}
```

### 列出会话

```bash
GET /v1/sessions
```

响应：
```json
{
  "sessions": [
    {
      "id": "sess_1234567890",
      "runtime_type": "claude",
      "status": "running",
      "working_dir": "/home/user/project",
      "started_at": "2024-01-15T10:00:00Z",
      "last_activity": "2024-01-15T10:25:00Z"
    }
  ]
}
```

### 创建会话

```bash
POST /v1/sessions
Content-Type: application/json

{
  "runtime_type": "claude",
  "working_dir": "/home/user/project",
  "env_vars": {
    "KEY": "value"
  }
}
```

### 获取会话

```bash
GET /v1/sessions/:id
```

### 停止会话

```bash
DELETE /v1/sessions/:id
```

### 发送聊天消息（SSE 流式）

```bash
POST /v1/chat
Content-Type: application/json
Accept: text/event-stream

{
  "session_id": "sess_1234567890",
  "messages": [
    {
      "role": "user",
      "content": "Hello!"
    }
  ]
}
```

SSE 事件：
```
event: message
data: {"type":"text","content":"Hello"}

event: message
data: {"type":"thinking","thinking":true}

event: message
data: {"type":"tool_call","tool":{"name":"Bash","input":{"command":"ls"}}}

event: message
data: {"type":"done"}
```

### 执行工具

```bash
POST /v1/sessions/:id/tools
Content-Type: application/json

{
  "tool": "Bash",
  "input": {
    "command": "ls -la"
  }
}
```

## WebSocket 消息协议

### 频道

- `daemon:{deviceId}` - 后端发送命令到 Daemon
- `user:{deviceId}` - Daemon 发送事件到后端

### 消息类型

#### Daemon 接收的消息

**聊天消息** (`chat:message`):
```json
{
  "type": "chat:message",
  "session_id": "sess_123",
  "messages": [
    {"role": "user", "content": "Hello"}
  ]
}
```

**创建会话** (`session:create`):
```json
{
  "type": "session:create",
  "runtime_type": "claude",
  "working_dir": "/path/to/dir"
}
```

**停止会话** (`session:stop`):
```json
{
  "type": "session:stop",
  "session_id": "sess_123",
  "force": false
}
```

**执行工具** (`tool:execute`):
```json
{
  "type": "tool:execute",
  "session_id": "sess_123",
  "tool_name": "Bash",
  "input": {"command": "ls"}
}
```

**心跳** (`system:ping`):
```json
{
  "type": "system:ping"
}
```

#### Daemon 发送的消息

**流式响应** (`chat:text`):
```json
{
  "type": "chat:text",
  "session_id": "sess_123",
  "data": {"content": "Hello!"},
  "timestamp": 1705312200
}
```

**工具调用** (`chat:tool_call`):
```json
{
  "type": "chat:tool_call",
  "session_id": "sess_123",
  "data": {
    "tool_name": "Bash",
    "tool_input": {"command": "ls"}
  }
}
```

**工具结果** (`chat:tool_result`):
```json
{
  "type": "chat:tool_result",
  "session_id": "sess_123",
  "data": {
    "output": "file1.txt file2.txt",
    "error": ""
  }
}
```

**完成** (`chat:done`):
```json
{
  "type": "chat:done",
  "session_id": "sess_123",
  "data": {"done": true}
}
```

## 运行时工具

### Bash

执行 shell 命令。

```json
{
  "tool": "Bash",
  "input": {
    "command": "ls -la",
    "working_dir": "/optional/path",
    "timeout": 30
  }
}
```

### Read

读取文件内容。

```json
{
  "tool": "Read",
  "input": {
    "file_path": "/path/to/file.txt",
    "limit": 100,
    "offset": 0
  }
}
```

### Write

写入文件内容。

```json
{
  "tool": "Write",
  "input": {
    "file_path": "/path/to/file.txt",
    "content": "Hello World"
  }
}
```

### Edit

编辑文件内容（查找替换）。

```json
{
  "tool": "Edit",
  "input": {
    "file_path": "/path/to/file.txt",
    "old_string": "Hello",
    "new_string": "Hi"
  }
}
```

### Glob

查找匹配模式的文件。

```json
{
  "tool": "Glob",
  "input": {
    "pattern": "**/*.go"
  }
}
```

### Grep

在文件中搜索内容。

```json
{
  "tool": "Grep",
  "input": {
    "pattern": "func main",
    "path": "/path/to/search",
    "output_mode": "content"
  }
}
```

## 开发

### 运行测试

```bash
cd cmd/daemon
go test ./...

# 运行基准测试
go test -bench=. ./...

# 带覆盖率
go test -cover ./...
```

### 代码结构

```
cmd/daemon/
├── main.go                      # 入口
├── cmd/                         # 命令定义
│   ├── root.go
│   ├── start.go
│   ├── stop.go
│   └── status.go
├── internal/
│   ├── config/                  # 配置管理
│   ├── runtime/                 # 运行时接口和实现
│   │   ├── runtime.go           # 接口定义
│   │   ├── claude.go            # Claude 运行时
│   │   └── opencode.go          # OpenCode 运行时
│   ├── session/                 # 会话管理
│   │   └── manager.go
│   ├── daemon/                  # HTTP 控制服务器
│   │   └── server.go
│   ├── ws/                      # WebSocket 客户端
│   │   └── client.go
│   ├── messaging/               # 消息路由和格式化
│   │   ├── router.go
│   │   └── formatter.go
│   └── integration/             # 集成测试
│       └── daemon_test.go
└── pkg/api/                     # 共享 API 类型
    └── types.go
```

## 许可证

MIT
