# Loom 升级改造计划

基于对 Happy CLI 架构的深入分析，制定 Loom 的升级改造计划。

## 当前状态分析

### Loom 现有架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      Loom Architecture                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐                    ┌─────────────────────────┐ │
│  │   Client    │◀── HTTP/WS ───────▶│        Backend          │ │
│  │  (React)    │                    │  (TanStack Start + Bun) │ │
│  └─────────────┘                    └───────────┬─────────────┘ │
│                                                 │               │
│                                        ┌────────▼────────┐      │
│                                        │    Centrifugo   │      │
│                                        │    (WebSocket)  │      │
│                                        └────────┬────────┘      │
└─────────────────────────────────┲───────────────┼───────────────┘
                                  ▼               │
                          ┌──────────────┐        │
                          │   Daemon     │◀───────┘
                          │  (Go CLI)    │ WebSocket
                          └──────┬───────┘
                                 │
                          ┌──────▼────────┐
                          │  Local AI     │
                          │ Claude Code   │
                          └───────────────┘
```

### 现有问题

1. **Daemon 过于简单**
   - 单次运行，无常驻能力
   - 无会话恢复机制
   - 无离线重连能力
   - 无进程生命周期管理

2. **通信协议简陋**
   - 简单 JSON，无版本控制
   - 无端到端加密
   - 无消息信封格式
   - 无 RPC 机制

3. **会话管理缺失**
   - 无 session ID 跟踪
   - 无 --resume 支持
   - 无本地/远程模式切换
   - 无思考状态跟踪

4. **架构耦合**
   - Backend 依赖 Centrifugo
   - Daemon 直接连接 Backend
   - 无清晰的模块边界

## 升级目标

### Phase 1: Daemon 架构升级（高优先级）

将 Go Daemon 升级为常驻服务，参考 Happy 的 Daemon 设计：

```
┌─────────────────────────────────────────────────────────────────┐
│                      Phase 1: Upgraded Daemon                   │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Loomd (Go)                            │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │   │
│  │  │   Session   │  │   Control   │  │   WebSocket     │   │   │
│  │  │   Manager   │  │   Server    │  │    Client       │   │   │
│  │  │             │  │ (Unix Sock) │  │                 │   │   │
│  │  │ - Spawn     │  │             │  │ - Connect       │   │   │
│  │  │ - Track PID │  │ - List      │  │ - Reconnect     │   │   │
│  │  │ - Resume    │  │ - Stop      │  │ - Heartbeat     │   │   │
│  │  │ - Cleanup   │  │ - Status    │  │                 │   │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────┘   │   │
│  │                                                            │   │
│  │  ┌─────────────────────────────────────────────────────┐  │   │
│  │  │                 State Management                      │  │   │
│  │  │  - PID to Session Map  - Heartbeat Timer             │  │   │
│  │  │  - Machine Metadata    - Version Check               │  │   │
│  │  └─────────────────────────────────────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

#### 具体任务

1. **Daemon 常驻化**
   ```go
   // daemon/run.go
   type Daemon struct {
       pidToSession map[int]*TrackedSession
       controlServer *ControlServer
       wsClient *WebSocketClient
       machineID string
       version string
   }
   
   func (d *Daemon) Start() error {
       // 1. 获取独占锁
       // 2. 启动控制服务器
       // 3. 连接 Backend
       // 4. 注册机器
       // 5. 启动心跳
   }
   ```

2. **会话管理**
   ```go
   type TrackedSession struct {
       SessionID string
       PID int
       Path string
       Runtime string  // claude/opencode
       StartedAt time.Time
       Status string   // running/stopped
   }
   
   func (d *Daemon) SpawnSession(opts SpawnOptions) (*TrackedSession, error)
   func (d *Daemon) StopSession(sessionID string) error
   func (d *Daemon) ResumeSession(sessionID string) error
   ```

3. **本地控制接口**
   ```go
   // 通过 Unix Socket 或 HTTP 提供本地 API
   type ControlServer interface {
       ListSessions() ([]*SessionInfo, error)
       StopSession(id string) error
       GetStatus() (*DaemonStatus, error)
       RequestShutdown() error
   }
   ```

4. **状态持久化**
   ```go
   type DaemonState struct {
       PID int
       HTTPPort int
       StartTime time.Time
       Version string
       LastHeartbeat time.Time
   }
   
   func WriteDaemonState(state *DaemonState) error
   func ReadDaemonState() (*DaemonState, error)
   ```

### Phase 2: 通信协议升级（高优先级）

参考 Happy Wire 设计统一消息协议：

```typescript
// pkg/protocol/messages.go
package protocol

type MessageRole string
const (
    RoleUser  MessageRole = "user"
    RoleAgent MessageRole = "agent"
)

type SessionEvent struct {
    Type string `json:"t"`
    // text
    Text string `json:"text,omitempty"`
    // tool-call
    CallID string `json:"call,omitempty"`
    Name   string `json:"name,omitempty"`
    Input  any    `json:"args,omitempty"`
    // file
    FileRef  string `json:"ref,omitempty"`
    FileName string `json:"name,omitempty"`
    Size     int64  `json:"size,omitempty"`
}

type SessionEnvelope struct {
    ID     string       `json:"id"`
    Time   int64        `json:"time"`
    Role   MessageRole  `json:"role"`
    Turn   string       `json:"turn,omitempty"`
    Event  SessionEvent `json:"ev"`
}
```

#### 具体任务

1. **消息信封格式**
   - 统一的 envelope 结构
   - CUID2 ID 生成
   - 时间戳和角色标记

2. **RPC 机制**
   ```go
   type RPCHandler interface {
       Handle(method string, params []byte) ([]byte, error)
   }
   
   type RPCHandlerManager struct {
       handlers map[string]RPCHandler
   }
   ```

