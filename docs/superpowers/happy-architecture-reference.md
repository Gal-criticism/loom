# Happy CLI 架构设计参考

本文档分析 Happy CLI 的架构设计，为 Loom 的升级改造提供参考。

## 1. 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Happy CLI (Node.js/TS)                       │
├─────────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │   Claude     │  │    Codex     │  │  Gemini/OpenClaw/ACP     │  │
│  │  Subsystem   │  │  Subsystem   │  │     Subsystems           │  │
│  └──────┬───────┘  └──────┬───────┘  └───────────┬──────────────┘  │
│         │                 │                      │                  │
│  ┌──────▼─────────────────▼──────────────────────▼──────────────┐  │
│  │                    Session Management                          │  │
│  │              (统一会话协议 + 消息队列)                          │  │
│  └────────────────────────┬──────────────────────────────────────┘  │
│                           │                                        │
│  ┌────────────────────────▼──────────────────────────────────────┐  │
│  │                      Daemon (常驻后台)                          │  │
│  │         管理会话生命周期 + 远程连接保活 + Push通知               │  │
│  └────────────────────────┬──────────────────────────────────────┘  │
└───────────────────────────┼─────────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────────┐
│                    Happy Server (Fastify + Socket.io)                │
│                   端到端加密 + 设备同步 + 会话中继                      │
└─────────────────────────────────────────────────────────────────────┘
```

## 2. 核心模块详解

### 2.1 双模式架构（本地/远程）

Happy 为每个 Runtime 实现了**双模式运行**，这是其核心设计之一：

```typescript
// 主循环在两种模式间无缝切换
export async function loop(opts: LoopOptions) {
    let mode: 'local' | 'remote' = opts.startingMode ?? 'local';
    while (true) {
        switch (mode) {
            case 'local': {
                const result = await claudeLocalLauncher(session);
                if (result.type === 'switch') mode = 'remote';
                break;
            }
            case 'remote': {
                const reason = await claudeRemoteLauncher(session);
                if (reason === 'switch') mode = 'local';
                break;
            }
        }
    }
}
```

**本地模式特点**：
- 直接 spawn Claude Code 进程
- 用户在本机终端交互
- 通过文件系统钩子获取 session ID
- 拦截 fetch 请求跟踪思考状态（通过 fd 3）

**远程模式特点**：
- 通过 SDK 调用 Claude Code
- 使用 Ink 渲染 UI 显示"手机控制中"
- 消息从手机 App 通过 WebSocket 中继
- 支持权限管理（工具调用需批准）

### 2.2 Session 抽象层

Happy 实现了统一的 Session 管理：

```typescript
export class Session {
    sessionId: string | null;
    messageQueue: MessageQueue2<EnhancedMode>;
    client: ApiSessionClient;  // 与服务器通信

    onSessionFound(sessionId: string) {
        this.sessionId = sessionId;
        this.client.registerClaudeSession(sessionId);
    }

    onThinkingChange(thinking: boolean) {
        this.client.updateAgentState({ thinking });
    }

    async switchMode() {
        this.queue.reset();
        this.rpcHandlerManager.trigger('switch');
    }
}
```

关键设计：
- **MessageQueue2**: 带模式的消息队列，支持隔离和清空
- **RPC Handler**: 处理 abort/switch 等控制命令
- **Session ID 跟踪**: 支持 --resume 恢复会话

### 2.3 Daemon 常驻服务

Daemon 是 Happy 的核心后台服务：

```typescript
export async function startDaemon(): Promise<void> {
    // 1. 获取独占锁（防止多个 daemon 运行）
    const daemonLockHandle = await acquireDaemonLock(5, 200);
    
    // 2. 启动控制服务器（Unix Socket/HTTP）
    const { port: controlPort, stop: stopControlServer } = 
        await startDaemonControlServer({
            getChildren: getCurrentChildren,
            stopSession,
            spawnSession,
            requestShutdown,
            onHappySessionWebhook
        });
    
    // 3. 注册机器到服务器
    const machine = await api.getOrCreateMachine({
        machineId,
        metadata: initialMachineMetadata,
        daemonState: initialDaemonState
    });
    
    // 4. 创建实时机器会话
    const apiMachine = api.machineSyncClient(machine);
    apiMachine.setRPCHandlers({
        spawnSession,    // 手机远程启动会话
        resumeSession,   // 恢复会话
        stopSession,     // 停止会话
        requestShutdown
    });
    
    // 5. 启动心跳和版本检查
    const restartOnStaleVersionAndHeartbeat = setInterval(async () => {
        // 清理僵尸会话
        // 检查 daemon 版本，过时则自动重启
        // 写入心跳状态
    }, 60000);
}
```

Daemon 职责：
- **会话管理**: spawn/stop/resume 会话
- **进程跟踪**: 通过 PID 跟踪子进程生命周期
- **Webhook 接收**: 接收 CLI 会话报告
- **Push 通知**: 发送通知到手机
- **自动更新**: 检测 CLI 版本变化，自动重启

### 2.4 API 客户端架构

Happy 使用分层 API 设计：

```typescript
// ApiClient - 管理机器和会话创建
export class ApiClient {
    async getOrCreateSession(opts: {...}): Promise<Session | null>
    async getOrCreateMachine(opts: {...}): Promise<Machine>
    sessionSyncClient(session: Session): ApiSessionClient
    machineSyncClient(machine: Machine): ApiMachineClient
}

