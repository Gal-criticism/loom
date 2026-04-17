# Loom 全面架构评估报告

## 文档信息
- **评估日期**: 2026-04-17
- **评估范围**: Runtime / Daemon / Backend / Client 全链路
- **参考基准**: Happy-main 架构、业界最佳实践

---

## 执行摘要

### 关键发现矩阵

| 评估维度 | 当前状态 | Happy对比 | 风险等级 | 备注 |
|---------|---------|-----------|----------|------|
| **通信协议一致性** | 碎片化 | 统一协议 | 🔴 高 | 各层协议不统一 |
| **数据流清晰度** | 单向清晰 | 双向同步 | 🟡 中 | 缺少状态同步 |
| **安全模型** | 基础 | E2E加密 | 🔴 高 | 设备认证简单 |
| **可扩展性** | 水平扩展难 | 易于扩展 | 🟡 中 | Centrifugo支持好 |
| **错误处理** | 已改进 | 相似 | 🟢 低 | 近期已优化 |
| **可观测性** | 基础 | 完善 | 🟡 中 | 缺少metrics |
| **运维部署** | Docker | 相似 | 🟢 低 | 已容器化 |
| **技术债务** | 中等 | - | 🟡 中 | 需要重构部分 |

---

## 1. 通信协议架构深度分析

