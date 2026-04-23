# SSE metrics v2 冒烟记录（2026-04-23）

目标：验证新增 SSE 指标可观测：
- `sse_bytes_total`
- `sse_flush_errors_total`
- `sse_conn_duration_ms_total/count/max`
- `sse_connections_current_by_world`

---

## 1) 启动服务

```bash
go run ./cmd/server
```

---

## 2) 运行 SSE 压测脚本（建立连接）

```bash
WORLD_ID=w_smoke CONNECTIONS=10 DURATION_SEC=3 ./scripts/loadtest_sse.sh
```

---

## 3) 查看 debug/metrics

```bash
curl -s http://localhost:8080/api/v0/debug/metrics
```

关注字段（期望）：
- `sse_bytes_total` > 0（有数据输出）
- `sse_flush_errors_total` >= 0（理想为 0）
- `sse_conn_duration_count` >= CONNECTIONS（连接关闭后会累计）
- `sse_connections_current_by_world.w_smoke` 在压测期间 > 0，结束后回到 0（或 key 消失）

