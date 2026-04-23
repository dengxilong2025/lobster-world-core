# 可观测性：SSE metrics v2 Design

**Goal**：补齐 SSE（`GET /api/v0/events/stream`）的“可排障”指标，在 debug/metrics 中回答：

1) SSE 是否在持续输出？（bytes / message）
2) 连接是否频繁断开？（disconnects / duration）
3) 哪些 world 的订阅量高？（按 world 的在线连接数，低基数）
4) 是否存在底层 flush/write 失败？（flush_errors）

**Non-goals**
- 不引入 Prometheus/OTEL
- 不做高基数（例如按 user/ip/ua）标签
- 不改变 SSE payload 结构与 replay 语义

---

## 1) 现状（v1）

当前 metrics 已有：
- `sse_connections_current`
- `sse_connections_total`
- `sse_disconnects_total`
- `sse_data_messages_total`

SSE 发送路径：
- `internal/gateway/routes_events.go`：`/api/v0/events/stream`
- `internal/gateway/sse.go`：`writeSSEMessage(bw, flusher, data)` 负责写入与 `bw.Flush()` + `flusher.Flush()`

---

## 2) 新增指标（v2）

### 2.1 bytes / flush errors

新增：
- `sse_bytes_total`：累计写入到响应流的字节数（包含 SSE framing + data JSON；不含 TCP/IP 头）。
- `sse_flush_errors_total`：`bw.Flush()` 返回 error 的次数（通常代表客户端断开或底层写失败）。

口径说明：
- bytes 统计在 `writeSSEMessage` 内部按“成功写入的 byte 数”累计：
  - `"event: message\n"`、`"data: "`、`data`、`"\n\n"` 的字节数之和
  - 另：初始 `:ok\n\n` 也计入 bytes_total（建立流时写入）

### 2.2 connection duration

新增：
- `sse_conn_duration_ms_total`
- `sse_conn_duration_count`
- `sse_conn_duration_ms_max`

口径：
- 连接开始：handler 开始处理并完成 headers + subscribe 后（写 `:ok\n\n` 之前或之后均可，建议之前）
- 连接结束：handler return（包括 `ctx.Done()`、写入失败、server 关闭）
- duration = `time.Since(start).Milliseconds()`，最小记 0（不强制 +1）

### 2.3 connections by world（低基数）

新增：
- `sse_connections_current_by_world`：`map[string]int64`

更新规则：
- 每个连接进入 `/events/stream` 时：`by_world[worldID] += 1`
- 连接退出时：`by_world[worldID] -= 1`；当归零时可选择删除 key（避免 map 膨胀）

限制：
- 若 world 数超过上限（建议 1024），新 world 的分桶不再创建（只统计全局 `sse_connections_current`）。

---

## 3) 代码改动建议（最小侵入）

1) `internal/gateway/metrics.go`
   - 增加原子计数：
     - `sseBytesTotal`
     - `sseFlushErrorsTotal`
     - `sseConnDurationMsTotal / sseConnDurationCount / sseConnDurationMsMax`
   - 增加 `byWorld` map（mutex + *atomic.Int64）或 mutex+int64（连接数变更频率低，mutex 足够）
   - Snapshot 输出新增字段

2) `internal/gateway/sse.go`
   - 将 `writeSSEMessage` 改为返回 `(n int64, err error)`，n 为本次成功写入字节数
   - `bw.Flush()` error 时：上报 `sse_flush_errors_total++`

3) `internal/gateway/routes_events.go`
   - 在连接建立/断开 defer 中：
     - 更新 `sse_connections_current_by_world`
     - 记录 conn duration（total/count/max）
   - 在发送每条 message 后：
     - `sse_bytes_total += n`
     - `sse_data_messages_total++`（保持现有）

---

## 4) 测试门禁（integration）

新增集成测试：
1) 建立 SSE 连接后，`sse_connections_current_by_world[worldID] >= 1`
2) 发送一条 intent 触发事件后：
   - `sse_bytes_total` 增长
   - `sse_data_messages_total` 增长（可复用已有逻辑）
3) 关闭连接后：
   - `sse_conn_duration_count` 增长（>=1）
   - `sse_connections_current_by_world[worldID]` 回到 0（或 key 不存在）

---

## 5) 回归与冒烟

使用现有脚本：
```bash
WORLD_ID=w_smoke CONNECTIONS=10 DURATION_SEC=3 ./scripts/loadtest_sse.sh
curl -s http://localhost:8080/api/v0/debug/metrics | jq '.metrics | {sse_bytes_total, sse_flush_errors_total, sse_connections_current_by_world}'
```

