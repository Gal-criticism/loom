# Loom 本地开发指南

## 快速开始

### 1. 克隆项目
```bash
git clone https://github.com/Gal-criticism/loom.git
cd loom
```

### 2. 安装依赖
```bash
# 安装后端依赖
cd backend && bun install && cd ..

# 安装前端依赖
cd client && bun install && cd ..
```

### 3. 启动开发环境

**方式 A：使用脚本（推荐）**
```bash
chmod +x scripts/dev.sh
./scripts/dev.sh
```

**方式 B：手动启动**
```bash
# 1. 启动 Docker 服务
docker-compose up -d

# 2. 启动 Backend
cd backend && bun run dev

# 3. 启动 Client（新终端）
cd client && bun run dev
```

### 4. 访问

| 服务 | 地址 |
|------|------|
| Client | http://localhost:8080 |
| Backend | http://localhost:3000 |
| WebSocket | ws://localhost:8000 |

---

## 环境配置

### 开发环境变量

复制环境变量模板并修改：
```bash
cp .env.example .env.local
```

编辑 `.env.local` 配置本地参数。

### Docker 服务

手动管理 Docker 容器：
```bash
# 启动
docker-compose up -d

# 查看状态
docker-compose ps

# 停止
docker-compose down

# 查看日志
docker-compose logs -f
```

---

## 项目结构

```
loom/
├── docker-compose.yml    # Docker 编排配置
├── .env.example          # 环境变量模板
├── Makefile              # 构建命令
│
├── cmd/daemon/           # Go CLI (用户本地运行)
│   ├── main.go
│   ├── api/              # HTTP API
│   ├── cmd/              # CLI 命令
│   ├── config/           # 配置管理
│   ├── runtime/          # Runtime 适配器
│   └── ws/               # WebSocket 客户端
│
├── backend/              # TanStack Start + Bun
│   └── src/
│       ├── lib/          # 工具库 (db, ws)
│       └── routes/        # API 路由
│
└── client/               # React 前端
    └── src/
        ├── components/   # UI 组件
        ├── hooks/        # React Hooks
        └── lib/          # 工具库
```

---

## 常见问题

### Docker 无法启动
```bash
# 检查 Docker 状态
docker info

# 重启 Docker
brew services restart docker
```

### 端口被占用
检查并关闭占用端口的程序：
```bash
lsof -i :5432  # PostgreSQL
lsof -i :8000  # Centrifugo
lsof -i :3000  # Backend
lsof -i :8080  # Client
```

### 数据库连接失败
确保 PostgreSQL 容器正常运行：
```bash
docker-compose logs postgres
```

---

## 下一步

1. 安装并运行 Daemon：
   ```bash
   cd cmd/daemon
   go build -o loomd .
   ./loomd start
   ```

2. 配置 Backend 生产环境：
   ```bash
   cp .env.example .env
   # 编辑 .env 填入生产配置
   ```
