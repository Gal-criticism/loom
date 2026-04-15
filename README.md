# Loom

具有"氛围感"的虚拟陪伴产品，通过本地 AI Runtime（Claude Code、OpenCode）提供 AI 对话能力。

## 快速开始

### 1. 克隆项目
```bash
git clone https://github.com/Gal-criticism/loom.git
cd loom
```

### 2. 启动开发环境
```bash
# 一键启动（推荐）
chmod +x scripts/dev.sh
./scripts/dev.sh
```

或手动启动：
```bash
docker-compose up --build -d
```

### 3. 访问
| 服务 | 地址 |
|------|------|
| Client | http://localhost:8080 |
| Backend | http://localhost:3000 |
| WebSocket | ws://localhost:8000 |

---

## 项目结构

```
loom/
├── docker-compose.yml     # Docker 编排
├── Dockerfile.backend     # Backend 镜像
├── Dockerfile.client     # Client 镜像
├── .env.example          # 环境变量模板
├── Makefile
│
├── cmd/daemon/           # Go CLI (用户本地运行)
│   ├── api/              # HTTP API
│   ├── cmd/              # CLI 命令
│   ├── config/           # 配置管理
│   ├── runtime/          # Runtime 适配器
│   └── ws/               # WebSocket 客户端
│
├── backend/              # TanStack Start + Bun
│   └── src/
│
└── client/               # React 前端
    └── src/
```

---

## 本地开发

### 环境变量
```bash
cp .env.example .env.local
# 编辑 .env.local 配置
```

### Docker 管理
```bash
# 启动
docker-compose up -d

# 停止
docker-compose down

# 查看日志
docker-compose logs -f

# 查看状态
docker-compose ps
```

### 端口分配
| 端口 | 服务 |
|------|------|
| 5432 | PostgreSQL |
| 8000 | Centrifugo (WebSocket) |
| 3000 | Backend API |
| 8080 | Client UI |

---

## 部署

### Docker Compose（生产）
```bash
# 复制并配置环境变量
cp .env.example .env
# 编辑 .env 填入生产配置

# 启动所有服务
docker-compose -f docker-compose.yml up -d
```

### 使用预构建镜像
```bash
# 拉取镜像
docker pull ghcr.io/gal-criticism/loom/backend:latest
docker pull ghcr.io/gal-criticism/loom/client:latest
```

---

## CI/CD

### GitHub Actions

两个自动化 Workflow：

| Workflow | 触发 | 作用 |
|----------|------|------|
| **CI** | push / PR | 代码检查 + 构建验证 |
| **Release** | 手动触发 / tag | 构建并推送 Docker 镜像 |

### 手动触发 Release
1. 访问 https://github.com/Gal-criticism/loom/actions
2. 选择 "Release" → "Run workflow"
3. 输入版本号（如 0.1.0）→ "Run workflow"

或使用 GitHub CLI：
```bash
gh workflow run release.yml -f version=0.1.0
```

### 查看结果
访问 https://github.com/Gal-criticism/loom/actions 查看所有 workflow 运行状态和日志。

---

## 常见问题

### Docker 无法启动
```bash
docker info
brew services restart docker
```

### 端口被占用
```bash
lsof -i :5432
lsof -i :8000
lsof -i :3000
lsof -i :8080
```

### 数据库连接失败
```bash
docker-compose logs postgres
```
