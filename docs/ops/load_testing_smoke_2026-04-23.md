# 压测脚本冒烟记录（2026-04-23）

目的：验证 `scripts/loadtest_*.sh` 在本地可跑通，并与 `/api/v0/debug/metrics` 的 v1 指标一致。

环境：
- server：`go run ./cmd/server`（默认 `:8080`）
- BASE_URL：`http://localhost:8080`

---

## 1) intents（轻量）

命令：
```bash
WORLD_ID=w_smoke CONCURRENCY=5 REQUESTS=20 GOAL='启动世界' ./scripts/loadtest_intents.sh
```

结果（摘要）：
- TOTAL=20
- STATUS_COUNTS：200×20
- QPS≈111.73

---

## 2) SSE（连接稳定性）

命令：
```bash
WORLD_ID=w_smoke CONNECTIONS=2 DURATION_SEC=1 ./scripts/loadtest_sse.sh
```

结果（摘要）：
- 2 条连接均以 exit code 124（timeout 到期）正常退出
- 期间 `data:` 行为 0（此用例未主动触发事件写出）

---

## 3) replay/export（轻量）

命令：
```bash
WORLD_ID=w_smoke CONCURRENCY=2 REQUESTS=5 ./scripts/loadtest_replay_export.sh
```

结果（摘要）：
- TOTAL=5
- STATUS_COUNTS：200×5
- AVG_BYTES≈5884

---

## 4) debug/metrics（对照）

压测后拉取：
```bash
curl -s http://localhost:8080/api/v0/debug/metrics
```

关键字段示例：
- `eventstore_append_total`≈20（对应 intents 请求写入）
- `replay_export_total`≈5；`replay_export_bytes_total`>0；`replay_export_time_ms_total`>0
- `sse_connections_total`≈2；`sse_disconnects_total`≈2；`sse_connections_current`=0

