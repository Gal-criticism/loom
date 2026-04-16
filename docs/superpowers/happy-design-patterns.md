# Happy CLI 设计模式速查

快速参考 Happy CLI 中的关键设计模式，便于在 Loom 升级中应用。

## 1. 双模式状态机模式

Happy 的核心设计是本地/远程模式无缝切换。

```typescript
// 状态机循环
async function loop(opts: LoopOptions) {
    let mode: 'local' | 'remote' = opts.startingMode ?? 'local';
    
    while (true) {
        switch (mode) {
            case 'local': {
                const result = await claudeLocalLauncher(session);
                if (result.type === 'switch') mode = 'remote';
                else if (result.type === 'exit') return result.code;
                break;
            }
            case 'remote': {
                const reason = await claudeRemoteLauncher(session);
                if (reason === 'switch') mode = 'local';
                else if (reason === 'exit') return 0;
                break;
            }
        }
    }
}
```

**要点**：
- 模式切换通过 launcher 返回值控制
- 共享 Session 对象保持状态
- 清理工作放在 finally 块

## 2. Session 封装模式

将所有会话状态封装到 Session 类：

```typescript
class Session {
    sessionId: string | null;
    messageQueue: MessageQueue<Mode>;
    client: ApiSessionClient;
    
    // 状态变更通知
    onSessionFound(sessionId: string) {
        this.sessionId = sessionId;
        this.client.registerSession(sessionId);
    }
    
    onThinkingChange(thinking: boolean) {
        this.client.updateAgentState({ thinking });
    }
    
    // 模式切换
    async switchMode() {
        this.queue.reset();
        this.rpcHandlerManager.trigger('switch');
    }
}
```

**要点**：
- Session ID 延迟获取（从 hook 或文件系统）
- 使用 EventEmitter 通知状态变更
- 封装队列和 RPC 管理

## 3. Message Queue 模式

带模式的消息队列，支持隔离和清空：

```typescript
class MessageQueue2<Mode> {
    private queue: Array<{ message: string; mode: Mode; hash: string }> = [];
    private onMessage: ((msg: string, mode: Mode) => void) | null = null;
    
    push(message: string, mode: Mode) {
        const hash = hashObject(mode);
        this.queue.push({ message, mode, hash });
        this.flush();
    }
    
    pushIsolateAndClear(message: string, mode: Mode) {
        // 清空队列，放入新模式的消息
        this.queue = [];
        this.push(message, { ...mode, isolate: true });
    }
    
    reset() {
        this.queue = [];
    }
}
```

**要点**：
- 模式变化时隔离队列
- hash 用于检测模式是否变化
- 支持异步等待消息

## 4. RPC Handler 模式

轻量级 RPC 机制：

```typescript
class RpcHandlerManager {
    private handlers = new Map<string, (...args: any[]) => Promise<any>>();
    
    registerHandler(method: string, handler: (...args: any[]) => Promise<any>) {
        this.handlers.set(method, handler);
    }
    
    async handleRequest(data: { method: string; params: string }): Promise<string> {
        const handler = this.handlers.get(data.method);
        if (!handler) throw new Error(`Unknown method: ${data.method}`);
        
        const params = JSON.parse(data.params);
        const result = await handler(...params);
        return JSON.stringify(result);
    }
    
    trigger(method: string, ...args: any[]) {
        const handler = this.handlers.get(method);
        if (handler) handler(...args);
    }
}
```

**要点**：
- 方法名映射到 handler
- 支持同步/异步 handler
- 用于 abort/switch 等控制命令

## 5. Daemon 进程管理模式

Daemon 管理子进程生命周期的模式：

