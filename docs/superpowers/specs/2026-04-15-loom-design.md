# Loom 项目技术设计文档

**日期**：2026-04-15
**状态**：草稿

---

## 一、项目概述

Loom 是一个具有"氛围感"的虚拟陪伴产品，通过本地 AI Runtime（Claude Code、OpenCode）提供 AI 对话能力，结合 lofi 音乐、角色形象等元素，打造二次元体验。

**核心定位**：解决现有 AI Runtime"工作属性太重"的问题，让 AI 交互更具娱乐化和二次元体验。

**目标用户**：具备代码能力且喜欢亚文化的重度用户

---

## 二、技术架构

### 2.1 整体架构

```
┌────────────────────────────────────────────────────────────────────┐
│                         用户机器 (localhost)                        │
│                                                                     │
│  ┌─────────────┐                  ┌──────────────────────────┐    │
│  │   Runtime   │◀── SDK ─────────▶│         Daemon           │    │
│  │ Claude Code │    (驱动层)       │  ┌──────────────────┐   │    │
│  │  OpenCode   │                  │  │  Runtime Adapter │   │    │
│  └─────────────┘                  │  │  (Claude/OpenCode)│   │    │
│                                    │  │  - Chat          │   │    │
│                                    │  │  - ExecuteTool  │   │    │
│                                    │  │  - ListCaps     │   │    │
│                                    │  └──────────────────┘   │    │
│                                    │  ┌──────────────────┐   │    │
│                                    │  │   WS Client      │   │    │
│                                    │  │  (连 Backend)    │   │    │
│                                    │  └──────────────────┘   │    │
│                                    └────────────┬─────────────┘    │
└─────────────────────────────────────────────────┼──────────────────┘
                                                  │ WebSocket
                                                  ▼
┌──────────────────────────────────────────────────────────────────────┐
│                           云端                                        │
│                                                                       │
│   ┌─────────────┐         ┌─────────────────┐    ┌────────────────┐ │
│   │   Client    │◀───────▶│    Backend     │◀───▶│  Centrifugo    │ │
│   │  (Web UI)   │   HTTP  │    (Bun)       │ WS  │  (WS Server)   │ │
│   │             │         │  - API         │    │                │ │
│   │ TanStack    │         │  - 业务逻辑    │    │  - 用户频道    │ │
│   │  Start      │         │  - 消息转发    │    │  - Daemon 频道 │ │
│   └─────────────┘         └─────────────────┘    └────────────────┘ │
└──────────────────────────────────────────────────────────────────────┘
```

### 2.2 技术栈

| 层级 | 技术选择 | 说明 |
|------|----------|------|
| 前端 | TanStack Start | 全栈框架 |
| 后端 API | Bun | 高性能 JavaScript 运行时 |
| Daemon | Go | 本地 CLI，编译成单文件二进制 |
| WebSocket | Centrifugo | 生产级 WS 服务器 |
| 数据库 | PostgreSQL | 持久化存储 |

### 2.3 通信协议

| 组件对 | 协议 | 说明 |
|--------|------|------|
| Client ↔ Backend | HTTP + WebSocket | Web UI 访问云端 API 和 WS |
| Daemon ↔ Backend | WebSocket | 持久连接，双向通信，心跳保活 |
| Runtime ↔ Daemon | 子进程/SDK | 调用本地 Claude Code/OpenCode |
| Client ↔ Daemon | HTTP | 开发调试用（可选） |

---

## 三、组件设计

### 3.1 Daemon（Go）

**职责**：
- 统一 Runtime 接口（适配 Claude Code、OpenCode）
- 管理本地 AI 能力（Tools、Skills）
- 与 Backend 建立 WebSocket 持久连接

**命令行参数**：
```bash
loomd [command]

Commands:
  start        启动 Daemon 并连接 Backend
  version      显示版本号
  config       查看/修改配置

Flags:
  --config string   配置文件路径 (默认 "~/loom/config.yaml")
  --verbose         详细输出
```

**运行时支持**：
- 通过配置切换：`runtime: "claude-code"` 或 `"opencode"`
- 内部统一抽象层，底层调用不同的 Runtime
- 后续加新 Runtime 只需加适配器

**内部 API（gRPC 风格设计，仅内部使用）**：
```go
// chat.proto (内部实现)
service LoomDaemon {
  rpc Chat(stream ChatRequest) returns (stream ChatResponse);
  rpc ExecuteTool(ToolRequest) returns (ToolResponse);
  rpc ListCapabilities(CapabilityRequest) returns (CapabilityResponse);
}
```

### 3.2 Backend（TanStack Start + Bun）

**职责**：
- 用户认证（设备指纹 + 邮箱注册）
- 人设管理
- 会话管理
- 消息路由（Client ↔ Daemon）
- WebSocket 消息转发

**API 设计**：

