# A3：BUSY 可解释性 + smoke 失败自动抓取 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 提升 staging 排障效率：`/api/v0/debug/metrics` 增加 `metrics.summary`（busy/queue/tick 人话摘要）；`scripts/smoke_staging.sh` 任一步失败时自动抓取并打印 `/api/v0/debug/build` 与 `/api/v0/debug/metrics`（截断输出）。

**Architecture:** summary 在 gateway 层根据现有结构化字段（busy_by_reason + sim.QueueStats/TickStats）即时计算并写入响应；smoke 脚本将错误处理统一到 `die()`，在退出前调用 `dump_debug()` 拉取 build+metrics（失败也不影响原始退出码/错误提示）。

**Tech Stack:** Go（net/http + time + fmt）、bash/curl。

---

## 0) Files（锁定）

**Modify:**
- `internal/gateway/routes_debug.go`（在 debug/metrics 响应内填充 summary）
- `scripts/smoke_staging.sh`（失败时 dump debug/build + debug/metrics）

**Create:**
- `internal/gateway/metrics_summary.go`（summary 生成逻辑，便于测试与隔离）
- `tests/integration/debug_metrics_summary_test.go`（门禁：summary 字段存在且形状正确）

---

## Task 1: TDD — 新增 integration test 门禁（RED）

**Files:**
- Create: `tests/integration/debug_metrics_summary_test.go`

- [ ] **Step 1: 写 failing test**

```go
package integration

import (
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "testing"
  "time"

  "lobster-world-core/internal/gateway"
)

func TestDebugMetrics_IncludesSummary(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  resp, err := http.Get(s.URL + "/api/v0/debug/metrics")
  if err != nil { t.Fatalf("GET metrics: %v", err) }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK { t.Fatalf("expected 200, got %d", resp.StatusCode) }

  var out struct {
    OK      bool                   `json:"ok"`
    Metrics map[string]interface{} `json:"metrics"`
  }
  if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
    t.Fatalf("decode: %v", err)
  }
  if !out.OK { t.Fatalf("ok should be true") }

  sum, ok := out.Metrics["summary"].(map[string]interface{})
  if !ok || sum == nil {
    t.Fatalf("missing metrics.summary")
  }
  // Required keys
  for _, k := range []string{"busy", "queue", "tick"} {
    if _, ok := sum[k]; !ok {
      t.Fatalf("missing summary.%s", k)
    }
  }
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run TestDebugMetrics_IncludesSummary -v
```
Expected: FAIL（当前没有 summary 字段）

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/debug_metrics_summary_test.go
git commit -m "test(debug): gate metrics summary"
```

---

## Task 2: 实现 summary 生成（GREEN）

**Files:**
- Create: `internal/gateway/metrics_summary.go`
- Modify: `internal/gateway/routes_debug.go`

- [ ] **Step 1: 新增 summary 生成函数**

Create: `internal/gateway/metrics_summary.go`

```go
package gateway

import (
  "fmt"
  "math"
  "sort"

  "lobster-world-core/internal/sim"
)