```typescript
class Daemon {
    private pidToSession = new Map<number, TrackedSession>();
    private pidToAwaiter = new Map<number, (session: TrackedSession) => void>();
    
    async spawnSession(opts: SpawnOptions): Promise<SpawnResult> {
        // 1. Spawn 进程
        const child = spawn('node', args, { detached: true });
        
        // 2. 创建跟踪记录
        const trackedSession: TrackedSession = {
            pid: child.pid,
            startedBy: 'daemon',
            // ...
        };
        this.pidToSession.set(child.pid, trackedSession);
        
        // 3. 等待 webhook 确认
        return new Promise((resolve) => {
            const timeout = setTimeout(() => {
                resolve({ type: 'error', errorMessage: 'Timeout' });
            }, 15000);
            
            this.pidToAwaiter.set(child.pid, (completedSession) => {
                clearTimeout(timeout);
                resolve({ type: 'success', sessionId: completedSession.sessionId });
            });
        });
    }
    
    // 接收 CLI webhook
    onHappySessionWebhook(sessionId: string, metadata: Metadata) {
        const pid = metadata.hostPid;
        const session = this.pidToSession.get(pid);
        const awaiter = this.pidToAwaiter.get(pid);
        
        if (session && awaiter) {
            session.sessionId = sessionId;
            awaiter(session);
        }
    }
}
```

**要点**：
- PID 作为临时标识
- Webhook 确认机制
- Awaiter 模式等待异步事件

## 6. 离线重连模式

智能离线处理和自动重连：

```typescript
function startOfflineReconnection(opts: {
    serverUrl: string;
    onReconnected: () => Promise<void>;
    onNotify: (msg: string) => void;
}) {
    let attempt = 0;
    const maxDelay = 30000; // 最大 30s
    
    const tryReconnect = async () => {
        // 指数退避
        const delay = Math.min(1000 * Math.pow(2, attempt), maxDelay);
        await sleep(delay);
        
        try {
            await opts.onReconnected();
            opts.onNotify('Reconnected!');
            return;
        } catch (error) {
            attempt++;
            opts.onNotify(`Reconnecting... (attempt ${attempt})`);
            tryReconnect();
        }
    };
    
    tryReconnect();
    
    return {
        cancel: () => { /* 停止重连 */ }
    };
}
```

**要点**：
- 指数退避避免频繁请求
- 用户通知保持透明
- 可取消的重连任务

## 7. Session Scanner 模式

监听文件系统获取 Session ID：

```typescript
class SessionScanner {
    private watcher: FSWatcher | null = null;
    private sessionId: string | null = null;
    
    async start(workingDirectory: string, onSessionFound: (id: string) => void) {
        const projectDir = getProjectPath(workingDirectory);
        
        // 1. 检查已存在的 session
        const existing = await this.findExistingSession(projectDir);
        if (existing) {
            this.sessionId = existing;
            onSessionFound(existing);
        }
        
        // 2. 监听新 session 文件
        this.watcher = watch(projectDir, async (eventType, filename) => {
            if (filename?.endsWith('.jsonl')) {
                const sessionId = filename.replace('.jsonl', '');
                if (isValidSessionId(sessionId)) {
                    this.sessionId = sessionId;
                    onSessionFound(sessionId);
                }
            }
        });
    }
    
    onNewSession(sessionId: string) {
        // 外部通知新 session（如通过 hook）
        this.sessionId = sessionId;
    }
}
```

**要点**：
- 双重检测：现有文件 + 实时监控
- 文件命名约定识别 session
- 支持外部触发（hook）

## 8. 端到端加密模式

数据加密流程：

```typescript
// 1. 客户端生成数据密钥
const dataKey = getRandomBytes(32);

// 2. 用服务器公钥加密数据密钥
const encryptedDataKey = libsodiumEncryptForPublicKey(
    dataKey,
    serverPublicKey
);

// 3. 用数据密钥加密数据
const encrypted = encryptWithDataKey(data, dataKey);

// 4. 发送加密后的数据密钥和数据
await axios.post('/v1/sessions', {
    data: encodeBase64(encrypted),
    dataEncryptionKey: encodeBase64(encryptedDataKey)
});

// 解密流程（只有拥有私钥的接收方可以解密）
const decryptedDataKey = libsodiumDecryptWithPrivateKey(
    encryptedDataKey,
    privateKey
);
const data = decryptWithDataKey(encrypted, decryptedDataKey);
```

