# v0.3-M5：/ui 贸易演示增强（高亮 + demo 按钮）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 更新 `/ui`：对 `market_boom` / `trade_dispute` 进行高亮显示（SSE + 回放入口 + 可选 world.summary），并新增 demo 按钮“一键触发繁荣/封锁”，让贸易线演示也能一键出效果。

**Architecture:** 仅修改 `internal/gateway/ui_page.html`（单文件 JS），复用 v0.3-M3 的 `EVENT_STYLES`、`formatEventHTML`、`submitDemoGoal` 逻辑：扩展事件样式映射 + 增加两个按钮及 onclick 绑定。测试层只做轻量门禁：确保 /ui HTML 引用了新事件类型与新按钮 id（防回归误删）。

**Tech Stack:** HTML/JS（无框架）、Go embed 静态页面、integration tests（httptest）。

---

## 0) Files（锁定）

**Modify:**
- `internal/gateway/ui_page.html`

**Modify:**
- `tests/integration/ui_demo_friendly_test.go`（扩展断言：包含 market_boom/trade_dispute + 新按钮 id）

---

## Task 1: TDD — 扩展 UI 门禁（RED）

**Files:**
- Modify: `tests/integration/ui_demo_friendly_test.go`

- [ ] **Step 1: 扩展断言（先红）**

在现有测试 `TestUI_IncludesDemoFriendlyBlocks` 中新增断言：

```go
for _, typ := range []string{
  "market_boom",
  "trade_dispute",
} {
  if !strings.Contains(html, typ) {
    t.Fatalf("ui should reference event type %q for highlighting", typ)
  }
}

for _, id := range []string{
  "btn_demo_boom",
  "btn_demo_dispute",
} {
  if !strings.Contains(html, id) {
    t.Fatalf("ui should include demo button id %q", id)
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
git commit -m "test(ui): gate trade demo buttons and highlights"
```

---

## Task 2: UI 事件高亮扩展（GREEN）

**Files:**
- Modify: `internal/gateway/ui_page.html`

- [ ] **Step 1: 扩展 EVENT_STYLES**

在 `EVENT_STYLES` 中新增两项（颜色可微调，但 label 必须稳定）：

```js
market_boom:   { label: 'boom',    color: '#047857', bg: '#d1fae5', border: '#a7f3d0' },
trade_dispute: { label: 'dispute', color: '#6d28d9', bg: '#ede9fe', border: '#ddd6fe' },
```

- [ ] **Step 2: 跑测试（包含全量）**
```bash
go test ./...
```

- [ ] **Step 3: Commit（高亮扩展）**
```bash
git add internal/gateway/ui_page.html
git commit -m "feat(ui): highlight market_boom and trade_dispute"
```

---

## Task 3: demo 按钮（繁荣/封锁）（GREEN）

**Files:**
- Modify: `internal/gateway/ui_page.html`

- [ ] **Step 1: 增加两个按钮**

在 intents row 中（靠近其它 demo 按钮）加入：
```html
<button id="btn_demo_boom" class="btn-sm">触发繁荣</button>
<button id="btn_demo_dispute" class="btn-sm">触发封锁</button>
```

- [ ] **Step 2: 绑定 onclick**

在已有 demo 按钮绑定附近加入：
```js
$('btn_demo_boom').onclick = () => submitDemoGoal('开放贸易：市场繁荣');
$('btn_demo_dispute').onclick = () => submitDemoGoal('封锁：加税关税');
```

- [ ] **Step 3: 全量测试**
```bash
go test ./...
```

- [ ] **Step 4: Commit（demo 按钮）**
```bash
git add internal/gateway/ui_page.html
git commit -m "feat(ui): add trade demo buttons"
```

---

## Task 4: 部署与验收（Render）

- [ ] **Step 1: push main**
```bash
git push origin main
```

- [ ] **Step 2: staging 手动验收**

打开：
- `https://lobster-world-core.onrender.com/ui`

步骤：
1) 设置新的 `world_id`（例如 `w_trade_demo_...`）
2) 点击 `触发繁荣`（应触发 `market_boom`）
3) 点击 `触发封锁`（应触发 `trade_dispute`）
4) 观察：
   - SSE 中对应事件行带 badge 高亮
   - 回放入口对应链接带 badge 高亮