// ApiSessionClient - 会话级实时同步
export class ApiSessionClient extends EventEmitter {
    private socket: Socket<...>;
    
    // 发送消息
    sendClaudeSessionMessage(body: RawJSONLines)
    sendAgentMessage(provider, body: ACPMessageData)
    
    // 状态更新
    updateMetadata(handler)
    updateAgentState(handler)
    
    // 生命周期
    closeClaudeSessionTurn(status)
    sendSessionDeath()
}

// ApiMachineClient - 机器级 RPC
export class ApiMachineClient {
    setRPCHandlers(handlers: {
        spawnSession: ...,
        stopSession: ...,
        resumeSession: ...,
    })
    connect()
}
```

通信机制：
- **REST API**: 创建/获取会话和机器
- **Socket.io**: 实时双向通信
- **端到端加密**: 使用 libsodium + AES-256-GCM

### 2.5 加密系统

Happy 实现了完整的端到端加密：

```typescript
// 加密变体
export type EncryptionVariant = 'legacy' | 'dataKey';

// Legacy: 使用 tweetnacl secretbox
export function encryptLegacy(data: any, secret: Uint8Array): Uint8Array
export function decryptLegacy(data: Uint8Array, secret: Uint8Array): any

// DataKey: 使用 AES-256-GCM
export function encryptWithDataKey(data: any, dataKey: Uint8Array): Uint8Array
export function decryptWithDataKey(bundle: Uint8Array, dataKey: Uint8Array): any

// 公钥加密（用于传输数据密钥）
export function libsodiumEncryptForPublicKey(
    data: Uint8Array, 
    recipientPublicKey: Uint8Array
): Uint8Array
```

密钥管理：
- 每个会话生成独立的数据加密密钥
- 使用服务器公钥加密数据密钥
- 服务器无法解密内容，仅作为中继

### 2.6 消息协议 (ACP)

Happy 定义了统一的 Agent Communication Protocol：

```typescript
export type ACPMessageData =
    // 核心消息
    | { type: 'message'; message: string }
    | { type: 'reasoning'; message: string }
    | { type: 'thinking'; text: string }
    // 工具调用
    | { type: 'tool-call'; callId: string; name: string; input: unknown }
    | { type: 'tool-result'; callId: string; output: unknown; isError?: boolean }
    // 文件操作
    | { type: 'file-edit'; description: string; filePath: string; diff?: string }
    // 任务生命周期
    | { type: 'task_started'; id: string }
    | { type: 'task_complete'; id: string }
    // 权限
    | { type: 'permission-request'; permissionId: string; toolName: string }
    // 用量统计
    | { type: 'token_count'; ... };
```

协议特点：
- **提供者无关**: Claude、Codex、Gemini 统一格式
- **版本兼容**: 支持协议演进
- **端到端加密**: 消息内容加密传输

### 2.7 离线重连机制

Happy 实现了智能离线处理：

```typescript
export function startOfflineReconnection(opts: {
    serverUrl: string;
    onReconnected: () => Promise<{ session, scanner }>;
    onNotify: (msg: string) => void;
    onCleanup: () => void;
}) {
    // 1. 通知用户离线状态
    // 2. 指数退避重试连接
    // 3. 重连成功后恢复会话
    // 4. 同步离线期间的消息
}
```

连接状态管理：
```typescript
export const connectionState = {
    fail(opts: { operation, errorCode, url, details }): void
    notifyOffline(): void
    isNetworkError(errorCode: string): boolean
}
```

### 2.8 权限管理系统

Happy 实现了细粒度的权限控制：

```typescript
export type PermissionMode = 
    | 'default'           // 每次询问
    | 'acceptEdits'       // 自动接受编辑
    | 'bypassPermissions' // 跳过权限（危险）
    | 'plan'              // 计划模式
    | 'yolo'              // 完全自动（--dangerously-skip-permissions）