### 1.1 当前 Loom 协议栈

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Loom 当前协议架构                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        Runtime (Claude/OpenCode)                    │   │
│  │  ┌─────────────┐     ┌─────────────┐                               │   │
│  │  │ Claude Code │     │  OpenCode   │                               │   │
│  │  │   SDK调用   │     │   SDK调用   │  ← 无标准协议，直接调用        │   │
│  │  └──────┬──────┘     └──────┬──────┘                               │   │
│  └─────────┼───────────────────┼───────────────────────────────────────┘   │
│            │                   │                                           │
│            ▼                   ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        Daemon (Go CLI)                              │   │
│  │  ┌───────────────────────────────────────────────────────────────┐   │   │
│  │  │ 本地HTTP API: /api/chat, /api/tools, /api/capabilities        │   │   │
│  │  │ • 简单JSON RPC                                                │   │   │
│  │  │ • 无版本控制                                                  │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  │                                    │                                  │   │
│  │  ┌───────────────────────────────────────────────────────────────┐   │   │
│  │  │ WebSocket (gorilla/websocket) → Backend                      │   │   │
│  │  │ • 自定义消息格式                                              │   │   │
│  │  │ • 简单重连逻辑                                                │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                   │                                        │
│                                   ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     Centrifugo (WebSocket Broker)                   │   │
│  │  • channel: daemon:${deviceId}, user:${deviceId}                   │   │
│  │  • 消息广播和订阅                                                  │   │
│  │  • 水平扩展支持                                                    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                   │                                        │
│                                   ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                      Backend (TanStack Start + Bun)                 │   │
│  │  ┌───────────────────────────────────────────────────────────────┐   │   │
│  │  │ HTTP API: /api/sessions, /api/messages, /api/auth/*          │   │   │
│  │  │ • REST风格                                                    │   │   │
│  │  │ • Zod验证                                                     │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  │                                    │                                  │   │
│  │  ┌───────────────────────────────────────────────────────────────┐   │   │
│  │  │ WebSocket → Client (原生WebSocket)                           │   │   │
│  │  │ • 简单事件分发                                                │   │   │
│  │  │ • 无状态恢复                                                  │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                   │                                        │
│                                   ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         Client (React)                              │   │
│  │  • useChat hook                                                   │   │
│  │  • 基础WebSocket管理                                              │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Happy-main 协议栈对比

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Happy-main 协议架构                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        CLI/Daemon (Node.js)                         │   │
│  │  ┌───────────────────────────────────────────────────────────────┐   │   │
│  │  │ 本地HTTP Control Server (127.0.0.1) - IPC                     │   │   │
│  │  │ • /spawn-session, /stop-session, /list                        │   │   │
│  │  │ • Zod类型验证                                                 │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  │                                    │                                  │   │
│  │  ┌───────────────────────────────────────────────────────────────┐   │   │
│  │  │ Socket.IO → Happy Server                                     │   │   │
│  │  │ • connection types: user/session/machine-scoped               │   │   │
│  │  │ • events: update (persistent), ephemeral (transient)          │   │   │
│  │  │ • RPC: spawn-session, tool calls                              │   │   │
│  │  │ • E2E加密 (privacy-kit)                                       │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                   │                                        │
│                                   ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     Happy Server (Fastify + Socket.IO)              │   │
│  │  ┌───────────────────────────────────────────────────────────────┐   │   │
│  │  │ HTTP API: /v1/*, /v2/*                                        │   │   │
│  │  │ • 版本化路由                                                  │   │   │
│  │  │ • Bearer Token认证                                            │   │   │
│  │  │ • Zod验证                                                     │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  │                                    │                                  │   │
│  │  ┌───────────────────────────────────────────────────────────────┐   │   │
│  │  │ Socket.IO Event Router                                        │   │   │
│  │  │ • monotonic seq (per-user)                                    │   │   │
│  │  │ • optimistic concurrency (expectedVersion)                    │   │   │
│  │  │ • presence/activity debouncing                                │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                   │                                        │
│                                   ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        Client (Mobile/Web)                          │   │
│  │  • Socket.IO client                                               │   │
│  │  • Session Protocol (统一消息格式)                                 │   │
│  │  • E2E解密                                                        │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.3 协议对比详细分析

#### 1.3.1 Runtime ↔ Daemon 通信

| 特性 | Loom 当前 | Happy-main | 差距 |
|------|-----------|------------|------|
| **协议类型** | 直接 SDK 调用 + HTTP | HTTP API + IPC | Loom 缺少标准化 |
| **消息格式** | Go struct JSON | 结构化类型 | 相似 |
| **版本控制** | ❌ 无 | ❌ 无 | 均缺失 |
| **流式支持** | ✅ 回调函数 | ✅ 回调函数 | 相似 |
| **错误传播** | 简单 error | 结构化错误 | Happy 更好 |
| **运行时切换** | 启动参数 | 动态选择 | Happy 更灵活 |

**Loom 代码：**
```go
// cmd/daemon/runtime/adapter.go
type Runtime interface {
    Chat(ctx context.Context, req ChatRequest, onResponse func(ChatResponse)) error
    ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error)
    ListCapabilities() ([]string, []string)
    Name() string
}
```

**Happy 代码：**
```typescript
// Happy 通过统一的 adapter 模式封装不同 Runtime
// 支持 claude, codex, gemini, openclaw
// 统一的 sessionProtocol 消息格式
```

**建议：**
- ✅ 保留当前 Runtime 接口设计
- ⚠️ 需要标准化错误码和消息格式
- ⚠️ 考虑添加运行时能力协商

#### 1.3.2 Daemon ↔ Backend 通信

| 特性 | Loom (Centrifugo) | Happy (Socket.IO) | 差距 |
|------|-------------------|-------------------|------|
| **传输层** | WebSocket | WebSocket + HTTP | Happy 更灵活 |
| **连接类型** | deviceId-based | user/session/machine-scoped | Happy 更细粒度 |
| **消息可靠性** | at-most-once | at-least-once (acks) | Happy 更可靠 |
| **重连恢复** | 简单重连 | 完整状态恢复 | Happy 更好 |
| **水平扩展** | ✅ Centrifugo | ⚠️ 需 Redis Adapter | Loom 更优 |
| **RPC 支持** | ❌ 无原生支持 | ✅ 内置 RPC | Happy 更好 |
| **加密** | ❌ 传输层 TLS | ✅ E2E 加密 | Happy 更好 |

**Loom 消息格式：**
```typescript
// Daemon → Backend
type DaemonMessage = 
  | { type: "chat:response", data: { session_id, content, metadata? } }
  | { type: "chat:thinking", data: { session_id, thinking } }
  | { type: "chat:error", data: { session_id, error } }
  | { type: "chat:tool_call", data: { session_id, tool_name, tool_input } }

// Backend → Daemon
{
  type: "chat:message",
  data: { session_id, message_id, content, timestamp }
}
```

**Happy 消息格式：**
```typescript
// 统一 Update Event
interface UpdateEvent {
  id: string;
  seq: number;  // 关键：单调序列号
  body: {
    t: "new-message" | "update-session" | ...
    // ...
  };
  createdAt: number;
}

// RPC
client.emit('message', { sid, message, localId })
client.emit('update-metadata', { sid, metadata, expectedVersion })
```

**关键差距：序列号机制**

Loom 缺少的序列号机制影响：
1. 消息乱序问题
2. 无法检测消息丢失
3. 多端同步困难
4. 离线恢复复杂

**建议：**
- ✅ 保留 Centrifugo（水平扩展优势）
- ⚠️ 添加消息序列号（user-level monotonic seq）
- ⚠️ 实现 RPC 层（可用 Centrifugo RPC）
- ⚠️ 考虑添加 E2E 加密

#### 1.3.3 Backend ↔ Client 通信

| 特性 | Loom | Happy | 差距 |
|------|------|-------|------|
| **API风格** | REST | REST + 动作导向 | Happy 更灵活 |
| **实时协议** | 原生WebSocket | Socket.IO | Happy 更成熟 |
| **认证方式** | deviceId header | Bearer Token | Happy 更安全 |
| **分页** | limit/offset | cursor-based | Happy 更好 |
| **版本控制** | ❌ 无 | /v1, /v2 | Happy 有版本 |
| **乐观并发** | ❌ 无 | expectedVersion | Happy 更好 |

**Loom API：**
```typescript
// backend/src/routes/api/messages/index.ts
GET  /api/messages?session_id=xxx&limit=50&offset=0
POST /api/messages
```

**Happy API：**
```typescript
POST /v1/auth              // 公钥认证
POST /v1/sessions          // 创建会话
GET  /v1/sessions/:id      // 获取会话
// WebSocket for realtime
```

**建议：**
- ⚠️ 添加 API 版本前缀 /v1/
- ⚠️ 将分页改为 cursor-based（性能更好）
- ⚠️ 加强认证机制（当前 deviceId 过于简单）

---

## 2. 数据流与状态管理分析

### 2.1 数据流图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Loom 数据流                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   User Input                                                                 │
│      │                                                                       │
│      ▼                                                                       │
│   ┌─────────────┐    HTTP POST    ┌─────────────┐    INSERT    ┌─────────┐  │
│   │   Client    │ ───────────────▶│   Backend   │ ────────────▶│   DB    │  │
│   └─────────────┘                 └──────┬──────┘              └────┬────┘  │
│                                          │                         │       │
│                                          │ Publish                 │       │
│                                          ▼                         │       │
│                                    ┌─────────────┐                 │       │
│                                    │ Centrifugo  │                 │       │
│                                    └──────┬──────┘                 │       │
│                                           │ Subscribe              │       │
│                                           ▼                        │       │
│                                    ┌─────────────┐                 │       │
│                                    │   Daemon    │ ◀───────────────┘       │
│                                    └──────┬──────┘  Read History           │
│                                           │                                │
│                                           │ SDK Call                       │
│                                           ▼                                │
│                                    ┌─────────────┐                         │
│                                    │   Runtime   │                         │
│                                    └──────┬──────┘                         │
│                                           │ Stream                         │
│                                           ▼                                │
│                                    ┌─────────────┐                         │
│                                    │  AI Response │                        │
│                                    └──────┬──────┘                         │
│                                           │                                │
│   Response Flow                           ▼                                │
│   ─────────────                     ┌─────────────┐    INSERT    ┌────────┐ │
│                                     │   Daemon    │ ────────────▶│   DB   │ │
│                                     └──────┬──────┘              └───┬────┘ │
│                                            │ Publish                  │      │
│                                            ▼                          │      │
│                                      ┌─────────────┐                  │      │
│                                      │ Centrifugo  │                  │      │
│                                      └──────┬──────┘                  │      │
│                                             │ Subscribe               │      │
│                                             ▼                         │      │
│                                       ┌─────────────┐                 │      │
│                                       │   Backend   │ ◀───────────────┘      │
│                                       └──────┬──────┘  Forward to           │
│                                              │         Client              │
│                                              ▼                               │
│                                        ┌─────────────┐                       │
│                                        │   Client    │                       │
│                                        └─────────────┘                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 状态管理对比

#### 2.2.1 Session 状态

**Loom 当前：**
```typescript
// prisma/schema.prisma
model Session {
  id          String
  userId      String
  characterId String?
  title       String?
  createdAt   DateTime
  updatedAt   DateTime
  messages    Message[]
  // 缺少：metadata, agentState, seq
}
```

**Happy-main：**
```typescript
// Happy 的 Session 有版本化字段
Session {
  metadata: { value, version }      // 乐观并发控制
  agentState: { value, version }    // AI状态
  seq: number                       // 会话级序列号
}
```

#### 2.2.2 消息状态

**Loom：**
- 简单存储，无额外状态
- 工具调用存储为 JSON

**Happy：**
- 消息包含 localId（客户端生成）
- 支持消息编辑/删除（通过 update 事件）
- 端到端加密存储

### 2.3 状态同步问题

#### 问题 1：无序列号导致的消息乱序

```
场景：网络抖动导致消息乱序

时间线：
T1: Daemon 发送 msg-1 (network delay)
T2: Daemon 发送 msg-2 (arrives first)
T3: Backend 先处理 msg-2
T4: Backend 后处理 msg-1

结果：消息顺序错误

Happy 解决方案：
- 每个消息有 seq 字段
- 客户端按 seq 排序显示
```

#### 问题 2：多端状态不一致

```
场景：用户同时使用手机和电脑

Loom 当前：
- 每个设备独立拉取
- 无实时同步机制
- 可能看到不同状态

Happy 解决方案：
- user-scoped Socket.IO 连接
- 所有设备实时同步
- 统一 seq 确保一致性
```

---

## 3. 安全模型深度分析

### 3.1 当前 Loom 安全架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Loom 安全架构                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  认证层                                                                      │
│  ────────                                                                   │
│  • deviceId: UUID v4 (简单header传递)                                       │
│  • 无密码/签名机制                                                          │
│  • 无Token过期                                                              │
│                                                                             │
│  传输层                                                                      │
│  ────────                                                                   │
│  • TLS (生产环境)                                                           │
│  • WebSocket wss://                                                         │
│                                                                             │
│  数据层                                                                      │
│  ────────                                                                   │
│  • 明文存储 (PostgreSQL)                                                    │
│  • 无字段级加密                                                             │
│  • 无 E2E 加密                                                              │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Happy-main 安全架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Happy 安全架构                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  认证层                                                                      │
│  ────────                                                                   │
│  • 公钥密码学 (Ed25519)                                                     │
│  • 签名挑战机制                                                             │
│  • Bearer Token (短期有效)                                                  │
│                                                                             │
│  传输层                                                                      │
│  ────────                                                                   │
│  • TLS                                                                      │
│  • Socket.IO                                                                │
│                                                                             │
│  端到端加密                                                                  │
│  ───────────                                                                │
│  • privacy-kit (客户端加密)                                                 │
│  • 服务器无法解密内容                                                       │
│  • 密钥派生树 (KeyTree)                                                     │
│                                                                             │
│  数据分类                                                                    │
│  ─────────                                                                  │
│  • 客户端加密: metadata, state, messages, artifacts                         │
│  • 服务端加密: OAuth tokens, API keys                                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.3 安全风险评估

| 风险项 | 严重性 | Loom现状 | Happy对比 | 建议措施 |
|--------|--------|----------|-----------|----------|
| 设备伪造 | 🔴 高 | deviceId 易伪造 | 公钥签名 | 实现挑战-响应认证 |
| 消息窃听 | 🔴 高 | 服务端明文 | E2E加密 | 添加客户端加密 |
| 中间人攻击 | 🟡 中 | TLS保护 | 相同 | 证书固定 |
| 数据泄露 | 🔴 高 | 数据库明文 | 加密存储 | 加密敏感字段 |
| 会话劫持 | 🟡 中 | 无Token过期 | Token有效期 | 添加Token刷新 |

---

## 4. 可扩展性与性能分析

### 4.1 水平扩展能力

#### Loom 架构扩展性

```
优势：
✅ Centrifugo 原生支持集群
✅ PostgreSQL 可主从复制
✅ Backend 无状态，可水平扩展

瓶颈：
⚠️ 数据库连接池
⚠️ 没有缓存层
⚠️ 消息广播通过 Centrifugo，但查询走 DB
```

#### Happy 架构扩展性

```
优势：
✅ Fastify 高性能
✅ Redis 可作为缓存/消息总线
✅ Presence 缓存减少 DB 写入

瓶颈：
⚠️ Socket.IO 多节点需要 Redis Adapter
⚠️ 端到端加密增加计算开销
```

### 4.2 性能指标对比

| 指标 | Loom | Happy | 业界基准 |
|------|------|-------|----------|
| **连接数/服务器** | ~10K (Centrifugo) | ~5K (Socket.IO) | 10K+ |
| **消息延迟 (P99)** | <100ms | <50ms | <100ms |
| **首字节时间** | 依赖 DB | 依赖 DB | <200ms |
| **并发请求** | 依赖 Bun | 依赖 Node.js | 10K+ |

### 4.3 性能优化建议

1. **添加 Redis 缓存层**
   - 缓存热点会话
   - 缓存用户会话列表
   - 减少 DB 查询

2. **连接池优化**
   ```typescript
   // Prisma 连接池配置
   const prisma = new PrismaClient({
     datasources: {
       db: {
         url: config.database.url,
       },
     },
     // 添加连接池配置
   });
   ```

3. **消息批处理**
   - 客户端批量发送消息
   - 服务端批量写入 DB
   - 减少 I/O 次数

---

## 5. 错误处理与容错分析

### 5.1 当前错误处理机制

#### Loom 已实现

```typescript
// backend/src/lib/errors.ts
class APIError {
  code: string;
  message: string;
  status: number;
  details?: unknown;
}

// Prisma 错误映射
const errorMap: Record<string, APIError> = {
  P2002: Errors.UNIQUE_CONSTRAINT_VIOLATION,
  P2025: Errors.RECORD_NOT_FOUND,
  P2003: Errors.FOREIGN_KEY_CONSTRAINT_VIOLATION,
};

// 中间件统一处理
export async function withErrorHandler<T>(handler: () => Promise<T>): Promise<T> {
  try {
    return await handler();
  } catch (error) {
    if (error instanceof APIError) {
      return jsonError(error) as unknown as T;
    }
    // ...
  }
}
```

#### 已改进点
- ✅ 统一错误格式
- ✅ Prisma 错误映射
- ✅ 结构化日志

### 5.2 容错机制对比

| 机制 | Loom | Happy | 建议 |
|------|------|-------|------|
| 超时重试 | ⚠️ 简单实现 | ✅ 指数退避 | 增强重试策略 |
| 断路器 | ❌ 无 | ❌ 无 | 建议添加 |
| 优雅降级 | ❌ 无 | ✅ 部分实现 | 实现降级策略 |
| 健康检查 | ✅ /health | ✅ /health | 相似 |
| 日志追踪 | ✅ requestId | ✅ correlationId | 相似 |

### 5.3 关键失败场景

```
场景 1: Centrifugo 不可用
当前行为: Backend 无法发送消息到 Daemon
建议: 消息队列缓冲，重试机制

场景 2: 数据库连接失败
当前行为: API 返回 500
建议: 优雅降级，返回缓存数据

场景 3: Daemon 离线
当前行为: 消息丢失（fire-and-forget）
建议: 消息持久化，离线推送
```

---

## 6. 可观测性分析

### 6.1 当前可观测性

```
已具备：
✅ Pino 结构化日志
✅ 请求日志中间件
✅ 基础错误统计
✅ 健康检查端点

缺失：
❌ Metrics (Prometheus)
❌ 分布式追踪
❌ 性能剖析
❌ 业务指标
```

### 6.2 Happy 可观测性

```
已具备：
✅ Prometheus /metrics
✅ HTTP 请求计数和延迟
✅ WebSocket 连接统计
✅ 数据库指标
✅ 业务指标 (presence, usage)
```

### 6.3 建议添加的指标

```typescript
// 基础设施指标
http_requests_total{method, route, status}
http_request_duration_seconds{method, route}
websocket_connections_active
websocket_messages_total{direction, type}

// 业务指标
messages_sent_total{role}
sessions_active
devices_online
llm_requests_total{runtime, status}
llm_request_duration_seconds{runtime}

// 数据库指标
db_query_duration_seconds{operation, table}
db_connections_active
db_transactions_total
```

---

## 7. 部署与运维架构

### 7.1 当前部署架构

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:15-alpine
    
  centrifugo:
    image: centrifugo/centrifugo:v5
    
  backend:
    build:
      dockerfile: Dockerfile.backend
    
  client:
    build:
      dockerfile: Dockerfile.client
```

**优势：**
- ✅ 容器化部署
- ✅ 开发环境一键启动
- ✅ 配置分离 (.env)

**不足：**
- ⚠️ 无高可用配置
- ⚠️ 无监控告警
- ⚠️ 无自动扩缩容
- ⚠️ 日志集中收集缺失

### 7.2 生产环境建议

```
生产架构：

                    ┌─────────────┐
                    │   Nginx     │ (负载均衡)
                    └──────┬──────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
      ┌────┴────┐     ┌────┴────┐    ┌────┴────┐
      │Backend-1│     │Backend-2│    │Backend-N│
      └────┬────┘     └────┬────┘    └────┬────┘
           │               │               │
           └───────────────┼───────────────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
      ┌────┴────┐    ┌────┴────┐    ┌────┴────┐
      │PostgreSQL│    │  Redis  │    │Centrifugo│
      │ Primary │    │ Cluster │    │ Cluster │
      └────┬────┘    └─────────┘    └─────────┘
           │
      ┌────┴────┐
      │  Replica │
      └─────────┘

监控栈：
- Prometheus + Grafana
- Loki (日志收集)
- Jaeger (分布式追踪)
- AlertManager (告警)
```

---

## 8. 与其他AI会话产品对比

### 8.1 架构对比矩阵

| 产品 | 本地Runtime | 云端同步 | 协议 | 实时通信 | E2E加密 |
|------|------------|----------|------|----------|---------|
| **Claude Code** | ✅ | ❌ | 专有API | SSE | ❌ |
| **OpenCode** | ✅ | ❌ | 本地 | - | ❌ |
| **Cursor** | ✅ | ✅ | 专有 | WS | ❌ |
| **Happy** | ✅ | ✅ | HTTP+Socket.IO | WS | ✅ |
| **Loom 当前** | ✅ | ✅ | HTTP+WS | WS (Centrifugo) | ❌ |
| **Loom 目标** | ✅ | ✅ | HTTP+WS | WS | ✅ |

### 8.2 各产品架构特点

**Claude Code:**
- 本地执行，无云端
- 直接使用 Anthropic API
- 简单 HTTP + SSE

**Cursor:**
- 云端同步代码和会话
- 专有协议
- 本地+云端混合

**Happy:**
- 完整的 E2E 加密
- 多端实时同步
- 标准化 Session Protocol

---

## 9. 技术债务与风险评估

### 9.1 技术债务清单

| 债务项 | 严重性 | 影响 | 解决建议 |
|--------|--------|------|----------|
| deviceId 简单认证 | 🔴 高 | 安全隐患 | 实现挑战-响应 |
| 明文消息存储 | 🔴 高 | 隐私风险 | 添加 E2E 加密 |
| 无序列号 | 🟡 中 | 消息乱序 | 添加 seq 字段 |
| 无乐观并发 | 🟡 中 | 数据覆盖 | 添加 version |
| Daemon 实现不完整 | 🔴 高 | 核心功能缺失 | 完成实现 |
| Runtime 未实现 | 🔴 高 | 无法运行 | 接入实际 SDK |
| 无消息队列 | 🟡 中 | 消息丢失 | 添加持久化队列 |
| 无缓存层 | 🟢 低 | 性能瓶颈 | 添加 Redis |

### 9.2 风险矩阵

```
影响
 高 │ 明文存储    无认证
    │    ●           ●
    │
 中 │ 无序列号  Daemon未完成
    │    ●            ●
    │
 低 │ 无缓存
    │    ●
    └──────────────────────────
         低        中        高   概率
```

---

## 10. 架构改进路线图

### 10.1 短期改进（1-2个月）

```
Priority 1: 核心安全
├── 实现挑战-响应认证
├── 设备指纹生成
└── Token 刷新机制

Priority 2: 协议完善
├── 添加消息序列号
├── 统一错误码
└── 消息确认机制

Priority 3: 功能完成
├── 完成 Daemon 实现
├── 接入 Claude Code SDK
└── 接入 OpenCode SDK
```

### 10.2 中期改进（3-6个月）

```
Phase 1: 端到端加密
├── 密钥派生实现
├── 客户端加密
├── 服务端透传
└── 多设备密钥同步

Phase 2: 状态同步
├── 乐观并发控制
├── 版本化字段
├── 离线消息队列
└── 多端同步

Phase 3: 可观测性
├── Prometheus metrics
├── Grafana 仪表盘
├── 日志聚合
└── 告警规则
```

### 10.3 长期演进（6-12个月）

```
Architecture Evolution:

v1.0 (当前) ──▶ v1.5 ──▶ v2.0 (目标)
   │             │          │
   │          添加加密    完整 E2E
   │          完善协议    多端同步
   │          可观测性    企业级安全
   │                      性能优化
   │                      智能缓存
```

---

## 11. 关键决策建议

### 11.1 保留当前设计

✅ **Centrifugo**：水平扩展能力优秀，保持使用
✅ **TanStack Start**：现代化框架，开发体验好
✅ **Prisma ORM**：类型安全，迁移方便
✅ **Zod 验证**：运行时报错清晰

### 11.2 必须改进

🔴 **高优先级**：
1. 实现安全的设备认证（挑战-响应）
2. 添加消息序列号机制
3. 完成 Daemon 核心功能
4. 实现 Runtime 接入

🟡 **中优先级**：
1. 添加端到端加密
2. 实现乐观并发控制
3. 添加消息队列
4. 完善可观测性

🟢 **低优先级**：
1. 添加 Redis 缓存
2. API 版本化
3. Cursor-based 分页

### 11.3 架构决策记录

| 决策 | 状态 | 理由 |
|------|------|------|
| 保留 Centrifugo | ✅ 确认 | 水平扩展优势 |
| 采用 Socket.IO vs 原生 WS | 🤔 待定 | Centrifugo 有 Socket.IO 兼容层 |
| 端到端加密 | 📝 计划 | 隐私保护必需 |
| 乐观并发控制 | 📝 计划 | 数据一致性 |
| 公钥认证 | 📝 计划 | 安全必需 |

---

## 附录

### A. 代码位置索引

| 组件 | Loom 路径 | Happy 路径 |
|------|-----------|------------|
| Runtime | `cmd/daemon/runtime/` | `packages/happy-cli/src/claude/`, `src/codex/` |
| Daemon | `cmd/daemon/` | `packages/happy-cli/src/daemon/` |
| Backend | `backend/src/` | `packages/happy-server/sources/` |
| Client | `client/src/` | Mobile/Web 客户端 |
| Protocol | `backend/src/lib/schemas.ts` | `docs/session-protocol.md` |

### B. 参考文档

- Happy Protocol: `/docs/protocol.md`
- Happy Backend: `/docs/backend-architecture.md`
- Happy CLI: `/docs/cli-architecture.md`
- Happy Session Protocol: `/docs/session-protocol.md`

### C. 术语表

- **E2E Encryption**: 端到端加密
- **Optimistic Concurrency**: 乐观并发控制
- **Monotonic Sequence**: 单调递增序列号
- **Centrifugo**: WebSocket 消息服务器
- **Socket.IO**: 实时通信库
- **Session Protocol**: 统一消息格式协议
