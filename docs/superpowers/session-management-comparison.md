# Session 管理方案比较与分析

## 1. 流行方案概览

### 方案 A: 纯内存管理 (Simple)
```
┌─────────────────┐
│   Daemon        │
│ ┌─────────────┐ │
│ │ Sessions    │ │  (map[string]*Session)
│ │  (memory)   │ │
│ └─────────────┘ │
└─────────────────┘
```
**代表**: 简单 CLI 工具、开发原型

**优点**:
- 实现简单，无外部依赖
- 访问速度快
- 无持久化开销

**缺点**:
- Daemon 重启后数据丢失
- 无法跨进程恢复
- 系统崩溃后无恢复能力

**生产安全**: ❌ 不推荐

---

### 方案 B: 文件系统存储 (File-Based)
```
┌─────────────────┐        ┌─────────────────┐
│   Daemon        │───────▶│ ~/.loom/sessions│
│ ┌─────────────┐ │        │ ├─ session-1.json
│ │ Sessions    │ │        │ ├─ session-2.json
│ │  (memory)   │ │        │ └─ session-3.json
│ └─────────────┘ │        └─────────────────┘
└─────────────────┘
```
**代表**: Happy CLI, Claude Code

**优点**:
- 持久化简单可靠
- 易于调试（可读文件）
- 无额外依赖
- 跨进程可恢复

**缺点**:
- 文件 I/O 性能开销
- 并发写入需要锁
- 查询能力有限
- 归档/清理需要额外实现

**生产安全**: ✅ 推荐

---

### 方案 C: SQLite/嵌入式数据库
```
┌─────────────────┐        ┌─────────────────┐
│   Daemon        │───────▶│ sessions.db     │
│ ┌─────────────┐ │        │ (SQLite)        │
│ │ Sessions    │ │        │ - ACID          │
│ │  (memory)   │ │        │ - Indexed       │
│ └─────────────┘ │        │ - Queryable     │
└─────────────────┘        └─────────────────┘
```
**代表**: 桌面应用、本地优先软件

**优点**:
- ACID 事务保证
- 强大的查询能力
- 索引支持
- 单文件便携

**缺点**:
- 增加依赖 (SQLite driver)
- 二进制文件不易调试
- 性能对小项目过度

**生产安全**: ✅ 推荐

---

### 方案 D: Redis/外部数据库
```
┌─────────────────┐        ┌─────────────────┐
│   Daemon        │───────▶│ Redis           │
│ ┌─────────────┐ │        │ - Fast KV       │
│ │ Sessions    │ │        │ - TTL support   │
│ │  (memory)   │ │        │ - Pub/Sub       │
│ └─────────────┘ │        └─────────────────┘
└─────────────────┘
```
**代表**: 分布式系统、多实例部署

**优点**:
- 极高性能
- 自动过期 (TTL)
- 支持分布式
- 发布/订阅能力

**缺点**:
- 需要运维 Redis
- 网络依赖
- 单机场景过度设计

**生产安全**: ⚠️ 需运维保障

---

### 方案 E: 混合方案 (本地 + 云端)
```
┌─────────────────┐        ┌─────────────────┐
│   Daemon        │───────▶│ Local Store     │
│ ┌─────────────┐ │        │ (Primary)       │
│ │ Sessions    │ │        └─────────────────┘
│ │  (memory)   │ │               │
│ └─────────────┘ │               ▼
└─────────────────┘        ┌─────────────────┐
         │                 │ Cloud Backup    │
         └───────────────▶│ (Sync)          │
                           └─────────────────┘
```
**代表**: Happy CLI, VS Code Settings Sync

**优点**:
- 本地优先（快速响应）
- 云端备份（跨设备）
- 离线可用
- 数据安全

**缺点**:
- 同步复杂性
- 冲突解决逻辑
- 端到端加密需求

**生产安全**: ✅ 推荐（实现复杂）

---

## 2. Happy CLI 方案深度分析

