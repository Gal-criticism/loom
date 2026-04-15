# Loom Daemon

Loom Daemon 是运行在用户本地的 AI Runtime 管理器。

## 构建

```bash
go build -o loomd .
```

## 运行

```bash
./loomd start
```

## 配置

创建 `config.yaml` 文件：

```yaml
runtime: claude-code
backend_ws: ws://localhost:3000/ws/daemon
listen: localhost:3456
log_level: info
```
