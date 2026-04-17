# Loom Backend

Loom Backend 是基于 TanStack Start + Bun 的 API 服务器，负责用户认证、会话管理、消息路由和 WebSocket 通信。

## 功能特性

- **用户认证**: 设备指纹 + 邮箱注册登录
- **会话管理**: 创建、列表、获取、删除会话
- **消息管理**: 消息存储、分页查询
- **WebSocket 通信**: 通过 Centrifugo 与 Daemon 和 Client 实时通信
- **消息验证**: 使用 Zod 进行严格的输入验证
- **结构化日志**: 使用 Pino 记录日志

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Loom Backend                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   HTTP API                    WebSocket (Centrifugo)        │
│   ─────────                   ─────────────────────         │
│                                                              │
│   POST /api/auth/device       Subscribe: user:${deviceId}   │
│   POST /api/sessions          Publish: daemon:${deviceId}   │
│   GET  /api/sessions                                        │
│   POST /api/messages                                        │
│                                                              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
                ┌─────────────────┐
                │   PostgreSQL    │
                └─────────────────┘
```

## 快速开始

### 安装依赖

```bash
cd backend
bun install
```

### 环境变量

```bash
# Database
DATABASE_URL=postgresql://loom:loom_dev@localhost:5432/loom

# Centrifugo
CENTRIFUGO_URL=ws://localhost:8000
CENTRIFUGO_API_KEY=your-api-key
CENTRIFUGO_TOKEN_SECRET=your-token-secret

# App
APP_PORT=3000
LOG_LEVEL=info
```

### 运行开发服务器

```bash
bun run dev
```

### 构建生产版本

```bash
bun run build
```

## API 文档

### 认证

#### 设备认证

```bash
POST /api/auth/device
Content-Type: application/json

{
  "device_id": "optional-existing-device-id"
}
```

响应：
```json
{
  "data": {
    "user": {
      "id": "uuid",
      "device_id": "device-uuid"
    },
    "is_new": true
  }
}
```

### 会话

#### 创建会话

```bash
POST /api/sessions
Content-Type: application/json
x-device-id: your-device-id

{
  "title": "Optional session title",
  "character_id": "optional-character-uuid"
}
```

#### 列列会话

```bash
GET /api/sessions
x-device-id: your-device-id
```

#### 获取会话

```bash
GET /api/sessions/:id
x-device-id: your-device-id
```

#### 删除会话

```bash
DELETE /api/sessions/:id
x-device-id: your-device-id
```

### 消息

#### 发送消息

```bash
POST /api/messages
Content-Type: application/json
x-device-id: your-device-id

{
  "session_id": "session-uuid",
  "content": "Hello!"
}
```

消息会通过 WebSocket 转发到 Daemon 处理。

#### 获取消息列表

```bash
GET /api/messages?session_id=xxx&limit=50&offset=0
x-device-id: your-device-id
```

### 健康检查

```bash
GET /api/health
```

## WebSocket 消息协议

### 频道

- `user:${deviceId}` - Backend 订阅接收 Daemon 消息
- `daemon:${deviceId}` - Backend 发布消息到 Daemon

### 消息格式

#### Daemon → Backend

```typescript
// chat:response
{
  type: "chat:response",
  data: {
    session_id: "uuid",
    content: "Response text",
    message_id?: "uuid"
  }
}

// chat:thinking
{
  type: "chat:thinking",
  data: {
    session_id: "uuid",
    thinking: true
  }
}

// chat:tool_call
{
  type: "chat:tool_call",
  data: {
    session_id: "uuid",
    tool_name: "Bash",
    tool_input: { command: "ls" },
    tool_call_id: "uuid"
  }
}

// chat:error
{
  type: "chat:error",
  data: {
    session_id: "uuid",
    error: "Error message"
  }
}
```

#### Backend → Client

与 Daemon → Backend 格式相同，通过 Client 的 WebSocket 连接发送。

## 项目结构

```
backend/
├── src/
│   ├── index.ts              # 入口文件
│   ├── router.ts             # 路由配置
│   ├── lib/                  # 工具库
│   │   ├── auth.ts           # 认证
│   │   ├── config.ts         # 配置
│   │   ├── db.ts             # 数据库连接
│   │   ├── errors.ts         # 错误定义
│   │   ├── logger.ts         # 日志
│   │   ├── messageHandler.ts # 消息处理
│   │   ├── response.ts       # 响应工具
│   │   ├── schemas.ts        # Zod 验证
│   │   └── ws.ts             # WebSocket 服务器
│   ├── middleware/           # 中间件
│   │   ├── errorHandler.ts
│   │   └── requestLogger.ts
│   └── routes/               # API 路由
│       ├── api/
│       │   ├── auth/
│       │   │   └── device.ts
│       │   ├── sessions/
│       │   │   ├── index.ts
│       │   │   └── [id].ts
│       │   ├── messages/
│       │   │   └── index.ts
│       │   └── health.ts
│       ├── __root.tsx
│       └── index.tsx
├── package.json
├── tsconfig.json
└── vite.config.ts
```

## 开发

### 添加新路由

使用 TanStack Start 的文件路由约定：

```typescript
// src/routes/api/my-feature/index.ts
import { Route } from "@tanstack/start";

export const myFeatureRoute = new Route({
  path: "/api/my-feature",
  method: "GET",
  handler: async (req: Request) => {
    return jsonSuccess({ message: "Hello" });
  },
});
```

### 错误处理

使用统一的错误类型：

```typescript
import { Errors, APIError } from "~/lib/errors";
import { jsonError } from "~/lib/response";

// 使用预定义错误
return jsonError(Errors.UNAUTHORIZED);

// 自定义错误
return jsonError(
  new APIError("CUSTOM_ERROR", "Custom message", 400)
);
```

## 许可证

MIT
