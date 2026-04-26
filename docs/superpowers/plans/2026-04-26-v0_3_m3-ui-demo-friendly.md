# v0.3-M3：/ui 演示友好增强（事件高亮 + build/metrics 摘要）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增强 `/ui` 演示体验：展示 build 信息（git_sha/uptime）、展示 debug/metrics 的 summary（busy/queue/tick），并在 SSE 事件流与回放入口高亮关键事件（betrayal/war_started/alliance_formed/treaty_signed/trade_agreement），加“一键触发背叛/宣战”按钮。

**Architecture:** 仅修改 `internal/gateway/ui_page.html`（单文件前端 JS），不改 API。新增两个 fetch：`/api/v0/debug/build` 与 `/api/v0/debug/metrics`，best-effort 渲染 + 手动刷新按钮；事件高亮通过对 SSE message 的 JSON parse 生成带标签的行文本；回放入口 link 文本同样加标签。

**Tech Stack:** HTML/JS（无框架），Go ServeMux 现有 UI 路由，integration tests（如需）。

---

## 0) Files（锁定）

**Modify:**
- `internal/gateway/ui_page.html`

**Modify (可选门禁):**
- `tests/integration/ui_smoke_test.go`（如果该测试对新增 DOM/文案敏感，更新断言；否则只追加一个最小门禁）

---

## Task 1: TDD — 新增/更新 UI 门禁（RED）

> 目标：防止未来误删 build/metrics 信息区与关键事件标签逻辑。  
> 注：如果现有 UI 测试非常轻量（只检查 /ui 200），这里就新增一个“包含关键字”的门禁即可。

**Files:**
- Modify or Create: `tests/integration/ui_demo_friendly_test.go`

- [ ] **Step 1: 写 failing test（检查 /ui HTML 包含关键 endpoint 常量 + debug endpoint）**

```go
package integration

import (
  "io"
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"
  "time"

  "lobster-world-core/internal/gateway"
)

func TestUI_IncludesDemoFriendlyBlocks(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  resp, err := http.Get(s.URL + "/ui")
  if err != nil { t.Fatalf("GET /ui: %v", err) }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK { t.Fatalf("expected 200, got %d", resp.StatusCode) }
  b, _ := io.ReadAll(resp.Body)
  html := string(b)

  // Ensure debug endpoints are referenced.
  if !strings.Contains(html, "/api/v0/debug/build") {
    t.Fatalf("ui should reference /api/v0/debug/build")
  }
  if !strings.Contains(html, "/api/v0/debug/metrics") {
    t.Fatalf("ui should reference /api/v0/debug/metrics")
  }
  // Ensure key event types appear in highlight mapping.
  for _, typ := range []string{"betrayal", "war_started"} {
    if !strings.Contains(html, typ) {
      t.Fatalf("ui should reference event type %q for highlighting", typ)
    }
  }
}
```

- [ ] **Step 2: 运行确认失败（RED）**
```bash
go test ./tests/integration -run TestUI_IncludesDemoFriendlyBlocks -v
```

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/ui_demo_friendly_test.go
git commit -m "test(ui): gate demo-friendly debug blocks"
```

---

## Task 2: UI 增加 build/metrics 摘要区（GREEN）

**Files:**
- Modify: `internal/gateway/ui_page.html`

- [ ] **Step 1: 增加 DOM：Build / Metrics 区域 + 刷新按钮**

在 intents row 下方插入：
- `Build：<span id="build_info">...</span>`
- `Metrics：<span id="metrics_info">...</span>`
- `button id="btn_refresh_debug"`

- [ ] **Step 2: 增加 JS：fetchDebug()**

新增常量：
```js
const API_DEBUG_BUILD = '/api/v0/debug/build';
const API_DEBUG_METRICS = '/api/v0/debug/metrics';
```

实现：
- `fetchBuild()`：拉取 build，展示 `git_sha`、`uptime_sec`
- `fetchMetricsSummary()`：拉取 metrics，展示 `summary.busy/queue/tick`
- `fetchDebug()`：并行调用上述两者（Promise.allSettled）

页面加载后调用一次（best-effort），并绑定 `btn_refresh_debug`。

- [ ] **Step 3: Commit（build/metrics UI）**
```bash
git add internal/gateway/ui_page.html
git commit -m "feat(ui): show build and metrics summary in /ui"
```

---

## Task 3: 事件高亮（SSE + 回放入口）(GREEN)

**Files:**
- Modify: `internal/gateway/ui_page.html`

- [ ] **Step 1: 增加样式与映射**

增加一个映射（示例）：
```js
const EVENT_STYLES = {
  betrayal: { label: 'betrayal', color: '#b91c1c' },
  war_started: { label: 'war', color: '#b45309' },
  alliance_formed: { label: 'alliance', color: '#2563eb' },
  treaty_signed: { label: 'treaty', color: '#1d4ed8' },
  trade_agreement: { label: 'trade', color: '#059669' },
};
```

并实现 `formatEventLine(obj, raw)`：
- 若 obj.type 命中映射：返回 `"[label] " + (obj.narrative||raw)` 的文本
- 并在 UI 上用 `<span>` 包装 label（颜色/粗体）

由于 `#events` 是 `<pre>`，建议改成：
- 维护 `lastEventNodes`，渲染到一个 `<div id="events_rich">`（monospace + pre-wrap），替代 `<pre>` 纯文本
（或保持 `<pre>` 但只做文本前缀，不做颜色；本轮目标是“高亮”，所以建议用 rich div）

- [ ] **Step 2: 回放入口文本加标签**

在 `addReplayLink` 中：
- parse type（从 `obj.type`），若命中映射，则 link 文本为 `"[label] " + title`

- [ ] **Step 3: Commit（事件高亮）**
```bash
git add internal/gateway/ui_page.html
git commit -m "feat(ui): highlight key story events"
```

---

## Task 4: 一键触发背叛/宣战（GREEN）

**Files:**
- Modify: `internal/gateway/ui_page.html`

- [ ] **Step 1: 增加两个按钮 + 绑定事件**

新增按钮：
- `btn_demo_betrayal`（触发背叛：翻脸）
- `btn_demo_war`（触发宣战：开战）

逻辑：
- 读取当前 world_id
- 调用 `postIntent(worldId, "背叛：翻脸")` / `postIntent(worldId, "宣战：开战")`
- 成功后调用 `fetchHome(worldId)`

- [ ] **Step 2: Commit（demo buttons）**
```bash
git add internal/gateway/ui_page.html
git commit -m "feat(ui): add demo buttons for betrayal/war"
```

---

## Task 5: 全量测试 + 部署验收

- [ ] **Step 1: go test**
```bash
go test ./...
```

- [ ] **Step 2: push main 触发 Render 部署**
```bash
git push origin main
```

- [ ] **Step 3: staging 手动验收**
打开：
- `https://lobster-world-core.onrender.com/ui`

检查：
- 页面展示 git_sha/uptime
- 页面展示 metrics summary 三行
- 点击“触发背叛/触发宣战”后 SSE 与回放入口出现高亮事件

