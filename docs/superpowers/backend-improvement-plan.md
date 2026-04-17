# Loom 后端改进计划

## 当前后端架构分析

```
┌─────────────────────────────────────────────────────────────────┐
│                     Current Backend Architecture                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Client (React)                                                 │
│       │                                                         │
│       ▼ HTTP                                                    │
│  ┌─────────────────┐                                            │
│  │   Backend       │  TanStack Start + Bun                      │
│  │   (3000)        │                                            │
│  │                 │  ┌─────────┐  ┌─────────┐  ┌─────────┐    │
│  │ - /api/auth/*   │  │  db.ts  │  │  ws.ts  │  │ routes  │    │
│  │ - / (index)     │  │  (Pool) │  │(Centri- │  │(sparse) │    │
│  │                 │  │         │  │ fugo)   │  │         │    │
│  └────────┬────────┘  └────┬────┘  └────┬────┘  └────┬────┘    │
│           │                │            │            │          │
│           ▼ WebSocket      ▼            ▼            ▼          │
│  ┌─────────────────┐  ┌──────────┐  ┌──────────┐                │
│  │   Centrifugo    │  │ Postgres │  │  Daemon  │                │
│  │   (8000)        │  │ (5432)   │  │ (Go CLI) │                │
│  │                 │  │          │  │          │                │
│  └─────────────────┘  └──────────┘  └──────────┘                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## 问题诊断

### 1. 🔴 API 不完整

**现状**: 只有 `/api/auth/device`

**缺失**:
- ❌ 会话管理 API (CRUD)
- ❌ 消息 API (发送/接收/历史)
- ❌ 角色/人设 API
- ❌ 用户设置 API
- ❌ Daemon 控制 API

**代码**:
```typescript
// 现在只有这个
/api/auth/device - POST - 设备认证

