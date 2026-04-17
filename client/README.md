# Loom Client

Loom Client 是基于 React + TanStack Start 的前端应用，提供 AI 对话界面、lofi 音乐播放和氛围感 UI。

## 功能特性

- **Centrifugo 集成**: 通过 Centrifugo 与后端实时通信
- **分层 UI**: 背景层、角色层、对话层、控制层
- **流式消息**: 支持思考状态、工具调用显示
- **lofi 音乐**: 集成在线电台播放
- **响应式设计**: 适配不同屏幕尺寸

## 快速开始

### 安装依赖

```bash
cd client
bun install
```

### 环境变量

创建 `.env` 文件：

```bash
VITE_API_URL=http://localhost:3000
VITE_WS_URL=ws://localhost:8000
```

### 运行

```bash
bun run dev
```

## 项目结构

```
client/
├── src/
│   ├── components/           # UI 组件
│   ├── hooks/                # React Hooks
│   └── lib/                  # 工具库
├── package.json
└── vite.config.ts
```

## 许可证

MIT
