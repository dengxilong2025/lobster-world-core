# SSE metrics v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `GET /api/v0/debug/metrics` 补齐 SSE 可排障指标：`sse_bytes_total`、`sse_flush_errors_total`、连接时长统计（total/count/max）以及 `sse_connections_current_by_world`（低基数）。

**Architecture:** 在 gateway 的 SSE handler（`/api/v0/events/stream`）记录连接开始/结束时间并维护 per-world 在线连接数；在 `writeSSEMessage` 内统计每条消息写入的 bytes 并捕获 `bw.Flush()` 错误计数；所有新增指标都写入 `Metrics` 并由 `Snapshot()` 输出（debug-only，不影响 sim 决定论）。

**Tech Stack:** Go、httptest integration tests、atomic+mutex 低基数结构。

---

## 0) Files（锁定）

**Create:**
- `tests/integration/sse_metrics_v2_test.go`

**Modify:**
- `internal/gateway/metrics.go`
- `internal/gateway/routes_events.go`
- `internal/gateway/sse.go`

---

## Task 1: 写 failing integration test（RED）

**Files:**
- Create: `tests/integration/sse_metrics_v2_test.go`

- [ ] **Step 1: 新增测试：按 world 分桶连接数 + bytes 增长 + conn duration 计数增长**

```go
package integration

import (
  "bufio"
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "testing"
  "time"

  "lobster-world-core/internal/gateway"
)

func TestSSEMetricsV2_ByWorldBytesAndDuration(t *testing.T) {
  // No t.Parallel(): involves timing and streaming.
  app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 5 * time.Millisecond, Seed: 123})
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_sse_v2"

  // connect SSE
  client := &http.Client{Timeout: 2 * time.Second}
  req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v0/events/stream?world_id="+worldID, nil)
  resp, err := client.Do(req)
  if err != nil { t.Fatalf("connect: %v", err) }
  if resp.StatusCode != http.StatusOK { t.Fatalf("status=%d", resp.StatusCode) }

  br := bufio.NewReader(resp.Body)
  // read first line ":ok\n" so stream is established
  _, _ = br.ReadString('\n')

  before := getMetricsMap(t, s.URL)
  bBytes := metricInt64(t, before, "sse_bytes_total")
  bDurCnt := metricInt64(t, before, "sse_conn_duration_count")

  // by-world gauge present
  by, ok := before["sse_connections_current_by_world"].(map[string]any)
  if !ok || by == nil { t.Fatalf("expected sse_connections_current_by_world") }
  if int64(by[worldID].(float64)) < 1 { t.Fatalf("expected by_world>=1") }

  // trigger at least one event
  body := []byte(`{"world_id":"` + worldID + `","goal":"启动世界"}`)
  r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
  if err != nil { t.Fatalf("post intent: %v", err) }
  _ = r.Body.Close()

  // read one SSE message line "data: ..."
  deadline := time.Now().Add(500 * time.Millisecond)
  for time.Now().Before(deadline) {
    line, _ := br.ReadString('\n')
    if len(line) >= 6 && line[:6] == "data: " {
      break
    }
  }

  mid := getMetricsMap(t, s.URL)
  if metricInt64(t, mid, "sse_bytes_total") <= bBytes {
    t.Fatalf("expected sse_bytes_total to increase")
  }

  // close and ensure duration count increments
  _ = resp.Body.Close()
  time.Sleep(50 * time.Millisecond)

  after := getMetricsMap(t, s.URL)
  if metricInt64(t, after, "sse_conn_duration_count") <= bDurCnt {
    t.Fatalf("expected conn_duration_count to increase")
  }
}
```

> 注意：上面的 `bytes` 包需要在 import 里加入 `bytes`；如果 gofmt 报错，补齐即可。

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run SSEMetricsV2 -v
```

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/sse_metrics_v2_test.go
git commit -m "test(debug): add sse metrics v2 gate"
```

---

## Task 2: 最小实现（GREEN）

### 2.1 Metrics：新增字段与 Snapshot 输出

**Files:**
- Modify: `internal/gateway/metrics.go`

- [ ] **Step 1: 增加 counters/gauges**

新增原子字段：
- `sseBytesTotal`
- `sseFlushErrorsTotal`
- `sseConnDurationMsTotal`
- `sseConnDurationCount`
- `sseConnDurationMsMax`

新增 by-world map（mutex 保护，低频更新可用 int64 而非 atomic）：
- `sseByWorld map[string]int64`
- （可选）`sseByWorldLimit int` 默认 1024

新增方法：
- `AddSSEBytes(n int64)`
- `IncSSEFlushErrors()`
- `ObserveSSEConnDuration(ms int64)`（更新 total/count/max）
- `AddSSEConnectionsCurrentByWorld(worldID string, delta int64)`

Snapshot 输出新增：
- `sse_bytes_total`
- `sse_flush_errors_total`
- `sse_conn_duration_ms_total`
- `sse_conn_duration_count`
- `sse_conn_duration_ms_max`
- `sse_connections_current_by_world`

### 2.2 SSE 写入：统计 bytes + flush errors

**Files:**
- Modify: `internal/gateway/sse.go`
- Modify: `internal/gateway/routes_events.go`

- [ ] **Step 2: 修改 writeSSEMessage 返回 bytes**

签名改为：
```go
func writeSSEMessage(bw *bufio.Writer, flusher http.Flusher, data []byte) (int64, error)
```

统计逻辑：
- 累加每次写入成功的字节数
- `bw.Flush()` error 仍返回 error（由 caller 退出循环）

- [ ] **Step 3: routes_events.go 中对每次发送做 AddSSEBytes**

在 replay missed events 与 live events 两处：
- `n, err := writeSSEMessage(...)`
- 成功时：`mt.AddSSEBytes(n)` 并 `mt.IncSSEDataMessagesTotal()`
- error 时：若 `mt!=nil`，`mt.IncSSEFlushErrors()`（或在 writeSSEMessage 内部做；二选一，保持单点即可）

- [ ] **Step 4: 连接开始/结束：by world + duration**

在 handler 中：
- `start := time.Now()`
- `mt.AddSSEConnectionsCurrentByWorld(worldID, +1)`（进入）
- defer：`mt.AddSSEConnectionsCurrentByWorld(worldID, -1)`、`mt.ObserveSSEConnDuration(time.Since(start).Milliseconds())`

### 2.3 测试转绿 + 回归

- [ ] **Step 5: 运行集成测试转绿**
```bash
go test ./tests/integration -run SSEMetricsV2 -v
```

- [ ] **Step 6: 全量回归**
```bash
go test ./...
```

- [ ] **Step 7: Commit（实现）**
```bash
git add internal/gateway/metrics.go internal/gateway/sse.go internal/gateway/routes_events.go
git commit -m "feat(debug): sse metrics v2"
```

---

## Task 3: 冒烟与覆盖包

- [ ] **Step 1: 冒烟**
```bash
go run ./cmd/server
WORLD_ID=w_smoke CONNECTIONS=10 DURATION_SEC=3 ./scripts/loadtest_sse.sh
curl -s http://localhost:8080/api/v0/debug/metrics | head -c 1200
```

- [ ] **Step 2: 记录到 docs/ops/**
新增一份 `docs/ops/sse_metrics_v2_smoke_YYYY-MM-DD.md`（含命令与关键字段截图/粘贴）。

- [ ] **Step 3: format-patch 覆盖包**
```bash
git format-patch -4 -o /workspace/patches_sse_metrics_v2
cd /workspace && zip -qr patches_sse_metrics_v2.zip patches_sse_metrics_v2
```