### Happy 的架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Happy Session Architecture               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │   Session    │    │   Claude     │    │   Happy      │  │
│  │   Scanner    │◀──▶│   Session    │    │   Server     │  │
│  │              │    │   Files      │    │   (Cloud)    │  │
│  │ - fsnotify   │    │   (.jsonl)   │    │              │  │
│  │ - Poll       │    │              │    │ - Encrypted  │  │
│  │ - Key track  │    │ - Raw JSONL  │    │ - Sync       │  │
│  └──────────────┘    └──────────────┘    └──────────────┘  │
│                                                              │
│  ┌──────────────┐    ┌──────────────┐                       │
│  │   Session    │    │   Daemon     │                       │
│  │   Manager    │    │   State      │                       │
│  │   (Memory)   │    │   (JSON)     │                       │
│  │              │    │              │                       │
│  │ - Active     │    │ - PID map    │                       │
│  │ - Pending    │    │ - Metadata   │                       │
│  │ - Finished   │    │ - Heartbeat  │                       │
│  └──────────────┘    └──────────────┘                       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Happy 的核心设计

**1. Claude Session 文件为主**
- Claude Code 自动写入 `.jsonl` 文件
- Happy 只读取，不干预 Claude 存储
- 天然持久化，无需额外存储

**2. Session Scanner 模式**
```typescript
// 关键设计：去重 + 增量读取
const processedMessageKeys = new Set<string>();

// 1. 启动时读取所有历史消息（标记为已处理）
// 2. fsnotify 监听文件变化
// 3. 只发送新增消息（通过 key 去重）
```

**3. 三层状态管理**
```
Layer 1: Claude Session Files (磁盘) - 真实数据源
Layer 2: Daemon PID Map (内存) - 运行时跟踪
Layer 3: Happy Server (云端) - 跨设备同步
```

**4. 恢复机制**
```typescript
// Find last session by mtime
const lastSession = claudeFindLastSession(workingDirectory);

// Resume with --resume flag
claude --resume <session-id>
```

### Happy 方案的优点

1. **零存储开销** - 复用 Claude 的存储
2. **强一致性** - 文件系统保证
3. **可调试** - 直接查看 `.jsonl` 文件
4. **简单可靠** - 无数据库依赖

### Happy 方案的局限

1. **依赖 Claude** - 只能用于 Claude Code
2. **查询能力弱** - 只能按时间/文件名查询
3. **无元数据扩展** - 无法存储额外 session 信息
4. **归档困难** - 需要手动清理旧文件

---

## 3. Loom 需求分析

### Loom 的产品定位
- **本地 AI 陪伴** - 情感化聊天
- **多 Runtime 支持** - Claude Code + OpenCode
- **Web UI 为主** - 非命令行优先
- **无需手机 App** - 不需要跨设备同步

### Loom 的 Session 需求

| 需求 | 优先级 | 说明 |
|------|--------|------|
| 会话持久化 | 高 | 刷新页面后恢复对话 |
| 会话恢复 | 高 | 支持 --resume |
| 思考状态 | 高 | 实时显示 AI 思考中 |
| 历史查询 | 中 | 查看历史会话列表 |
| 离线可用 | 中 | Daemon 重启后恢复 |
| 云端同步 | 低 | 无需跨设备 |
| 复杂查询 | 低 | 无需全文检索 |

---

## 4. 方案对比矩阵

| 维度 | 内存 | 文件系统 | SQLite | Redis | 混合 |
|------|------|----------|--------|-------|------|
| 实现复杂度 | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ | ⭐ |
| 可靠性 | ⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| 性能 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |
| 查询能力 | ⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| 运维成本 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |
| 生产安全 | ❌ | ✅ | ✅ | ⚠️ | ✅ |
| Loom 匹配度 | ⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |

---

## 5. 推荐方案: 增强文件系统方案

### 为什么选文件系统？

1. **与 Happy 一致** - 参考成熟方案
2. **符合 Loom 需求** - 无需复杂查询
3. **简单可靠** - 易于维护和调试
4. **Runtime 无关** - Claude 和 OpenCode 都支持文件存储

### 增强设计

```
┌─────────────────────────────────────────────────────────────┐
│                  Loom Session Architecture                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ~/.loom/                                                    │
│  ├── daemon.json              # Daemon 状态                 │
│  ├── daemon.lock              # 进程锁                      │
│  ├── daemon.sock              # 控制接口                    │
│  └── sessions/                # Session 存储                │
│      ├── index.json           # Session 索引                │
│      ├── session-xxx/         # 单个 session 目录           │
│      │   ├── metadata.json    # 元数据                      │
│      │   ├── messages.jsonl   # 消息记录                    │
│      │   └── state.json       # 运行时状态                  │
│      └── session-yyy/                                       │
│                                                              │
│  ~/.claude/loom/              # Claude 原生 session         │
│  └── <uuid>.jsonl             # Claude 原生格式             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 核心组件

**1. Session Store (接口)**
```go
type SessionStore interface {
    // CRUD
    Create(session *Session) error
    Get(id string) (*Session, error)
    Update(session *Session) error
    Delete(id string) error
    
    // Query
    List(opts ListOptions) ([]*Session, error)
    GetActive() ([]*Session, error)
    
    // Lifecycle
    Archive(id string) error
    Cleanup(before time.Time) error
}
```

**2. FileSessionStore (实现)**
```go
type FileSessionStore struct {
    basePath string
    mu       sync.RWMutex
    index    *SessionIndex
}

