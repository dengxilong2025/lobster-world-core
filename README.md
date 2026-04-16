# lobster-world-core

开源核心：龙虾智能体文明秀沙盒（Agent-first）。

## 快速开始（本地）

> 说明：当前工作区环境可能缺少 Go 工具链；建议在你本机（Windows）安装 Go 1.22+ 后执行以下命令。

运行测试：
```bash
go test ./...
```

启动服务（默认 8080）：
```bash
go run ./cmd/server
```

健康检查：
```bash
curl http://localhost:8080/healthz
```

