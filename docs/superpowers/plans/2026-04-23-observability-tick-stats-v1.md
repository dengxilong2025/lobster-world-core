# tick 观测 v1（wall-clock）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `GET /api/v0/debug/metrics` 增加 `world_tick_stats`，观测每个 world 的 tick 是否推进、最后 tick 时间、tick 间隔漂移与明显卡顿次数（wall-clock 口径，保证不影响 sim 决定论）。

**Architecture:** 在 `sim.world.loop()` 的 tick 分支（`<-t.C`）维护一个仅用于观测的 `TickStat`（lastTickAt、计数器、jitter/overrun）。通过 `sim.Engine.TickStats()` 暴露只读快照，并由 gateway 的 debug/metrics 合并进输出。benchmarks 脚本增加摘要段，便于快速阅读回归结果。

**Tech Stack:** Go、httptest integration tests、bash+curl（benchmarks）。

---

## 0) Files 结构与改动范围（先锁定）

**Create:**
- `internal/sim/tick_stats.go`
- `tests/integration/tick_stats_metrics_v1_test.go`

**Modify:**
- `internal/sim/world.go`
- `internal/sim/engine.go`
- `internal/gateway/routes_debug.go`
- `scripts/benchmarks_run.sh`

---

## Task 1: 写 failing integration tests（RED）

**Files:**
- Create: `tests/integration/tick_stats_metrics_v1_test.go`

- [ ] **Step 1: 新增测试：字段存在 + tick 会增长**

```go
package integration

import (
  "net/http/httptest"
  "testing"
  "time"

  "lobster-world-core/internal/gateway"
)

func TestDebugMetrics_TickStatsV1_TicksIncrease(t *testing.T) {
  t.Parallel()
  app := gateway.NewAppWithOptions(gateway.AppOptions{
    TickInterval: 5 * time.Millisecond,
    Seed: 123,
  })
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_tick_stats"
  // Ensure world exists so it starts ticking.
  app.Sim.EnsureWorld(worldID)
  time.Sleep(50 * time.Millisecond)

  mm := getMetricsMap(t, s.URL)
  wts, ok := mm["world_tick_stats"].(map[string]any)
  if !ok || wts == nil {
    t.Fatalf("expected world_tick_stats object, got %#v", mm["world_tick_stats"])
  }
  ws, ok := wts[worldID].(map[string]any)
  if !ok || ws == nil {
    t.Fatalf("expected world entry for %q, got %#v", worldID, wts[worldID])
  }
  // json numbers decode to float64
  tickCount := int64(ws["tick_count_total"].(float64))
  lastMs := int64(ws["tick_last_unix_ms"].(float64))
  if tickCount <= 0 {
    t.Fatalf("expected tick_count_total>0, got %d", tickCount)
  }
  if lastMs <= 0 {
    t.Fatalf("expected tick_last_unix_ms>0, got %d", lastMs)
  }
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run TickStatsV1 -v
```

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/tick_stats_metrics_v1_test.go
git commit -m "test(debug): add tick stats v1 gate"
```

---

## Task 2: 最小实现 v1（GREEN）

### 2.1 sim：TickStat + Engine.TickStats()

**Files:**
- Create: `internal/sim/tick_stats.go`
- Modify: `internal/sim/world.go`
- Modify: `internal/sim/engine.go`

- [ ] **Step 1: 定义 TickStat**

`internal/sim/tick_stats.go`：
```go
package sim

type TickStat struct {
  TickCountTotal   int64 `json:"tick_count_total"`
  TickLastUnixMs   int64 `json:"tick_last_unix_ms"`
  TickJitterMsTotal int64 `json:"tick_jitter_ms_total"`
  TickJitterCount  int64 `json:"tick_jitter_count"`
  TickOverrunTotal int64 `json:"tick_overrun_total"`
}
```

- [ ] **Step 2: world 增加字段并在 tick 分支更新**

在 `internal/sim/world.go` 的 `world` 结构体增加（在 `mu` 保护下）：
- `tickStat TickStat`
- `lastTickAt time.Time`（wall-clock，观测用）

在 `loop()` 的 `case <-t.C:` 分支里：
```go
now := time.Now()
w.mu.Lock()
w.tickStat.TickCountTotal++
w.tickStat.TickLastUnixMs = now.UnixMilli()
if !w.lastTickAt.IsZero() {
  actual := now.Sub(w.lastTickAt).Milliseconds()
  expected := w.tickInterval.Milliseconds()
  if expected > 0 {
    d := actual - expected
    if d < 0 { d = -d }
    w.tickStat.TickJitterMsTotal += d
    w.tickStat.TickJitterCount++
    if actual >= 2*expected {
      w.tickStat.TickOverrunTotal++
    }
  }
}
w.lastTickAt = now
w.mu.Unlock()

w.step()
```

> 注意：更新 tickStat 只用于观测，不影响 `w.tick` / event timeline。

- [ ] **Step 3: world.tickStats() 只读快照**

```go
func (w *world) tickStats() TickStat {
  w.mu.Lock()
  defer w.mu.Unlock()
  return w.tickStat
}
```

- [ ] **Step 4: Engine.TickStats() 暴露 map**

在 `internal/sim/tick_stats.go` 追加：
```go
func (e *Engine) TickStats() map[string]TickStat {
  e.mu.Lock()
  defer e.mu.Unlock()
  out := map[string]TickStat{}
  for id, w := range e.worlds {
    if w == nil { continue }
    out[id] = w.tickStats()
  }
  return out
}
```

### 2.2 gateway：debug/metrics 合并字段

**Files:**
- Modify: `internal/gateway/routes_debug.go`

- [ ] **Step 5: 在 /debug/metrics 合并 world_tick_stats**

```go
if sm != nil {
  snap["world_tick_stats"] = sm.TickStats()
}
```

- [ ] **Step 6: 运行测试转绿 + 全量回归**
```bash
go test ./tests/integration -run TickStatsV1 -v
go test ./...
```

- [ ] **Step 7: Commit（实现）**
```bash
git add internal/sim/tick_stats.go internal/sim/world.go internal/sim/engine.go internal/gateway/routes_debug.go
git commit -m "feat(debug): add tick stats v1"
```

---

## Task 3: benchmarks 摘要增强

**Files:**
- Modify: `scripts/benchmarks_run.sh`

- [ ] **Step 1: 增加 world_tick_stats 摘要段**

在脚本末尾追加：
```bash
echo "## world_tick_stats（摘要）"
echo
echo '```'
curl -s "${BASE_URL}/api/v0/debug/metrics" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("metrics", {}).get("world_tick_stats", {}))'
echo '```'
echo
```

- [ ] **Step 2: Commit**
```bash
git add scripts/benchmarks_run.sh
git commit -m "chore(ops): add tick stats summary to benchmarks"
```

---

## Task 4: 交付覆盖包

- [ ] **Step 1: format-patch + zip**
```bash
git format-patch -4 -o /workspace/patches_tick_stats_v1
cd /workspace && zip -qr patches_tick_stats_v1.zip patches_tick_stats_v1
```

