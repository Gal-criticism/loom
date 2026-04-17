# Loom 前端改进计划

## 当前问题分析

### 1. 🎨 样式与 UI 问题

**现状**:
- 全部使用 inline styles，难以维护
- 无设计系统，颜色/字体硬编码
- 无响应式设计，移动端体验差
- 缺少动画和过渡效果

**影响**:
```tsx
// 现在的代码 - 难以维护
<div style={{
  padding: "8px 12px",
  borderRadius: "8px",
  background: msg.role === "user" ? "#e94560" : "#2a2a4e",
  // ... 每次修改要改多处
}}>
```

---

### 2. 🏗️ 架构问题

**现状**:
- 无状态管理库，仅靠 useState
- 组件职责不清
- 无错误边界，出错直接白屏
- 类型定义松散

**文件结构混乱**:
```
components/
├── BackgroundLayer.tsx    # 空壳组件
├── CharacterLayer.tsx     # 占位符
├── ChatLayer.tsx          # 只是包裹
├── Chat.tsx               # 实际逻辑在这里
├── ControlLayer.tsx       # 简单的播放器
└── Player.tsx             # 另一个播放器？
```

---

### 3. ⚡ 功能缺失

**核心功能不全**:
- ❌ 无会话历史列表
- ❌ 无会话切换/创建
- ❌ 无设置界面
- ❌ 无消息搜索
- ❌ 无代码高亮
- ❌ 无文件上传
- ❌ 无思考状态显示

**用户体验差**:
- ❌ 消息滚动不自动
- ❌ 无消息时间戳
- ❌ 无复制消息按钮
- ❌ 无重发/删除功能

---

### 4. 🔌 后端集成问题

**现状**:
- WebSocket 连接简单，无重连
- 无请求/响应拦截
- 错误处理缺失
- 无加载状态管理

---

## 改进方案

### Phase A: 基础架构 (1-2 天)

#### 1. 引入 Tailwind CSS + shadcn/ui

```bash
# 初始化 shadcn/ui
cd client
npx shadcn@latest init

# 安装常用组件
npx shadcn add button input card scroll-area
npx shadcn add avatar badge tooltip
```

**收益**:
- 统一设计系统
- 响应式支持
- 暗黑模式内置
- 组件可复用

#### 2. 引入 Zustand 状态管理

```typescript
// stores/chat.ts
import { create } from 'zustand'

interface ChatState {
  sessions: Session[]
  currentSessionId: string | null
  messages: Message[]
  isLoading: boolean
  thinking: boolean
  
  // Actions
  setCurrentSession: (id: string) => void
  sendMessage: (content: string) => Promise<void>
  createSession: () => Promise<void>
  loadSessions: () => Promise<void>
}

export const useChatStore = create<ChatState>((set, get) => ({
  // ... implementation
}))
```

**收益**:
- 全局状态管理
- 跨组件通信
- 持久化支持
- 更好的性能

#### 3. 重构项目结构

```
client/src/
├── components/
│   ├── ui/                    # shadcn/ui 组件
│   ├── chat/
│   │   ├── ChatContainer.tsx
│   │   ├── MessageList.tsx
│   │   ├── MessageItem.tsx
│   │   ├── ChatInput.tsx
│   │   └── ThinkingIndicator.tsx
│   ├── sessions/
│   │   ├── SessionList.tsx
│   │   ├── SessionItem.tsx
│   │   └── NewSessionButton.tsx
│   ├── layout/
│   │   ├── Sidebar.tsx
│   │   ├── Header.tsx
│   │   └── MainContent.tsx
│   └── common/
│       ├── ErrorBoundary.tsx
│       ├── LoadingSpinner.tsx
│       └── ThemeToggle.tsx
├── hooks/
│   ├── useWebSocket.ts        # 增强版 WebSocket
│   ├── useSession.ts
│   └── useChat.ts
├── stores/
│   ├── chatStore.ts
│   ├── sessionStore.ts
│   └── settingsStore.ts
├── lib/
│   ├── utils.ts
│   ├── api.ts                 # API 封装
│   └── websocket.ts           # 增强 WebSocket
├── types/
│   └── index.ts
└── styles/
    └── globals.css
```

---

### Phase B: 核心功能 (2-3 天)

#### 1. 会话管理 UI

