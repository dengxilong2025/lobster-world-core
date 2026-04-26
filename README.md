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

## 运行配置（环境变量）

### 端口
- `PORT`：服务监听端口（默认 `8080`）

### 反代/Ingress 场景的限流 IP 识别（X-Forwarded-For）
默认情况下，限流使用 `RemoteAddr` 作为 client IP，仅在 **loopback** 受控反代（`127.0.0.1` / `::1`）场景下才信任 `X-Forwarded-For`。

如需在真实反向代理 / Ingress 后信任 XFF，请配置：
- `TRUSTED_PROXY_CIDRS`：逗号分隔 CIDR 列表。仅当 `RemoteAddr` 命中这些网段时，才解析 `X-Forwarded-For` 的第一个 IP 作为 client IP。
  - 示例：`TRUSTED_PROXY_CIDRS="10.0.0.0/8,192.168.0.0/16"`

### Shock 调度（冲击事件）
用于在固定周期触发“预警/开始/结束”的冲击事件，并在开始时可额外生成一次“背叛”戏剧事件（从候选的 `ActorsPool` 中抽取）。

- `LW_SHOCK_ENABLED=1`：开启 shock scheduler（默认关闭）
- `LW_SHOCK_EPOCH_TICKS`（默认 `720`）
- `LW_SHOCK_WARNING_OFFSET`（默认 `1`）
- `LW_SHOCK_DURATION_TICKS`（默认 `3`）
- `LW_SHOCK_COOLDOWN_TICKS`（默认 `720`）

内置候选 key（当前版本）：
- `riftwinter`（裂冬）
- `greatdrought`（大旱）
- `plaguewave`（疫病）
- `outsiderraid`（外族入侵）

健康检查：
```bash
curl http://localhost:8080/healthz
```

## Staging（Render）快速验收

```bash
# 默认验收 https://lobster-world-core.onrender.com
bash scripts/smoke_staging.sh
```
