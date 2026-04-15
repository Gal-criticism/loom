#!/bin/bash

# Loom 本地开发启动脚本

set -e

echo "🚀 启动 Loom 本地开发环境..."

# 1. 启动 Docker 服务
echo "📦 启动 Docker 服务..."
docker-compose up -d postgres centrigugo

# 等待数据库就绪
echo "⏳ 等待数据库就绪..."
sleep 3

# 2. 安装并启动 Backend
echo "🔧 启动 Backend..."
cd backend
bun install --silent 2>/dev/null || true
bun run dev &
BACKEND_PID=$!

# 3. 安装并启动 Client
echo "🎨 启动 Client..."
cd ../client
bun install --silent 2>/dev/null || true
bun run dev &
CLIENT_PID=$!

cd ..

echo ""
echo "✅ Loom 开发环境已启动!"
echo ""
echo "访问地址："
echo "  - Client:  http://localhost:8080"
echo "  - Backend: http://localhost:3000"
echo "  - WebSocket: ws://localhost:8000"
echo ""
echo "按 Ctrl+C 停止所有服务"

# 等待中断信号
trap "kill $BACKEND_PID $CLIENT_PID; docker-compose down" EXIT
wait