// 原子写入：temp file + rename
func (s *FileSessionStore) writeAtomic(path string, data []byte) error
```

**3. Session Index (内存缓存)**
```go
type SessionIndex struct {
    sessions map[string]*SessionMeta
    mu       sync.RWMutex
}

// 启动时从磁盘加载
// 变更时增量更新
```

**4. Session Manager (协调层)**
```go
type SessionManager struct {
    store    SessionStore
    scanner  *SessionScanner
    active   map[string]*ActiveSession
    mu       sync.RWMutex
}
```

### 数据模型

**Session Metadata**
```go
type Session struct {
    ID          string    `json:"id"`
    Path        string    `json:"path"`           // 工作目录
    Runtime     string    `json:"runtime"`        // claude/opencode
    Status      string    `json:"status"`         // active/paused/archived
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    LastMessage time.Time `json:"last_message"`
    MessageCount int      `json:"message_count"`
    
    // Runtime 特定
    RuntimeSessionID string `json:"runtime_session_id"` // Claude's UUID
}
```

**Message Format**
```go
type StoredMessage struct {
    ID        string          `json:"id"`
    Time      int64           `json:"time"`
    Role      string          `json:"role"`  // user/agent
    Content   string          `json:"content"`
    Type      string          `json:"type"`  // text/tool/file
    Metadata  json.RawMessage `json:"metadata,omitempty"`
}
```

### 关键流程

**创建 Session**
```
1. 生成 Session ID (CUID2)
2. 创建目录 ~/.loom/sessions/<id>/
3. 写入 metadata.json
4. 启动 Runtime (claude --session-id <id>)
5. Scanner 开始监听
6. 更新索引
```

**恢复 Session**
```
1. 查询 index.json 获取列表
2. 用户选择 session
3. 读取 metadata.json
4. 检查 runtime session ID
5. 执行 claude --resume <uuid>
6. 重新连接 Scanner
```

**消息同步**
```
1. Scanner 监听文件变化
2. 读取新增行 (JSONL)
3. 转换为 StoredMessage
4. 追加到 messages.jsonl
5. 通过 WebSocket 推送到 UI
6. 更新索引 (last_message, count)
```

### 归档与清理

```go
// 自动归档策略
func (m *SessionManager) AutoArchive() {
    // 归档 7 天无活动的 session
    // 永久删除 30 天前的归档
}

// 手动归档
func (m *SessionManager) ArchiveSession(id string) error
```

---

## 6. 生产安全保证

### 数据安全
- ✅ 原子写入 (temp + rename)
- ✅ 文件权限 0600
- ✅ 定期备份 (可选)

### 并发安全
- ✅ 读写锁保护
- ✅ 单进程 Daemon 访问
- ✅ 文件锁防止并发写

### 可靠性
- ✅ 启动时数据校验
- ✅ 损坏自动恢复
- ✅ 优雅降级 (空数据)

### 可观测性
- ✅ 结构化日志
- ✅ 指标收集
- ✅ 健康检查

---

## 7. 实施优先级

| 优先级 | 组件 | 说明 |
|--------|------|------|
| P0 | SessionStore 接口 | 核心存储抽象 |
| P0 | FileSessionStore | 文件系统实现 |
| P0 | SessionManager | 协调层 |
| P1 | Session Index | 内存索引加速 |
| P1 | Archive/Cleanup | 生命周期管理 |
| P2 | Backup | 自动备份 |
| P2 | Migration | 数据迁移工具 |

---

## 8. 总结

**选择: 增强文件系统方案**

理由:
1. **匹配需求** - 满足 Loom 所有需求，不过度设计
2. **生产安全** - 简单可靠，易于维护
3. **参考成熟** - 基于 Happy CLI 验证的方案
4. **扩展性好** - 未来可切换到 SQLite 或混合方案

**不选择其他方案的理由:**
- SQLite: 小项目过度，增加依赖
- Redis: 需要运维，单机场景过度
- 混合: 实现复杂，Loom 无需跨设备