**要点**：
- 每个会话独立密钥
- 公钥加密传输密钥
- 对称加密传输数据

## 9. Thinking 状态跟踪

通过 fd 3 跟踪 AI 思考状态：

```typescript
// launcher 脚本 (claude_local_launcher.cjs)
const originalFetch = global.fetch;
global.fetch = async (...args) => {
    const id = generateId();
    
    // 发送开始信号到 fd 3
    process.stderr.write(JSON.stringify({
        type: 'fetch-start',
        id,
        hostname: new URL(args[0]).hostname,
        timestamp: Date.now()
    }) + '\n');
    
    try {
        const response = await originalFetch(...args);
        return response;
    } finally {
        // 发送结束信号
        process.stderr.write(JSON.stringify({
            type: 'fetch-end',
            id
        }) + '\n');
    }
};

// 父进程监听
const rl = createInterface({ input: child.stdio[3] });
rl.on('line', (line) => {
    const msg = JSON.parse(line);
    if (msg.type === 'fetch-start') onThinkingChange(true);
    if (msg.type === 'fetch-end') onThinkingChange(false);
});
```

**要点**：
- 使用额外的 fd 通信
- 拦截 fetch 检测活动
- 超时处理避免状态卡住

## 10. 优雅退出模式

确保资源正确清理：

```typescript
async function runWithCleanup(main: () => Promise<void>) {
    const cleanupHandlers: Array<() => Promise<void>> = [];
    
    const registerCleanup = (handler: () => Promise<void>) => {
        cleanupHandlers.push(handler);
    };
    
    const cleanup = async (signal: string) => {
        console.log(`Received ${signal}, cleaning up...`);
        for (const handler of cleanupHandlers.reverse()) {
            try {
                await handler();
            } catch (e) {
                console.error('Cleanup error:', e);
            }
        }
        process.exit(0);
    };
    
    process.on('SIGINT', () => cleanup('SIGINT'));
    process.on('SIGTERM', () => cleanup('SIGTERM'));
    process.on('uncaughtException', (e) => {
        console.error('Uncaught:', e);
        cleanup('exception');
    });
    
    try {
        await main();
    } finally {
        await cleanup('normal');
    }
}

// 使用
runWithCleanup(async (registerCleanup) => {
    const server = await startServer();
    registerCleanup(async () => await server.stop());
    
    const session = await createSession();
    registerCleanup(async () => await session.close());
    
    // ... 主逻辑
});
```

**要点**：
- 注册表模式管理清理函数
- 信号处理确保优雅退出
- finally 块保证执行

## 11. Version Check & Auto-Restart

Daemon 版本检查自动重启：

```typescript
const heartbeatInterval = setInterval(async () => {
    // 检查版本是否变化
    const diskVersion = readPackageJson().version;
    if (diskVersion !== currentVersion) {
        // 触发新版本 daemon
        spawn('happy', ['daemon', 'start'], { detached: true });
        
        // 自我了断
        clearInterval(heartbeatInterval);
        await cleanup();
        process.exit(0);
    }
    
    // 写入心跳
    writeDaemonState({
        ...state,
        lastHeartbeat: new Date().toISOString()
    });
}, 60000);
```

**要点**：
- 心跳间隔检查版本
- 启动新 daemon 后自我退出
- 状态文件同步

## 快速应用指南

在 Loom 中应用这些模式：

1. **Daemon 改造**: 使用 Pattern 5 (Daemon 进程管理) + Pattern 10 (优雅退出)
2. **通信协议**: 使用 Pattern 4 (RPC Handler) + Pattern 8 (加密)
3. **Session 管理**: 使用 Pattern 2 (Session 封装) + Pattern 7 (Session Scanner)
4. **离线处理**: 使用 Pattern 6 (离线重连)
5. **状态跟踪**: 使用 Pattern 9 (Thinking 状态)