func buildMetricsSummary(m *Metrics, qs map[string]sim.QueueStat, ts map[string]sim.TickStat) map[string]any {
  out := map[string]any{
    "busy":  "unknown",
    "queue": "unknown",
    "tick":  "unknown",
  }
  if m != nil {
    snap := m.Snapshot()
    // Busy summary
    if bt, ok := snap["busy_total"].(int64); ok && bt == 0 {
      out["busy"] = "ok"
    } else {
      // busy_by_reason is map[string]any with int64 values
      by, _ := snap["busy_by_reason"].(map[string]any)
      out["busy"] = fmt.Sprintf("busy_total=%v (intent_ch_full=%v, pending_queue_full=%v, accept_timeout=%v)",
        snap["busy_total"],
        pick(by, "intent_ch_full"),
        pick(by, "pending_queue_full"),
        pick(by, "accept_timeout"),
      )
    }
  }

  // Queue summary
  if len(qs) == 0 {
    out["queue"] = "no_worlds"
  } else {
    hottest := ""
    hottestScore := -1.0
    hottestLine := ""
    maxPending := 0
    maxIntentCh := 0
    keys := make([]string, 0, len(qs))
    for k := range qs { keys = append(keys, k) }
    sort.Strings(keys)
    for _, wid := range keys {
      q := qs[wid]
      if q.PendingQueueMax > maxPending { maxPending = q.PendingQueueMax }
      if q.IntentChCap > maxIntentCh { maxIntentCh = q.IntentChCap }
      pr := ratio(q.PendingQueueLen, q.PendingQueueMax)
      ir := ratio(q.IntentChLen, q.IntentChCap)
      score := math.Max(pr, ir)
      if score > hottestScore {
        hottestScore = score
        hottest = wid
        hottestLine = fmt.Sprintf("pending=%d/%d intent_ch=%d/%d tick=%d", q.PendingQueueLen, q.PendingQueueMax, q.IntentChLen, q.IntentChCap, q.Tick)
      }
    }
    out["queue"] = fmt.Sprintf("worlds=%d max_pending=%d max_intent_ch=%d hottest=%s %s",
      len(qs), maxPending, maxIntentCh, hottest, hottestLine)
  }

  // Tick summary
  if len(ts) == 0 {
    out["tick"] = "no_worlds"
  } else {
    hottest := ""
    hottestOverrun := int64(-1)
    overrunTotal := int64(0)
    jitterMsTotal := int64(0)
    jitterCount := int64(0)
    for wid, t := range ts {
      overrunTotal += t.TickOverrunTotal
      jitterMsTotal += t.TickJitterMsTotal
      jitterCount += t.TickJitterCount
      if t.TickOverrunTotal > hottestOverrun {
        hottestOverrun = t.TickOverrunTotal
        hottest = wid
      }
    }
    avg := 0.0
    if jitterCount > 0 {
      avg = float64(jitterMsTotal) / float64(jitterCount)
    }
    out["tick"] = fmt.Sprintf("worlds=%d overrun_total=%d jitter_avg_ms≈%.1f hottest=%s overrun=%d",
      len(ts), overrunTotal, avg, hottest, hottestOverrun)
  }

  return out
}

func ratio(a, b int) float64 {
  if b <= 0 { return 0 }
  return float64(a) / float64(b)
}

func pick(m map[string]any, k string) any {
  if m == nil { return 0 }
  if v, ok := m[k]; ok { return v }
  return 0
}
```

- [ ] **Step 2: 在 debug/metrics 中写入 summary**

修改 `internal/gateway/routes_debug.go`：在构建 `snap` 后插入：

```go
var qs map[string]sim.QueueStat
var ts map[string]sim.TickStat
if sm != nil {
  qs = sm.QueueStats()
  ts = sm.TickStats()
  snap["world_queue_stats"] = qs
  snap["world_tick_stats"] = ts
}
snap["summary"] = buildMetricsSummary(mt, qs, ts)
```

- [ ] **Step 3: 跑测试转绿**
```bash
go test ./tests/integration -run TestDebugMetrics_IncludesSummary -v
go test ./...
```

- [ ] **Step 4: Commit（summary 实现）**
```bash
git add internal/gateway/metrics_summary.go internal/gateway/routes_debug.go
git commit -m "feat(debug): add metrics summary"
```

---

## Task 3: smoke 脚本失败自动抓取（GREEN）

**Files:**
- Modify: `scripts/smoke_staging.sh`

- [ ] **Step 1: 把失败处理统一到 die() + dump_debug()**

在脚本中新增：
- `dump_debug()`：抓取并打印 debug/build 与 debug/metrics（截断）
- `die()`：打印 FAIL + 调用 dump_debug + exit 1

并把现有 `exit 1` 的路径替换为 `die "..."`

输出格式（示例）：
```
[FAIL] POST /api/v0/intents: expected 200, got 503
--- debug/build ---
{...}
--- debug/metrics ---
{...}
```

- [ ] **Step 2: 本地失败演练**
```bash
BASE_URL=http://localhost:8080 bash scripts/smoke_staging.sh || true
```
Expected: FAIL 后会额外打印 debug/build 与 debug/metrics（可能抓取失败，但应有清晰提示）。

- [ ] **Step 3: staging 实测（可选）**
```bash
bash scripts/smoke_staging.sh
```
Expected: ALL OK（不额外打印 debug 信息）

- [ ] **Step 4: Commit（smoke 更新）**
```bash
git add scripts/smoke_staging.sh
git commit -m "feat(scripts): dump debug build+metrics on smoke failure"
```

---

## Task 4: 推送与部署验收（Render）

- [ ] **Step 1: push main 触发 Render 自动部署**
```bash
git push origin main
```

- [ ] **Step 2: staging 验收**
```bash
curl -sS https://lobster-world-core.onrender.com/api/v0/debug/metrics | python3 -m json.tool | head -n 80
bash scripts/smoke_staging.sh
```
Expected:
- `metrics.summary` 存在且包含 busy/queue/tick
- smoke 成功时不打印 debug；失败时自动打印 debug/build + debug/metrics

