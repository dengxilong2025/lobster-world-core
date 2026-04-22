# 本地压测指南（零依赖：bash + curl）

本目录提供一组 **不引入任何外部压测工具** 的脚本，用于快速得到趋势性基准数据（吞吐/QPS、错误率、平均耗时），并可与 `GET /api/v0/debug/metrics` 交叉验证。

> 注意：这些脚本 **不进入 CI**，只用于人工/本地/临时环境压测。

---

## 0) 前置条件

- bash（macOS/Linux）
- curl
- awk
- `timeout`（Linux 通常自带；macOS 可用 `gtimeout` 或跳过 SSE 脚本）

---

## 1) 启动服务

在仓库根目录：

```bash
go run ./cmd/server
```

默认地址：`http://localhost:8080`

如果你启动在其他地址/端口，请通过 `BASE_URL` 传给脚本。

---

## 2) 通用环境变量

所有脚本支持：
- `BASE_URL`：服务地址（默认 `http://localhost:8080`）

示例：
```bash
BASE_URL=http://localhost:8080 ./scripts/loadtest_intents.sh
```

---

## 3) Auth 压测：challenge

脚本：`scripts/loadtest_auth_challenge.sh`

参数：
- `CONCURRENCY`（默认 10）
- `REQUESTS`（默认 200）

示例：
```bash
CONCURRENCY=20 REQUESTS=500 ./scripts/loadtest_auth_challenge.sh
```

说明：
- 若出现 `429`，通常是预期行为（auth 限流在工作）。

---

## 4) Intents 压测

脚本：`scripts/loadtest_intents.sh`

参数：
- `WORLD_ID`（默认 `w_load`）
- `CONCURRENCY`（默认 10）
- `REQUESTS`（默认 200）
- `GOAL`（默认 `启动世界`）

示例：
```bash
WORLD_ID=w_load CONCURRENCY=50 REQUESTS=1000 GOAL="探索：推进" ./scripts/loadtest_intents.sh
```

说明：
- `503` 表示 `BUSY`（背压）。建议：
  1) 降低 `CONCURRENCY`
  2) 观察 `GET /api/v0/debug/metrics` 中的 `busy_total`

---

## 5) SSE 压测（连接稳定性/事件吞吐 proxy）

脚本：`scripts/loadtest_sse.sh`

参数：
- `WORLD_ID`（默认 `w_load`）
- `CONNECTIONS`（默认 10）
- `DURATION_SEC`（默认 10）

示例：
```bash
WORLD_ID=w_load CONNECTIONS=20 DURATION_SEC=15 ./scripts/loadtest_sse.sh
```

输出解释：
- 每条连接会统计 `data:` 行数（可作为事件吞吐的粗略 proxy）
- exit code `124` 表示 `timeout` 到期（预期），非 0/124 可能是提前断开

---

## 6) replay/export 压测

脚本：`scripts/loadtest_replay_export.sh`

参数：
- `WORLD_ID`（默认 `w_load`）
- `LIMIT`（默认 5000）
- `CONCURRENCY`（默认 5）
- `REQUESTS`（默认 50）

示例：
```bash
WORLD_ID=w_load LIMIT=5000 CONCURRENCY=10 REQUESTS=200 ./scripts/loadtest_replay_export.sh
```

---

## 7) 与 debug/metrics 交叉验证

压测前后可查看：

```bash
curl -s http://localhost:8080/api/v0/debug/metrics | jq
```

重点字段：
- `requests_total`
- `responses_by_status`
- `busy_total`

它们应与脚本输出的状态码分布大致一致（允许少量差异：例如压测过程中你做了其他请求）。