// 权限处理器
export class PermissionHandler {
    async handleToolCall(toolName, input, mode, options): Promise<PermissionResult>
    setOnPermissionRequest(callback)
    getResponses(): Map<string, PermissionResponse>
}
```

权限流程：
1. 工具调用时检查当前权限模式
2. 如果需要批准，发送通知到手机
3. 等待用户响应（approve/deny）
4. 记录权限决策，用于审计

## 3. 关键技术点

### 3.1 进程间通信

| 方式 | 用途 |
|------|------|
| **文件系统钩子** | Claude 写入 session 文件，Happy 监听获取 session ID |
| **fd 3 (pipe)** | launcher 向父进程发送思考状态 JSON |
| **HTTP Hook Server** | Claude 的 SessionStart hook 通知 Happy |
| **Socket.io** | 与 Happy Server 实时通信 |
| **Unix Socket/IPC** | Daemon 与 CLI 进程通信 |

### 3.2 会话恢复机制

Happy 支持多种会话恢复方式：

```typescript
// 1. 通过 --resume 标志
claude --resume <session-id>

// 2. 通过 --continue 标志（恢复上一个会话）
claude --continue

// 3. 通过 happy resume 命令
happy resume <happy-session-id>

// 4. Daemon 远程恢复
apiMachine.resumeSession(happySessionId)
```

### 3.3 沙箱支持

Happy 支持 OS-level 沙箱（macOS/Linux）：

```typescript
export interface SandboxConfig {
    enabled: boolean;
    workspaceRoot?: string;
    networkMode?: 'allow' | 'deny' | 'local-only';
}

// 初始化沙箱
const cleanupSandbox = await initializeSandbox(sandboxConfig, workingDirectory);

// 包装命令在沙箱中运行
const wrappedCommand = await wrapCommand(fullCommand);
```

### 3.4 tmux 集成

Happy 支持在 tmux 会话中 spawn：

```typescript
const tmux = getTmuxUtilities(tmuxSessionName);
const result = await tmux.spawnInTmux(
    [command],
    { sessionName, windowName, cwd },
    env
);
```

## 4. 与 Loom 的差异总结

| 方面 | Happy | Loom |
|------|-------|------|
| **Runtime 控制** | 深度集成，支持权限管理、工具控制 | 简单包装，透传命令 |
| **会话管理** | 复杂的 session 跟踪和恢复 | 基础会话记录 |
| **双模式** | 本地/远程无缝切换 | 仅本地模式 |
| **Hook 机制** | 文件系统 + HTTP hooks | 暂无 |
| **消息协议** | 完善的 envelope + ACP 协议 | 简单 JSON |
| **SDK 依赖** | 使用 @anthropic-ai/claude-agent-sdk | 直接调用 CLI |
| **加密** | 端到端加密 | 无加密 |
| **Daemon** | 常驻后台服务 | 单次运行 |
| **离线处理** | 智能重连 | 无 |
| **Push 通知** | 支持 | 无 |
| **沙箱** | 支持 | 无 |

## 5. 可借鉴的设计模式

### 5.1 Session 管理
- 使用统一的 Session 类封装状态
- 消息队列处理异步消息
- 支持会话恢复和模式切换

### 5.2 进程管理
- Daemon 常驻服务管理子进程生命周期
- PID 跟踪和僵尸进程清理
- 优雅退出处理（SIGTERM/SIGINT）

### 5.3 通信协议
- 定义清晰的消息信封格式
- 版本化和加密支持
- 双向 RPC 机制

### 5.4 错误处理
- 网络错误的分类处理
- 指数退避重试
- 离线模式降级

### 5.5 配置管理
- 分层配置（全局/项目/会话）
- 环境变量扩展
- 持久化状态管理

## 6. 参考文献

- [Happy CLI GitHub](https://github.com/slopus/happy)
- [Claude Code SDK](https://www.npmjs.com/package/@anthropic-ai/claude-agent-sdk)
- [Socket.io](https://socket.io/)
- [libsodium](https://libsodium.gitbook.io/)