```
Authentication:
  POST   /api/auth/register        注册
  POST   /api/auth/login           登录
  GET    /api/auth/me              当前用户

Characters:
  GET    /api/characters           角色列表
  POST   /api/characters           创建角色
  DELETE /api/characters/:id       删除角色
  POST   /api/characters/import    导入 Tavern AI 人设

Sessions:
  GET    /api/sessions             会话列表
  POST   /api/sessions             创建会话
  DELETE /api/sessions/:id         删除会话

Chat (WebSocket):
  WS     /ws/chat                  实时对话
```

### 3.3 Centrifugo（WebSocket 服务器）

**职责**：
- 处理大量 WebSocket 连接
- 消息路由
- 连接鉴权

**频道设计**：
- `user:{user_id}` - 用户自己的频道
- `daemon:{device_id}` - Daemon 频道

### 3.4 Client（TanStack Start）

**lofi 音乐来源**：
- **免费电台**：接入公共电台 API（如 Radio Browser、Internet Radio）
- **自定义**：用户可以自己添加音乐 URL
- **后续扩展**：高级版提供精选音乐池

**UI 分层设计**：

```
┌─────────────────────────────────┐
│  Layer 4: 控制层 (最小化/设置)   │  ← 始终可触达
├─────────────────────────────────┤
│  Layer 3: 对话层 (可收起)        │  ← 对话时展开
├─────────────────────────────────┤
│  Layer 2: 角色层 (占位符)        │  ← 始终显示
├─────────────────────────────────┤
│  Layer 1: 背景层 (GIF/视频)      │  ← 始终播放
└─────────────────────────────────┘
```

**MVP 界面布局**：

```
┌─────────────────────────────────────────┐
│  🌙 [lofi 播放条]              [设置] ⚙️ │  ← 顶部常驻
├─────────────────────────────────────────┤
│                                         │
│           [GIF/视频背景]                 │  ← 主体
│                                         │
│   ┌─────────────┐                       │
│   │  角色占位符  │                       │
│   └─────────────┘                       │
│                                         │
│  ┌────────────────────────────────────┐ │
│  │ 💬 对话框 (输入框)                  │ │  ← 底部，可收起
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

---

## 四、数据模型

### 4.1 用户表（users）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| device_id | UUID | 设备指纹（匿名用） |
| email | VARCHAR(255) | 邮箱（可选） |
| password_hash | VARCHAR(255) | 密码 hash |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |
| last_login | TIMESTAMP | 最后登录 |

### 4.2 角色表（characters）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| user_id | UUID | 所属用户 |
| name | VARCHAR(100) | 角色名 |
| description | TEXT | 简介 |
| prompt | TEXT | 角色设定 prompt |
| avatar_url | VARCHAR(500) | 头像 URL |
| created_at | TIMESTAMP | 创建时间 |

### 4.3 会话表（sessions）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| user_id | UUID | 所属用户 |
| character_id | UUID | 关联角色 |
| title | VARCHAR(200) | 会话标题 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

### 4.4 消息表（messages）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | 主键 |
| session_id | UUID | 所属会话 |
| role | VARCHAR(20) | user/assistant |
| content | TEXT | 消息内容 |
| tools | JSONB | 使用的 tools |
| created_at | TIMESTAMP | 创建时间 |

---

## 五、MVP 功能范围

| 功能 | 状态 | 说明 |
|------|------|------|
| 对话能力 | MVP | AI 对话 + Tools 执行 |
| lofi 音乐 | MVP | 背景音乐播放 |
| 角色占位 | MVP | 简单图片占位 |
| 设备指纹 | MVP | 匿名使用 |
| 用户认证 | v1.0 | 邮箱注册登录 |
| 人设系统 | v1.1 | Tavern AI 导入 |
| TTS | v1.2 | 语音输出 |
| ASR | v1.3 | 语音输入 |
| 记忆系统 | v1.4 | 记住用户偏好 |
| Live2D | v2.0 | 动态立绘 |

---

## 六、商业模式

| 层级 | 价格 | 功能 |
|------|------|------|
| **免费版** | ¥0 | 基础对话 + 基础 lofi 背景 + 设备指纹 |
| **托管版** | ¥9.9/月 | 云端服务，免部署，多设备同步 |
| **高级版** | ¥19.9/月 | Live2D 角色、高级音乐池、自定义背景 |

---

## 七、部署方式

### 7.1 Daemon 分发

| 方式 | 说明 |
|------|------|
| **npm（推荐）** | `npx loom-daemon` 或 `npm i -g loom-daemon` |
| **二进制** | GitHub Releases 下载 |

### 7.2 Backend 部署

- Docker Compose 一键部署
- 包含：Backend + Centrifugo + PostgreSQL

---

## 八、待确定事项

- [ ] TTS/ASR 技术选型（Azure/Coqui/其他）
- [ ] Live2D 模型来源（社区/自制）
- [ ] lofi 音乐版权问题
- [ ] 支付接入方案

---

## 九、参考项目

- **TanStack Start**：全栈框架
- **Centrifugo**：生产级 WebSocket 服务器
- **lofi.cafe**：UI 风格参考
- **Tavern AI**：人设导入参考
- **Claude Code**：Runtime 参考
