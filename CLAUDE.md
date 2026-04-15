# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Loom is an AI companion product with "vibe" - provides AI chat using local Runtime (Claude Code, OpenCode).

## Architecture

```
User Machine                    Cloud
┌─────────────┐                ┌─────────────────┐
│   Runtime   │◀── SDK ───────▶│    Daemon      │
│ Claude Code │                │  (Go CLI)       │
│  OpenCode   │                └────────┬────────┘
└─────────────┘                         │ WebSocket
                                        ▼
┌─────────────┐                ┌─────────────────┐
│    Client   │◀───────────────▶│    Backend     │
│  (React)    │   HTTP + WS    │  (Bun + TS)     │
└─────────────┘                └────────┬────────┘
                                        ▼
                                 ┌─────────────┐
                                 │ Centrifugo  │
                                 │  (WS)       │
                                 └─────────────┘
```

## Tech Stack

| Component | Technology |
|-----------|------------|
| Client | React + TanStack Start + TanStack Query |
| Backend | TanStack Start + Bun |
| Daemon | Go CLI |
| Database | PostgreSQL |
| WebSocket | Centrifugo |

## Common Commands

```bash
# Start all services locally
docker-compose up --build -d

# Build Daemon (requires Go)
cd cmd/daemon && go build -o loomd .

# Build and push Docker images
docker build -t ghcr.io/gal-criticism/loom/backend:latest ./backend
docker build -t ghcr.io/gal-criticism/loom/client:latest ./client

# Lint
cd cmd/daemon && go vet ./...
cd backend && bun run build
cd client && bun run build
```

## Key Files

- `docker-compose.yml` - All services orchestration
- `Dockerfile.backend` / `Dockerfile.client` - Container definitions
- `cmd/daemon/` - Go CLI that runs on user's machine
- `backend/` - TanStack Start API server
- `client/` - React frontend

## Environment

- Development: `.env.local` (gitignored)
- Production: `.env`
- Template: `.env.example`

## Ports

| Service | Port |
|---------|------|
| Client UI | 8080 |
| Backend API | 3000 |
| PostgreSQL | 5432 |
| Centrifugo WS | 8000 |
