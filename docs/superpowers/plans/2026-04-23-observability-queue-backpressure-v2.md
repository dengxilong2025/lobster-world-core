# 可观测性 v2（队列背压口径）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 debug/metrics v1 基础上新增队列背压口径：提供 intent accept 等待耗时统计 + per-world 队列深度快照（intentCh/pendingQueue），用于解释 BUSY 与吞吐/延迟。

**Architecture:** sim 增加只读 `Engine.QueueStats()` 暴露每个 world 的 `intentCh` 与 `pending queue` 深度；gateway 在 `/api/v0/intents` 计时 `sm.SubmitIntent` 得到 accept wait（ms），并在 `/api/v0/debug/metrics` 合并返回 `world_queue_stats`。所有新增仅用于 debug/metrics，不改变 event schema 与 replay 决定论。

**Tech Stack:** Go、integration tests（httptest）。

---

## 0) Files 结构与改动范围（先锁定）

**Create:**
- `tests/integration/queue_backpressure_metrics_v2_test.go`
- `internal/sim/queue_stats.go`（定义 QueueStat + Engine.QueueStats）

**Modify:**
- `internal/sim/world.go`（增加 world.queueStats 只读快照）
- `internal/gateway/metrics.go`（新增 intent_accept_wait_* 两个 counters）
- `internal/gateway/routes_intents.go`（计时 SubmitIntent，成功时写入 metrics）
- `internal/gateway/routes_debug.go`（合并 `world_queue_stats` 到 debug/metrics 输出）

---

## Task 1: 写 failing integration tests（RED）

**Files:**
- Create: `tests/integration/queue_backpressure_metrics_v2_test.go`

- [ ] **Step 1: 字段存在 + accept wait 计数增长**

```go
package integration

import (
  "bytes"
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "testing"
  "time"

  "lobster-world-core/internal/gateway"
)

func TestDebugMetricsV2Queue_AcceptWaitCountersIncrease(t *testing.T) {
  t.Parallel()
  app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 5 * time.Millisecond, Seed: 123})
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  before := getMetricsMap(t, s.URL)
  bCnt := metricInt64(t, before, "intent_accept_wait_count")

  worldID := "w_qv2_wait"
  for i := 0; i < 3; i++ {
    body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
    r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
    if err != nil { t.Fatalf("POST intents: %v", err) }
    _ = r.Body.Close()
    if r.StatusCode != http.StatusOK { t.Fatalf("status=%d", r.StatusCode) }
  }
  time.Sleep(50 * time.Millisecond)

  after := getMetricsMap(t, s.URL)
  aCnt := metricInt64(t, after, "intent_accept_wait_count")
  if aCnt < bCnt+3 {
    t.Fatalf("expected accept_wait_count to increase by >=3, before=%d after=%d", bCnt, aCnt)
  }
  if metricInt64(t, after, "intent_accept_wait_ms_total") <= 0 {
    t.Fatalf("expected accept_wait_ms_total > 0")
  }
}
```

- [ ] **Step 2: 制造背压后 world_queue_stats 中 pending_queue_len>0**

```go
func TestDebugMetricsV2Queue_WorldQueueStatsShowsPendingQueue(t *testing.T) {
  t.Parallel()
  // Large tick interval so the queue doesn't drain (world executes <=1 intent per tick).
  app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 5 * time.Second, Seed: 123, MaxIntentQueue: 1024})
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_qv2_depth"
  for i := 0; i < 10; i++ {
    body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
    r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
    if err != nil { t.Fatalf("POST intents: %v", err) }
    _ = r.Body.Close()
    if r.StatusCode != http.StatusOK { t.Fatalf("status=%d", r.StatusCode) }
  }
  time.Sleep(50 * time.Millisecond)

  mm := getMetricsMap(t, s.URL)
  wqs, ok := mm["world_queue_stats"].(map[string]any)
  if !ok || wqs == nil {
    t.Fatalf("expected world_queue_stats object, got %#v", mm["world_queue_stats"])
  }
  ws, ok := wqs[worldID].(map[string]any)
  if !ok || ws == nil {
    t.Fatalf("expected world_id entry, got %#v", wqs[worldID])
  }
  // json numbers decode to float64
  pq := int64(ws["pending_queue_len"].(float64))
  if pq <= 0 {
    t.Fatalf("expected pending_queue_len>0, got %d (ws=%#v)", pq, ws)
  }
}
```

