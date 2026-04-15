#!/bin/bash

# Loom 本地开发启动脚本（使用外部 Centrifugo）
# 需要先在 .env 中配置外部 Centrifugo 地址

set -e

echo "🚀 启动 Loom 本地开发环境（外部 Centrifugo）..."

# 检查环境变量
if [ ! -f .env ]; then
    echo "❌ 错误：未找到 .env 文件"
    echo "请复制 .env.example 并配置外部 Centrifugo："
    echo "  cp .env.example .env"
    echo "  # 编辑 .env 填入你的 Centrifugo 配置"
    exit 1
fi

# 检查 Centrifugo 配置
source .env
if [ -z "$CENTRIFUGO_URL" ] || [ "$CENTRIFUGO_URL" = "ws://your-centrifugo-server:8000" ]; then
    echo "❌ 错误：未配置外部 Centrifugo"
    echo "请在 .env 文件中设置："
    echo "  CENTRIFUGO_URL=ws://your-centrifugo-server:8000"
    echo "  CENTRIFUGO_API_KEY=your_api_key"
    echo "  CENTRIFUGO_TOKEN_SECRET=your_token_secret"
    exit 1
fi

echo "📦 启动服务..."
docker-compose -f docker-compose.local.yml up --build -d

echo ""
echo "✅ Loom 已启动!"
echo ""
echo "服务地址："
echo "  - Client:    http://localhost:${CLIENT_PORT:-8080}"
echo "  - Backend:   http://localhost:${APP_PORT:-3000}"
echo "  - Centrifugo: ${CENTRIFUGO_URL} (外部)"
echo ""
echo "查看日志: docker-compose -f docker-compose.local.yml logs -f"
echo "停止服务: docker-compose -f docker-compose.local.yml down"
