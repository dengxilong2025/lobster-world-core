# 生产级可观测性 v1（debug/metrics 扩展）Design

**目标**：在不偏离现有 “事件溯源 + 投影 + 仿真” 架构、不引入外部依赖（Prometheus/k6 等）的前提下，把 `/api/v0/debug/metrics` 从“只有基础请求计数”扩展到可用于排障与容量评估的 **v1 指标集**（包含写入失败、export/highlight、SSE 在线连接等关键观测点）。

**范围**：
- 扩展 `internal/gateway/metrics.go` 指标集合
- 扩展 `GET /api/v0/debug/metrics` 返回 JSON（向后兼容：只新增字段）
- 在关键路径做最小埋点：SSE、replay/export、replay/highlight、EventStore.Append 失败
- 增加/更新 integration tests 作为回归门禁

**非目标**：
- 不做 Prometheus exposition / label 系统
- 不做高基数按 path/param 细分（只对少数固定端点做计数）
- 不引入 wall-clock 相关指标（tick 延迟）——当前 sim 是逻辑时间，先保持决定论

---

## 1) 现状

当前 `/api/v0/debug/metrics` 字段：
- `requests_total`
- `responses_by_status`（按 HTTP status 聚合）
- `busy_total`（`writeError` 在 `code=="BUSY"` 时计数）

缺口：
- 不知道 **EventStore.Append** 是否在失败（以及失败次数）
- 不知道 replay/export/highlight 的调用量与失败量（以及粗略耗时/响应大小）
- 不知道 SSE 当前在线连接数（对于观战稳定性很关键）

---

## 2) v1 指标集（新增字段）

### 2.1 EventStore 写入

新增：
- `eventstore_append_total`
- `eventstore_append_errors_total`

说明：
- 统计所有 `Append` 调用（sim 的 world.appendAndPublish、adoption routes 的 Append）
- 只计数，不记录事件类型（避免高基数）

### 2.2 replay/export

新增：
- `replay_export_total`
- `replay_export_errors_total`（非 200 或 Query 失败）
- `replay_export_time_ms_total`（累计耗时，便于粗略算均值）
- `replay_export_bytes_total`（累计输出 bytes，便于估算带宽/压力）

### 2.3 replay/highlight

新增：
- `replay_highlight_total`
- `replay_highlight_errors_total`
- `replay_highlight_time_ms_total`

### 2.4 SSE（events/stream）

新增：
- `sse_connections_current`（gauge）
- `sse_connections_total`（累计建立）
- `sse_disconnects_total`（累计断开/退出）
- `sse_data_messages_total`（写出 `data:` 消息的次数，粗略 proxy）

---

## 3) debug/metrics JSON 结构（向后兼容）

返回结构保持：
```json
{ "ok": true, "metrics": { ... } }
```

仅新增字段，不删除/不重命名旧字段。示例：
```json
{
  "ok": true,
  "metrics": {
    "requests_total": 123,
    "responses_by_status": {"200": 120, "400": 2, "503": 1},
    "busy_total": 1,

    "eventstore_append_total": 999,
    "eventstore_append_errors_total": 2,

    "replay_export_total": 10,
    "replay_export_errors_total": 0,
    "replay_export_time_ms_total": 420,
    "replay_export_bytes_total": 123456,

    "replay_highlight_total": 5,
    "replay_highlight_errors_total": 0,
    "replay_highlight_time_ms_total": 110,

    "sse_connections_current": 2,
    "sse_connections_total": 7,
    "sse_disconnects_total": 5,
    "sse_data_messages_total": 80
  }
}
```

---

## 4) 实现方案（最小侵入）

### 4.1 EventStore.Append 失败计数

采用 wrapper（装饰器）方式，避免侵入 sim/world：
- 新增 `internal/events/store/metrics_store.go`（或放在 gateway 内部 wrapper）：`type MetricsEventStore struct { inner store.EventStore; m *gateway.Metrics }`
- `Append`：先 `m.IncEventStoreAppend()`，若 inner.Append 返回 error 则 `m.IncEventStoreAppendError()`
- Query/GetByID 直接转发，不计数（v1 只关注写入健康）

在 `gateway.NewHandler` wiring 时：
- 若 opts.EventStore 非 nil：用 wrapper 包一层再传给 sim 与 spectator
- 若 nil：对 NewInMemoryEventStore() 的结果包一层

### 4.2 replay/export & highlight

在 `routes_replay.go` 的两个 handler 内：
- handler 开始记录 `start := time.Now()`
- 结束时 `m.AddReplayExportTimeMs(...)` / `m.AddReplayHighlightTimeMs(...)`
- export：用 `bytes.Count(body, '\n')` 不做；只用 `io.TeeReader` 或在写入前用 `bytes.Buffer`？（v1 采用 **response size 估算**：在 export handler 中累计 `len(encodedLineBytes)`）
- errors_total：在 `writeError` 调用前先 `Inc...Errors`，或在 handler 里按分支计数

### 4.3 SSE

在 `routes_events.go` 的 `/events/stream`：
- 建立连接后：`IncSSEConnectionsTotal()` + `IncSSEConnectionsCurrent(+1)`
- defer：`IncSSEConnectionsCurrent(-1)` + `IncSSEDisconnectsTotal()`
- 每次成功写出 `writeSSEMessage` 后：`IncSSEDataMessagesTotal()`

> 注意：指标埋点不能破坏 SSE：我们之前已确保 metrics wrapper 的 ResponseWriter 转发 `Flush()`。

---

## 5) 测试门禁（integration）

新增/扩展 integration tests：
1) **metrics 基础字段存在**（已有 `TestDebugMetrics_ExposesCounters`，扩展断言新字段 key 存在）
2) **SSE gauge 变化**：
   - 建立一个 `/events/stream` 连接
   - 立刻 GET `/debug/metrics`，断言 `sse_connections_current >= 1`
   - 关闭连接，再 GET metrics，断言回到 0
3) **export 计数与 bytes/time 增长**：
   - 先创建 world 并制造事件（post intents）
   - 调用 `/replay/export`
   - 再读 `/debug/metrics`，断言 `replay_export_total` 增长且 `replay_export_bytes_total > 0`
4) **EventStore append 计数增长**：
   - post intents 后读 metrics，断言 `eventstore_append_total` 增长（只断言 >0，避免绑定精确值）

---

## 6) 风险与控制

- 指标仅用于 debug：不承诺稳定的“监控协议”，但字段名保持向后兼容（只加不删）
- 不做 per-world/per-route 高基数拆分，避免未来爆炸
- 仍保持决定论：不引入 wall-clock 影响 sim；仅在 HTTP handler 内用于统计耗时

