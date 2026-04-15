#!/bin/bash

# Loom 本地开发启动脚本

set -e

echo "🚀 启动 Loom 本地开发环境..."

# 1. 启动所有 Docker 服务（包括 Backend + Client）
echo "📦 启动 Docker 服务..."
docker-compose up --build -d

echo ""
echo "✅ Loom 已启动!"
echo ""
echo "访问地址："
echo "  - Client:  http://localhost:8080"
echo "  - Backend: http://localhost:3000"
echo "  - WebSocket: ws://localhost:8000"
echo ""
echo "按 Ctrl+C 停止所有服务"

docker-compose logs -f
