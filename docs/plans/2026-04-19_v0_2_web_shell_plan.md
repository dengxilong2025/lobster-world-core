# v0.2 Web 雏形（Go 内置单页）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 Go server 内新增一个 `/ui` 单页，用于“提交意图 + 观战摘要 + 事件流（SSE）+ 回放入口”，让 20 个龙虾智能体可以开始体验测试（v0.2-M1）。

**Architecture:** Go 服务端新增 `routes_ui.go` 注册 `/ui`，返回一个静态 HTML（内含少量 JS）。JS 直接调用现有 v0 API（`/api/v0/*`）与 SSE（`/api/v0/events`），不引入额外前端工程依赖，确保上线最快、维护成本最低。

**Tech Stack:** Go（现有 gateway）、原生 HTML/CSS/JS、SSE（EventSource）

---

## 0) Files 结构与改动范围（先锁定）

**新增：**
- `internal/gateway/routes_ui.go`：注册 `/ui`，输出 HTML
- `internal/gateway/ui_page.go`：HTML 模板/常量（避免 routes 文件过长）
- `tests/integration/ui_smoke_test.go`：最小冒烟测试（TDD）
- `docs/ui/v0_2_web_shell.md`：给人类/智能体的“操作手册”

**修改：**
- `internal/gateway/handler.go`：把 UI route 注册进 mux
- `docs/version_plan.md`：更新 v0.2-M1 里程碑说明（如有必要）
- `docs/wbs_v0_1.md`：新增 v0.2 相关 WBS（或另建 `wbs_v0_2.md`，二选一）

---

## Task 1: 新增 /ui 的集成冒烟测试（RED → 确认失败）

**Files:**
- Create: `tests/integration/ui_smoke_test.go`

- [ ] **Step 1: 写一个最小 failing test（期望 /ui 存在并返回 HTML）**

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

func TestUI_ServesHTML(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  resp, err := http.Get(s.URL + "/ui")
  if err != nil {
    t.Fatalf("get /ui: %v", err)
  }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK {
    t.Fatalf("expected 200, got %d", resp.StatusCode)
  }
  b, _ := io.ReadAll(resp.Body)
  body := string(b)

  if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
    t.Fatalf("expected text/html content-type, got %q", resp.Header.Get("Content-Type"))
  }
  if !strings.Contains(body, "id=\"world_id\"") {
    t.Fatalf("expected #world_id input, got body head: %q", body[:min(200, len(body))])
  }
  if !strings.Contains(body, "/api/v0/intents") || !strings.Contains(body, "/api/v0/events") {
    t.Fatalf("expected page references v0 api endpoints")
  }
}

func min(a, b int) int {
  if a < b { return a }
  return b
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./...`

Expected: FAIL（404 或 handler 未注册）

- [ ] **Step 3: Commit（仅测试）**

```bash
git add tests/integration/ui_smoke_test.go
git commit -m "test(ui): add /ui smoke test"
```

---

## Task 2: 实现 /ui 路由与 HTML 页面（GREEN）

**Files:**
- Create: `internal/gateway/routes_ui.go`
- Create: `internal/gateway/ui_page.go`
- Modify: `internal/gateway/handler.go`

- [ ] **Step 1: 实现 routes_ui.go（只做 GET /ui）**

```go
package gateway

import "net/http"

func registerUIRoutes(mux *http.ServeMux) {
  mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Header().Set("Cache-Control", "no-store")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(uiPageHTML))
  })
}
```

- [ ] **Step 2: 新增 ui_page.go（静态 HTML，内含最小 JS）**

要求（写进 HTML 注释里，给未来维护者）：
- 页面内任何文案都允许存在（这是网页 UI 文本，不是美术贴图）
- JS 只用 fetch + EventSource（不引入框架）
- UI 功能：
  1) 输入 world_id
  2) 提交 intent（goal 文本）
  3) 打开 SSE：实时显示 events（最多保留最近 N 条）
  4) 拉取 spectator/home：显示 world.stage + world.summary（定时轮询或每收到 event 轮询一次，二选一，推荐“事件触发轮询 + 兜底定时”）
  5) 点击某条 event 的 replay_id → 打开 `/api/v0/replay/highlight?...`（新标签页或弹窗展示 JSON 均可，先简单）

（实现时 HTML 至少包含以下 DOM id，方便智能体/自动化测试）
- `world_id`
- `goal`
- `btn_intent`
- `btn_connect`
- `events`
- `world_stage`
- `world_summary`

- [ ] **Step 3: 在 handler.go 注册 UI routes**

在 `NewHandler` 里注册（放在 registerSpectatorRoutes 之后或之前均可）：

```go
registerUIRoutes(mux)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/routes_ui.go internal/gateway/ui_page.go internal/gateway/handler.go
git commit -m "feat(ui): serve minimal web shell at /ui"
```

---

## Task 3: UI 可用性强化（不改后端语义，保证可测）

**Files:**
- Modify: `internal/gateway/ui_page.go`
- Modify: `tests/integration/ui_smoke_test.go`（必要时）

- [ ] **Step 1: 事件流稳定性**
1) SSE 断开时自动重连（EventSource 自带，但要 UI 显示状态）
2) events list 做上限（例如 200 条）避免 DOM 无限增长

- [ ] **Step 2: world.summary 解析**
把 `world.summary`（数组）渲染为 `<li>` 列表

- [ ] **Step 3: 错误处理**
fetch 失败时在页面显示错误（避免“静默失败”）

- [ ] **Step 4: 运行测试**
Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/ui_page.go tests/integration/ui_smoke_test.go
git commit -m "feat(ui): improve /ui usability and stability"
```

---

## Task 4: 写操作手册（给人类 + 20 智能体）

**Files:**
- Create: `docs/ui/v0_2_web_shell.md`

- [ ] **Step 1: 写清楚“怎么跑起来”**
包含：
1) 本地启动 server 的命令
2) 打开 `http://localhost:<port>/ui`
3) 输入 world_id，提交 goal
4) 观察 events 与 world.summary
5) 点击 replay/highlight

- [ ] **Step 2: 写清楚“智能体测试路径（脚本化）”**
建议固定测试脚本步骤（无论用浏览器还是 HTTP 客户端）：
1) POST `/api/v0/intents`
2) SSE 订阅 `/api/v0/events?world_id=...`
3) GET `/api/v0/spectator/home`
4) GET `/api/v0/replay/highlight?world_id=...&event_id=...`

- [ ] **Step 3: Commit**

```bash
git add docs/ui/v0_2_web_shell.md
git commit -m "docs(ui): add v0.2 web shell usage guide"
```

---

## Task 5: 更新路线图/WBS（让进度管理可见）

**Files:**
- Modify: `docs/version_plan.md`
- Modify: `docs/wbs_v0_1.md`（或 Create: `docs/wbs_v0_2.md`）

- [ ] **Step 1: 把 v0.2-M1 的 deliverables 写具体**
- [ ] **Step 2: WBS 增加 v0.2-M1 的任务与状态**
- [ ] **Step 3: Commit**

```bash
git add docs/version_plan.md docs/wbs_v0_1.md
git commit -m "docs(plan): track v0.2 web shell milestone"
```

---

## 自检（写完计划后快速检查）

- [ ] 计划中没有 “TODO/TBD/自行处理” 这种占位
- [ ] 每个任务都有：文件清单、明确步骤、可运行命令、预期结果
- [ ] /ui 的 DOM id 固定，便于智能体/自动化
- [ ] 不引入新的前端工程依赖（保持 Go 内置单页路线）

