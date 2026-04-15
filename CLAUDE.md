# Loom Project Guide

## 项目概述

Loom 是一个具有"氛围感"的虚拟陪伴产品，通过本地 AI Runtime（Claude Code、OpenCode）提供 AI 对话能力。

## 技术栈

| 组件 | 技术 |
|------|------|
| Client | React + TanStack Start + TanStack Query |
| Backend | TanStack Start + Bun |
| Daemon | Go CLI |
| Database | PostgreSQL |
| WebSocket | Centrifugo |

## 项目结构

```
loom/
├── cmd/daemon/        # Go CLI (用户本地运行)
├── backend/          # TanStack Start + Bun API
├── client/           # React 前端
├── docker-compose.yml
├── Dockerfile.backend
├── Dockerfile.client
└── .github/workflows/
```

## 开发规范

### Git 工作流
- 功能开发使用分支
- 提交信息遵循 conventional commits
- 推送前确保 CI 通过

### 代码规范
- **Go**: 使用 `go vet` 检查
- **TypeScript/React**: 使用 Bun 构建验证

### 环境变量
- 开发使用 `.env.local`（已忽略）
- 生产使用 `.env`
- 模板见 `.env.example`

### Docker
- 生产部署使用 docker-compose
- 镜像推送到 GHCR

## 常用命令

```bash
# 本地开发
docker-compose up --build -d

# 构建 Daemon
cd cmd/daemon && go build -o loomd .

# 构建镜像
docker build -t ghcr.io/gal-criticism/loom/backend:latest ./backend
docker build -t ghcr.io/gal-criticism/loom/client:latest ./client
```

## 快速参考

| 服务 | 端口 |
|------|------|
| Client | 8080 |
| Backend | 3000 |
| PostgreSQL | 5432 |
| Centrifugo | 8000 |