3. **加密支持（可选）**
   ```go
   type Encryption interface {
       Encrypt(data []byte) ([]byte, error)
       Decrypt(data []byte) ([]byte, error)
   }
   ```

### Phase 3: Runtime 集成升级（中优先级）

深度集成 Claude Code，支持更多功能：

```
┌─────────────────────────────────────────────────────────┐
│              Runtime Adapter Pattern                     │
├─────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │   Claude     │  │  OpenCode    │  │   Future     │  │
│  │   Adapter    │  │   Adapter    │  │   Adapters   │  │
│  │              │  │              │  │              │  │
│  │ - Spawn      │  │ - Spawn      │  │ - Spawn      │  │
│  │ - Hooks      │  │ - Hooks      │  │ - Hooks      │  │
│  │ - Resume     │  │ - Resume     │  │ - Resume     │  │
│  │ - Thinking   │  │ - Thinking   │  │ - Thinking   │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  │
│         └─────────────────┼─────────────────┘          │
│                           ▼                            │
│                  ┌─────────────────┐                   │
│                  │  Runtime Manager │                   │
│                  └─────────────────┘                   │
└─────────────────────────────────────────────────────────┘
```

#### 具体任务

1. **Claude Adapter 增强**
   ```go
   type ClaudeAdapter struct {
       launcherPath string
       hookServerPort int
   }
   
   func (a *ClaudeAdapter) Spawn(opts SpawnOptions) (*Process, error)
   func (a *ClaudeAdapter) GetSessionID(path string) (string, error)
   func (a *ClaudeAdapter) Resume(sessionID string) error
   func (a *ClaudeAdapter) TrackThinking(pid int, callback func(bool))
   ```

2. **Session Scanner**
   ```go
   type SessionScanner struct {
       watchPath string
       onSessionFound func(sessionID string)
   }
   
   func (s *SessionScanner) Start() error
   func (s *SessionScanner) Stop() error
   ```

3. **Hook Server**
   ```go
   type HookServer struct {
       port int
       onSessionHook func(sessionID string, data SessionData)
   }
   ```

### Phase 4: Session 管理升级（中优先级）

实现完整的会话生命周期管理：

```go
// pkg/session/manager.go
package session

type Manager struct {
   activeSessions map[string]*Session
   storage SessionStorage
}

type Session struct {
   ID string
   Path string
   Runtime string
   Mode SessionMode  // local/remote
   Status SessionStatus
   CreatedAt time.Time
   LastActiveAt time.Time
}

func (m *Manager) Create(opts CreateOptions) (*Session, error)
func (m *Manager) Get(id string) (*Session, error)
func (m *Manager) List() ([]*Session, error)
func (m *Manager) Resume(id string) error
func (m *Manager) Archive(id string) error
```

### Phase 5: 双模式支持（低优先级）

支持本地/远程模式切换（类似 Happy）：

```
本地模式                    远程模式
┌─────────────┐            ┌─────────────┐
│  Terminal   │            │   Phone     │
│  Interactive│◄──────────►│   Remote    │
│  Mode       │  切换       │   Control   │
└─────────────┘            └─────────────┘
```

## 实施路线图

### Week 1-2: 基础架构
- [ ] 重构 Daemon 为常驻服务
- [ ] 实现控制服务器（Unix Socket）
- [ ] PID 跟踪和进程管理
- [ ] 状态持久化

### Week 3-4: 通信升级
- [ ] 设计消息协议（参考 Happy Wire）
- [ ] 实现 RPC 机制
- [ ] 升级 WebSocket 客户端
- [ ] 消息队列和重连机制

### Week 5-6: Runtime 集成
- [ ] 增强 Claude Adapter
- [ ] Session Scanner
- [ ] Hook Server
- [ ] 思考状态跟踪

### Week 7-8: Session 管理
- [ ] Session Manager
- [ ] 会话恢复（--resume）
- [ ] 会话历史存储
- [ ] 归档和清理

### Week 9-10: 测试和优化
- [ ] 集成测试
- [ ] 性能优化
- [ ] 错误处理
- [ ] 文档更新

## 技术选型

### 保持使用
- **Go**: Daemon 语言
- **Bun**: Backend 运行时
- **React**: 前端框架
- **Centrifugo**: WebSocket 服务器（或考虑替换）

### 引入新依赖
```go
// Go dependencies
go get github.com/paralleldrive/cuid2  // ID 生成
go get github.com/coder/websocket      // WebSocket 客户端
go get golang.org/x/crypto/nacl        // 加密（可选）
```

### 参考实现
- [Happy CLI](https://github.com/slopus/happy)
- [Happy Wire Protocol](https://github.com/slopus/happy/tree/main/packages/happy-wire)

## 风险和对策

| 风险 | 对策 |
|------|------|
| 架构变动大，可能引入 bug | 分阶段实施，每阶段充分测试 |
| 学习曲线陡峭 | 参考 Happy 实现，复用成熟模式 |
| 与现有代码不兼容 | 设计兼容层，逐步迁移 |
| 性能问题 | 压力测试，性能基准 |

## 预期收益

1. **稳定性**: Daemon 常驻服务，会话不丢失
2. **可恢复性**: 支持会话恢复和断线重连
3. **可扩展性**: 清晰的模块边界，易于添加新 Runtime
4. **安全性**: 可选的端到端加密
5. **功能丰富**: 思考状态、权限管理、Push 通知

## 下一步行动

1. 审查此计划，确定优先级
2. 创建 GitHub issues 跟踪任务
3. 从 Phase 1 开始实施
4. 建立测试基准