// 缺失的 API
/api/sessions/*     - 会话管理
/api/messages/*     - 消息操作  
/api/characters/*   - 角色管理
/api/settings/*     - 用户设置
/api/daemon/*       - Daemon 控制
```

---

### 2. 🔴 数据库层薄弱

**现状**:
```typescript
// db.ts - 太简单
export const db = {
  query: async (text: string, params?: any[]) => {
    const result = await pool.query(text, params);
    return result;
  },
};
```

**问题**:
- 无类型安全
- 无事务支持
- 无连接池管理
- SQL 硬编码在各处
- 无迁移管理

---

### 3. 🟡 与 Daemon 集成不清晰

**现状**:
- Backend → Centrifugo → Daemon (WebSocket)
- 但消息流转逻辑不清晰
- 无统一的 RPC 机制

**问题**:
```
Client → Backend → Centrifugo → Daemon → Claude
   ↑_________________________________________|
              (响应如何回来?)
```

需要明确的请求-响应流程。

---

### 4. 🟡 错误处理缺失

**现状**: 各路由自行处理错误，无统一格式

**问题**:
- 错误响应格式不一致
- 无错误码定义
- 客户端难以处理

---

### 5. 🟡 配置管理简单

**现状**: 全部用 `process.env`

**问题**:
- 无配置验证
- 无环境区分
- 无默认值管理
- 敏感信息可能泄露

---

### 6. 🟡 日志不完善

**现状**: 无结构化日志

**问题**:
- 无法追踪请求链路
- 无法排查问题
- 无性能指标

---

### 7. 🟡 测试缺失

**现状**: 无任何测试

**问题**:
- 无法保证代码质量
- 重构风险高
- 回归问题多

---

## 改进方案

### Phase 1: 核心 API 完善 (2-3 天)

#### 1. 创建完整的路由结构

```
backend/src/
├── routes/
│   ├── api/
│   │   ├── auth/
│   │   │   └── device.ts          # 已有
│   │   ├── sessions/
│   │   │   ├── index.ts           # GET /api/sessions
│   │   │   └── [id].ts            # GET/PUT/DELETE /api/sessions/:id
│   │   ├── messages/
│   │   │   └── index.ts           # GET/POST /api/messages
│   │   ├── characters/
│   │   │   └── index.ts           # CRUD /api/characters
│   │   └── daemon/
│   │       └── index.ts           # POST /api/daemon/command
│   └── index.tsx                  # 主页
```

#### 2. 实现 Session API

```typescript
// routes/api/sessions/index.ts
import { Route, json } from "@tanstack/start";
import { db } from "~/lib/db";

export const sessionsRoute = new Route({
  path: "/api/sessions",
  method: "GET",
  handler: async (req: Request) => {
    const userId = await getCurrentUserId(req);
    const sessions = await db.sessions.findMany({
      where: { user_id: userId },
      orderBy: { updated_at: "desc" },
    });
    return json({ sessions });
  },
});

export const createSessionRoute = new Route({
  path: "/api/sessions",
  method: "POST",
  handler: async (req: Request) => {
    const userId = await getCurrentUserId(req);
    const { title, character_id } = await req.json();
    
    const session = await db.sessions.create({
      user_id: userId,
      title: title || "新会话",
      character_id,
    });
    
    // 通知 Daemon 创建本地 session
    await notifyDaemon(userId, {
      type: "session:create",
      data: { session_id: session.id },
    });
    
    return json({ session }, { status: 201 });
  },
});
```

#### 3. 实现 Message API

```typescript
// routes/api/messages/index.ts
export const messagesRoute = new Route({
  path: "/api/messages",
  method: "POST",
  handler: async (req: Request) => {
    const userId = await getCurrentUserId(req);
    const { session_id, content } = await req.json();
    
    // 验证 session 属于当前用户
    const session = await db.sessions.findFirst({
      where: { id: session_id, user_id: userId },
    });
    
    if (!session) {
      return json({ error: "Session not found" }, { status: 404 });
    }
    
    // 保存用户消息
    const message = await db.messages.create({
      session_id,
      role: "user",
      content,
    });
    
    // 通过 Centrifugo 发送给 Daemon
    await ws.sendToDaemon(userId, {
      type: "chat:message",
      data: {
        session_id,
        message_id: message.id,
        content,
      },
    });
    
    return json({ message }, { status: 201 });
  },
});
```

---

### Phase 2: 数据库层重构 (2 天)

#### 1. 使用 Prisma ORM

```bash
cd backend
npm install prisma @prisma/client
npx prisma init
```

```prisma
// prisma/schema.prisma

generator client {
  provider = "prisma-client-js"
}

datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}

model User {
  id         String   @id @default(uuid())
  deviceId   String   @unique @map("device_id")
  email      String?  @unique
  createdAt  DateTime @default(now()) @map("created_at")
  updatedAt  DateTime @updatedAt @map("updated_at")
  
  sessions   Session[]
  characters Character[]
  
  @@map("users")
}

model Session {
  id            String    @id @default(uuid())
  userId        String    @map("user_id")
  characterId   String?   @map("character_id")
  title         String?
  status        String    @default("active")
  createdAt     DateTime  @default(now()) @map("created_at")
  updatedAt     DateTime  @updatedAt @map("updated_at")
  
  user          User      @relation(fields: [userId], references: [id])
  character     Character? @relation(fields: [characterId], references: [id])
  messages      Message[]
  
  @@index([userId])
  @@map("sessions")
}

model Message {
  id          String   @id @default(uuid())
  sessionId   String   @map("session_id")
  role        String   // user | assistant | system
  content     String
  tools       Json?    // tool calls
  metadata    Json?    // thinking, timing, etc.
  createdAt   DateTime @default(now()) @map("created_at")
  
  session     Session  @relation(fields: [sessionId], references: [id], onDelete: Cascade)
  
  @@index([sessionId])
  @@index([createdAt])
  @@map("messages")
}

model Character {
  id          String   @id @default(uuid())
  userId      String   @map("user_id")
  name        String
  description String?
  prompt      String?
  avatarUrl   String?  @map("avatar_url")
  createdAt   DateTime @default(now()) @map("created_at")
  
  user        User     @relation(fields: [userId], references: [id], onDelete: Cascade)
  sessions    Session[]
  
  @@map("characters")
}
```

#### 2. 创建类型安全的 DB 客户端

```typescript
// lib/prisma.ts
import { PrismaClient } from "@prisma/client";

const globalForPrisma = global as unknown as { prisma: PrismaClient };

export const prisma = globalForPrisma.prisma || new PrismaClient();

if (process.env.NODE_ENV !== "production") {
  globalForPrisma.prisma = prisma;
}

// 扩展 with 日志
export const db = prisma.$extends({
  query: {
    $allModels: {
      async findMany({ model, operation, args, query }) {
        const start = performance.now();
        const result = await query(args);
        const duration = performance.now() - start;
        
        console.log(`${model}.${operation} took ${duration}ms`);
        return result;
      },
    },
  },
});
```

---

### Phase 3: 错误处理与 API 规范 (1-2 天)

#### 1. 统一错误响应

```typescript
// lib/errors.ts
export class APIError extends Error {
  constructor(
    public code: string,
    public message: string,
    public status: number,
    public details?: Record<string, any>
  ) {
    super(message);
  }
}

export const Errors = {
  UNAUTHORIZED: new APIError("UNAUTHORIZED", "Unauthorized", 401),
  SESSION_NOT_FOUND: new APIError("SESSION_NOT_FOUND", "Session not found", 404),
  INVALID_INPUT: new APIError("INVALID_INPUT", "Invalid input", 400),
  DAEMON_OFFLINE: new APIError("DAEMON_OFFLINE", "Daemon is offline", 503),
};

// 统一响应格式
export function successResponse(data: any) {
  return json({ success: true, data });
}

export function errorResponse(error: APIError, details?: any) {
  return json(
    {
      success: false,
      error: {
        code: error.code,
        message: error.message,
        details,
      },
    },
    { status: error.status }
  );
}
```

#### 2. 全局错误处理

```typescript
// middleware/errorHandler.ts
export async function errorHandler(handler: Function) {
  try {
    return await handler();
  } catch (error) {
    if (error instanceof APIError) {
      return errorResponse(error);
    }
    
    console.error("Unhandled error:", error);
    return errorResponse(
      new APIError("INTERNAL_ERROR", "Internal server error", 500)
    );
  }
}
```

---

### Phase 4: 配置与日志 (1 天)

#### 1. 配置管理

```typescript
// lib/config.ts
import { z } from "zod";

const configSchema = z.object({
  database: z.object({
    url: z.string().url(),
  }),
  centrifugo: z.object({
    url: z.string().url(),
    apiKey: z.string(),
    tokenSecret: z.string(),
  }),
  app: z.object({
    port: z.number().default(3000),
    env: z.enum(["development", "production", "test"]),
    logLevel: z.enum(["debug", "info", "warn", "error"]).default("info"),
  }),
});

export const config = configSchema.parse({
  database: {
    url: process.env.DATABASE_URL,
  },
  centrifugo: {
    url: process.env.CENTRIFUGO_URL,
    apiKey: process.env.CENTRIFUGO_API_KEY,
    tokenSecret: process.env.CENTRIFUGO_TOKEN_SECRET,
  },
  app: {
    port: parseInt(process.env.PORT || "3000"),
    env: process.env.NODE_ENV || "development",
    logLevel: process.env.LOG_LEVEL,
  },
});
```

#### 2. 结构化日志

```typescript
// lib/logger.ts
import pino from "pino";

export const logger = pino({
  level: config.app.logLevel,
  transport:
    config.app.env === "development"
      ? { target: "pino-pretty", options: { colorize: true } }
      : undefined,
});

// 请求日志中间件
export function requestLogger(handler: Function) {
  return async (req: Request) => {
    const start = Date.now();
    const url = new URL(req.url);
    
    logger.info({
      method: req.method,
      path: url.pathname,
      query: url.search,
    }, "Request started");
    
    try {
      const response = await handler(req);
      
      logger.info({
        method: req.method,
        path: url.pathname,
        status: response.status,
        duration: Date.now() - start,
      }, "Request completed");
      
      return response;
    } catch (error) {
      logger.error({
        method: req.method,
        path: url.pathname,
        error: error.message,
        duration: Date.now() - start,
      }, "Request failed");
      
      throw error;
    }
  };
}
```

---

### Phase 5: 测试 (2-3 天)

#### 1. 单元测试

```typescript
// tests/sessions.test.ts
import { describe, it, expect, beforeEach } from "vitest";
import { db } from "~/lib/prisma";

describe("Sessions API", () => {
  beforeEach(async () => {
    await db.session.deleteMany();
  });
  
  it("should create a session", async () => {
    const user = await db.user.create({
      data: { deviceId: "test-device" },
    });
    
    const session = await db.session.create({
      data: {
        userId: user.id,
        title: "Test Session",
      },
    });
    
    expect(session.title).toBe("Test Session");
    expect(session.status).toBe("active");
  });
  
  it("should list user sessions", async () => {
    // ... test
  });
});
```

#### 2. 集成测试

```typescript
// tests/integration/chat.test.ts
import { describe, it, expect } from "vitest";
import { createTestServer } from "~/tests/utils";

describe("Chat Integration", () => {
  it("should send message and receive response", async () => {
    const server = await createTestServer();
    
    // 创建会话
    const session = await server.post("/api/sessions", {
      title: "Test",
    });
    
    // 发送消息
    const message = await server.post("/api/messages", {
      session_id: session.id,
      content: "Hello",
    });
    
    expect(message.role).toBe("user");
    
    // 等待 Daemon 响应 (模拟)
    const response = await server.waitForWebSocketMessage("chat_response");
    expect(response.content).toBeDefined();
  });
});
```

---

## 优先级建议

### P0 (阻塞功能)
1. ✅ Session API (CRUD)
2. ✅ Message API (发送/接收)
3. ✅ Daemon 消息转发

### P1 (用户体验)
1. 数据库 ORM 迁移
2. 错误处理标准化
3. API 文档 (OpenAPI)

### P2 (稳定性)
1. 配置验证
2. 结构化日志
3. 测试覆盖

### P3 (扩展性)
1. 角色/人设 API
2. 文件上传
3. 用户设置

---

## 实施建议

基于你的 Daemon 已经升级完成，建议按以下顺序：

1. **先实现 Session/Message API** - 让前端能调用后端
2. **建立 Backend ↔ Daemon 通信** - 打通整个链路
3. **迁移到 Prisma** - 提升开发体验
4. **添加测试** - 保证质量

需要我开始实施哪个部分？