- [ ] **Step 3: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run DebugMetricsV2Queue_ -v
```

- [ ] **Step 4: Commit（仅测试）**
```bash
git add tests/integration/queue_backpressure_metrics_v2_test.go
git commit -m "test(debug): add queue backpressure metrics v2 gates"
```

---

## Task 2: 最小实现 v2-queue（GREEN）

### 2.1 sim：Engine.QueueStats()

**Files:**
- Create: `internal/sim/queue_stats.go`
- Modify: `internal/sim/world.go`

- [ ] **Step 1: 定义 QueueStat + Engine.QueueStats**

```go
package sim

type QueueStat struct {
  IntentChLen    int   `json:"intent_ch_len"`
  IntentChCap    int   `json:"intent_ch_cap"`
  PendingQueueLen int  `json:"pending_queue_len"`
  PendingQueueMax int  `json:"pending_queue_max"`
  Tick           int64 `json:"tick"`
}

func (e *Engine) QueueStats() map[string]QueueStat {
  e.mu.Lock()
  defer e.mu.Unlock()
  out := map[string]QueueStat{}
  for id, w := range e.worlds {
    if w == nil { continue }
    out[id] = w.queueStats()
  }
  return out
}
```

- [ ] **Step 2: world.queueStats()**

在 `world.go`：
```go
func (w *world) queueStats() QueueStat {
  w.mu.Lock()
  defer w.mu.Unlock()
  return QueueStat{
    IntentChLen: len(w.intentCh),
    IntentChCap: cap(w.intentCh),
    PendingQueueLen: len(w.queue),
    PendingQueueMax: w.maxQueue,
    Tick: w.tick,
  }
}
```

### 2.2 gateway：intent_accept_wait_* + world_queue_stats

**Files:**
- Modify: `internal/gateway/metrics.go`
- Modify: `internal/gateway/routes_intents.go`
- Modify: `internal/gateway/routes_debug.go`

- [ ] **Step 3: metrics 增加 counters**

新增 atomic：
- `intentAcceptWaitMsTotal`
- `intentAcceptWaitCount`

并在 Snapshot 中追加：
- `intent_accept_wait_ms_total`
- `intent_accept_wait_count`

- [ ] **Step 4: intents handler 计时 SubmitIntent**

在 `routes_intents.go` 中：
- `start := time.Now()`
- 调用 `sm.SubmitIntent`
- 若成功（200）：`ms := max(1, sinceMs)`；`mt.AddIntentAcceptWaitMs(ms)` + `mt.IncIntentAcceptWaitCount()`

- [ ] **Step 5: debug/metrics 合并 world_queue_stats**

在 `routes_debug.go` 的 debug/metrics handler：
- `snap := mt.Snapshot()`
- 如果 `sm != nil`：`snap["world_queue_stats"] = sm.QueueStats()`

- [ ] **Step 6: 运行测试转绿**
```bash
go test ./tests/integration -run DebugMetricsV2Queue_ -v
go test ./...
```

- [ ] **Step 7: Commit（实现）**
```bash
git add internal/sim/queue_stats.go internal/sim/world.go internal/gateway/metrics.go internal/gateway/routes_intents.go internal/gateway/routes_debug.go
git commit -m "feat(debug): add queue backpressure metrics v2"
```

---

## Task 3: 回归与交付

- [ ] **Step 1: 背压冒烟记录**
用 `MaxIntentQueue=1` + `loadtest_intents.sh` 制造 BUSY，并记录 metrics 中：
`world_queue_stats` 与 `intent_accept_wait_*` 的变化，写入 `docs/ops/`。

- [ ] **Step 2: format-patch 覆盖包**
```bash
git format-patch -4 -o /workspace/patches_queue_backpressure_v2
cd /workspace && zip -qr patches_queue_backpressure_v2.zip patches_queue_backpressure_v2
```