```tsx
// components/sessions/SessionList.tsx
export function SessionList() {
  const { sessions, currentSessionId, setCurrentSession } = useSessionStore()
  
  return (
    <div className="w-64 h-full border-r bg-muted/50">
      <div className="p-4 border-b">
        <Button onClick={createSession} className="w-full">
          <Plus className="w-4 h-4 mr-2" />
          新会话
        </Button>
      </div>
      
      <ScrollArea className="h-[calc(100vh-80px)]">
        {sessions.map(session => (
          <SessionItem
            key={session.id}
            session={session}
            isActive={session.id === currentSessionId}
            onClick={() => setCurrentSession(session.id)}
          />
        ))}
      </ScrollArea>
    </div>
  )
}
```

**功能**:
- 会话列表展示
- 新建/切换/删除会话
- 会话标题编辑
- 搜索历史会话

#### 2. 增强聊天界面

```tsx
// components/chat/MessageItem.tsx
export function MessageItem({ message }: { message: Message }) {
  return (
    <div className={cn(
      "flex gap-4 p-4",
      message.role === "assistant" && "bg-muted/50"
    )}>
      <Avatar className="w-8 h-8">
        {message.role === "user" ? <UserIcon /> : <BotIcon />}
      </Avatar>
      
      <div className="flex-1 space-y-2">
        <div className="flex items-center gap-2">
          <span className="font-semibold">
            {message.role === "user" ? "你" : "AI"}
          </span>
          <span className="text-xs text-muted-foreground">
            {formatTime(message.timestamp)}
          </span>
        </div>
        
        <div className="prose dark:prose-invert max-w-none">
          {message.role === "assistant" ? (
            <MarkdownContent content={message.content} />
          ) : (
            <p>{message.content}</p>
          )}
        </div>
        
        <MessageActions message={message} />
      </div>
    </div>
  )
}
```

**功能**:
- Markdown 渲染
- 代码高亮 + 复制
- 消息操作（复制、重发、删除）
- 时间戳显示

#### 3. 思考状态显示

```tsx
// components/chat/ThinkingIndicator.tsx
export function ThinkingIndicator() {
  const { thinking, thinkingText } = useChatStore()
  
  if (!thinking) return null
  
  return (
    <div className="flex items-center gap-2 p-4 text-muted-foreground">
      <Loader2 className="w-4 h-4 animate-spin" />
      <span className="text-sm">{thinkingText || "AI 正在思考..."}</span>
      
      {/* 显示正在使用的工具 */}
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger>
            <Badge variant="secondary">Bash</Badge>
          </TooltipTrigger>
          <TooltipContent>
            <p>正在执行: ls -la</p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>
  )
}
```

---

### Phase C: 增强功能 (1-2 天)

#### 1. 设置界面

```tsx
// app/settings/page.tsx
export default function SettingsPage() {
  return (
    <div className="container max-w-2xl py-8">
      <h1 className="text-3xl font-bold mb-8">设置</h1>
      
      <Tabs defaultValue="general">
        <TabsList>
          <TabsTrigger value="general">通用</TabsTrigger>
          <TabsTrigger value="appearance">外观</TabsTrigger>
          <TabsTrigger value="about">关于</TabsTrigger>
        </TabsList>
        
        <TabsContent value="general">
          <GeneralSettings />
        </TabsContent>
        
        <TabsContent value="appearance">
          <AppearanceSettings />
        </TabsContent>
      </Tabs>
    </div>
  )
}
```

**功能**:
- 主题切换（暗黑/亮色）
- 字体大小
- 语言设置
- 快捷键配置

#### 2. 错误处理与重试

```tsx
// hooks/useWebSocket.ts
export function useWebSocket() {
  const [status, setStatus] = useState<'connected' | 'disconnected' | 'reconnecting'>('disconnected')
  
  useEffect(() => {
    const ws = new WebSocket(url)
    
    ws.onclose = () => {
      setStatus('disconnected')
      // 自动重连
      setTimeout(connect, 3000)
    }
    
    ws.onerror = (error) => {
      toast.error('连接失败，正在重试...')
    }
  }, [])
  
  return { status, send }
}
```

---

## 优先级建议

### P0 (必须)
1. ✅ 引入 Tailwind + shadcn/ui
2. ✅ 重构项目结构
3. ✅ 增强聊天界面（Markdown、代码高亮）
4. ✅ 错误边界和加载状态

### P1 (重要)
1. 会话管理 UI
2. 思考状态显示
3. 响应式布局
4. WebSocket 重连

### P2 (可选)
1. 设置界面
2. 文件上传
3. 消息搜索
4. PWA 支持

---

## 快速开始

你想先解决哪个问题？推荐顺序：

1. **先引入 Tailwind** - 1 小时，立即提升视觉效果
2. **重构项目结构** - 2 小时，便于后续开发
3. **增强聊天界面** - 半天，核心体验提升
4. **会话管理 UI** - 半天，连接后端新功能

需要我帮你开始实施吗？
