# 队列背压指标 v2 冒烟记录（2026-04-23）

目标：验证 `debug/metrics v2-queue` 能观测到：
- `intent_accept_wait_*` 随 intents 提交增长
- `world_queue_stats` 能看到 per-world 的 intentCh / pendingQueue 深度

环境：
- server：`go run ./cmd/server`（默认 `:8080`）
- BASE_URL：`http://localhost:8080`

---

## 1) 高并发提交 intents（制造 pending queue 累积）

命令：
```bash
WORLD_ID=w_bp CONCURRENCY=50 REQUESTS=500 GOAL='启动世界' ./scripts/loadtest_intents.sh
```

结果（摘要）：
- TOTAL=500
- QPS≈113
- 503 BUSY = 0（accept 没被拒）

---

## 2) 对照 debug/metrics

命令：
```bash
curl -s http://localhost:8080/api/v0/debug/metrics
```

关键字段（示例）：
- `intent_accept_wait_count` = 500
- `intent_accept_wait_ms_total` = 500（每次至少 1ms，避免 0ms）
- `world_queue_stats.w_bp.pending_queue_len` = 500
- `world_queue_stats.w_bp.intent_ch_len` = 0（入口 channel 未堆积，背压主要体现在 pending queue）

解释：
- 当前 TickInterval=5s，world 每 tick 只执行 1 个 intent，因此在短时间 burst 下 `pending_queue_len` 会累积；
- 这可以直接解释“为什么后续会出现 BUSY / 为什么执行进度慢”。

