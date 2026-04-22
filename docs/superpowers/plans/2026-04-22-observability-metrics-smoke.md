# 可观测性/指标（metrics）+ 端到端 smoke 门禁 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增一个最小侵入的 `/api/v0/debug/metrics` JSON 指标端点，并补齐一条端到端 smoke 集成测试门禁，覆盖“外交/贸易 intents → home 建议 → 事件出现 → export 可回放”。

**Architecture:** 先用 integration test 建立回归门禁（黑盒 HTTP 驱动）。metrics 采用 gateway 外层 middleware + `writeError` 钩子记录 BUSY；debug 端点返回聚合计数（总请求/按状态码/Busy）。不引入 Prometheus/第三方依赖，不改变现有 API。

**Tech Stack:** Go、`net/http/httptest`、`sync/atomic`。

---

## 0) Files 结构与改动范围（先锁定）

**Create:**
- `tests/integration/e2e_smoke_diplomacy_trade_test.go`
- `internal/gateway/metrics.go`

**Modify:**
- `internal/gateway/handler.go`（返回 mux 外层包 middleware）
- `internal/gateway/routes_debug.go`（新增 debug/metrics 端点）
- `internal/gateway/response.go`（在 BUSY 错误时计数）

---

## Task 1: 端到端 smoke 门禁（TDD 先红）

**Files:**
- Create: `tests/integration/e2e_smoke_diplomacy_trade_test.go`

- [ ] **Step 1: 写 failing test（覆盖：home 建议 + 事件出现 + export）**

```go
package integration

import (
  "bytes"
  "encoding/json"
  "io"
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"
  "time"

  "lobster-world-core/internal/gateway"
)

func TestE2ESmoke_DiplomacyTradeIntent_ToEvents_ToExport(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{
    TickInterval: 5 * time.Millisecond,
    Seed: 123,
  })
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_e2e_smoke"

  postIntent := func(goal string) {
    b, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": goal})
    r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(b))
    if err != nil { t.Fatalf("POST /intents: %v", err) }
    _ = r.Body.Close()
    if r.StatusCode != http.StatusOK { t.Fatalf("intent status=%d goal=%q", r.StatusCode, goal) }
  }

  // 1) build conflict risk deterministically
  for i := 0; i < 10; i++ {
    postIntent("背叛：挑起冲突")
  }
  time.Sleep(250 * time.Millisecond)

  // 2) home hint should mention diplomacy keywords + expected event types
  hr, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=" + worldID)
  if err != nil { t.Fatalf("GET home: %v", err) }
  defer hr.Body.Close()
  if hr.StatusCode != http.StatusOK { t.Fatalf("home status=%d", hr.StatusCode) }
  hb, _ := io.ReadAll(hr.Body)
  hs := string(hb)
  if !strings.Contains(hs, "建议：") { t.Fatalf("expected hints in home, got=%s", hs) }
  if !(strings.Contains(hs, "停战") || strings.Contains(hs, "谈判") || strings.Contains(hs, "条约") || strings.Contains(hs, "结盟")) {
    t.Fatalf("expected diplomacy keywords in home hints, got=%s", hs)
  }
  if !(strings.Contains(hs, "alliance_formed") || strings.Contains(hs, "treaty_signed")) {
    t.Fatalf("expected expected-event types in home hints, got=%s", hs)
  }

  // 3) produce story events deterministically
  postIntent("结盟：达成联盟")
  postIntent("条约：签署停战条约")
  postIntent("贸易：开通商路")
  time.Sleep(250 * time.Millisecond)

  // 4) export should contain story event types
  er, err := http.Get(s.URL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
  if err != nil { t.Fatalf("GET export: %v", err) }
  defer er.Body.Close()
  if er.StatusCode != http.StatusOK { t.Fatalf("export status=%d", er.StatusCode) }
  eb, _ := io.ReadAll(er.Body)
  es := string(eb)
  if !strings.Contains(es, "\"type\":\"alliance_formed\"") { t.Fatalf("missing alliance_formed in export") }
  if !strings.Contains(es, "\"type\":\"treaty_signed\"") { t.Fatalf("missing treaty_signed in export") }
  if !strings.Contains(es, "\"type\":\"trade_agreement\"") { t.Fatalf("missing trade_agreement in export") }
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./... -run TestE2ESmoke_DiplomacyTradeIntent_ToEvents_ToExport -v
```

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/e2e_smoke_diplomacy_trade_test.go
git commit -m "test(e2e): add diplomacy/trade smoke gate"
```

---

## Task 2: metrics（TDD 先红）

**Files:**
- Create: `internal/gateway/metrics.go`
- Modify: `internal/gateway/handler.go`
- Modify: `internal/gateway/routes_debug.go`
- Modify: `internal/gateway/response.go`

- [ ] **Step 1: 新增 metrics 结构（atomic counters）**

在 `internal/gateway/metrics.go`：
- `type Metrics struct { requestsTotal atomic.Int64; busyTotal atomic.Int64; byStatus map[int]*atomic.Int64 ... }`
- `func NewMetrics() *Metrics`
- `func (m *Metrics) IncRequest()`
- `func (m *Metrics) IncStatus(code int)`
- `func (m *Metrics) IncBusy()`
- `func (m *Metrics) Snapshot() map[string]any`（生成 JSON-friendly 结构）

- [ ] **Step 2: middleware 统计请求与状态码**

在 `handler.go` 返回 mux 前包一层：
- 自定义 `statusCapturingResponseWriter` 捕获 `WriteHeader`
- middleware：`m.IncRequest()`，handler 执行后 `m.IncStatus(status)`

- [ ] **Step 3: debug/metrics 路由**

扩展 `registerDebugRoutes`：新增
- `GET /api/v0/debug/metrics` → `writeJSON(ok:true, metrics:m.Snapshot())`

- [ ] **Step 4: BUSY 计数**

在 `writeError` 中，当 `code=="BUSY"` 时 `metrics.IncBusy()`。
（为避免全局变量污染：把 metrics 指针挂在 package-level `defaultMetrics` 并由 `NewHandler` 初始化一次。）

- [ ] **Step 5: 新增 integration test 验证 metrics 增长**

新增测试（可放到 `tests/integration/metrics_debug_test.go` 或复用 e2e 测试）：
- 先请求几次 `/api/v0/me`（401 也算状态码）
- 再 GET `/api/v0/debug/metrics`，断言 `requests_total>0` 且 `responses_by_status` 中对应状态码计数>0

- [ ] **Step 6: Run 全量测试**
```bash
go test ./...
```

- [ ] **Step 7: Commit（实现 metrics）**
```bash
git add internal/gateway/metrics.go internal/gateway/handler.go internal/gateway/routes_debug.go internal/gateway/response.go tests/integration/metrics_debug_test.go
git commit -m "feat(debug): add /debug/metrics counters"
```

---

## Task 3: 文档/交付

- [ ] **Step 1: 更新 roadmap（阶段4 可观测性项）**
- [ ] **Step 2: 打补丁包**
```bash
git format-patch -4 -o /workspace/patches_observability
cd /workspace && zip -qr patches_observability.zip patches_observability
```

