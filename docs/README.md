# Loom 项目文档

## 文档索引

| 文档 | 说明 |
|------|------|
| [brainstorm.md](brainstorm.md) | 项目初始脑暴记录，产品概念和 MVP 定义 |
| [superpowers/comprehensive-architecture-evaluation.md](superpowers/comprehensive-architecture-evaluation.md) | 全面架构评估报告 |
| [superpowers/loom-upgrade-plan.md](superpowers/loom-upgrade-plan.md) | Loom 升级改造计划 |
| [superpowers/happy-architecture-reference.md](superpowers/happy-architecture-reference.md) | Happy CLI 架构参考 |
| [superpowers/specs/2026-04-15-loom-design.md](superpowers/specs/2026-04-15-loom-design.md) | 技术设计文档 |

---

## 快速链接

### 组件文档

- **Daemon**: 见 `cmd/daemon/README.md`
- **Backend**: 见 `backend/README.md`
- **Client**: 见 `client/README.md`

### 架构决策

1. **WebSocket 方案**: 使用 Centrifugo 作为 WebSocket 服务器
2. **Runtime 接口**: Go 接口抽象，支持 Claude Code 和 OpenCode
3. **通信协议**: Daemon ↔ Backend 通过 Centrifugo，Client ↔ Backend 通过 HTTP + Centrifugo

### 参考实现

- 参考 [Happy CLI](https://github.com/slopus/happy) 的架构设计
- 参考 [Claude Code](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview) 的 Runtime 设计
