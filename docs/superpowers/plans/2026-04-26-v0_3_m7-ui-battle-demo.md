# v0.3-M7：/ui 战争演示增强（battle_resolved 高亮 + demo 按钮）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 更新 `/ui`：对 `battle_resolved` 进行高亮显示（SSE + 回放入口 + 可选 world.summary），并新增 demo 按钮“一键触发会战”，让战争线演示也能一键出效果。

**Architecture:** 仅修改 `internal/gateway/ui_page.html`（单文件 JS）：扩展 `EVENT_STYLES` 添加 `battle_resolved`，并在 intents row 增加按钮 `btn_demo_battle`，onclick 绑定到 `submitDemoGoal('进攻：发动会战')`。测试层做轻量门禁：扩展 `tests/integration/ui_demo_friendly_test.go`，断言 `/ui` HTML 中包含 `battle_resolved` 与 `btn_demo_battle`。

**Tech Stack:** HTML/JS（无框架）、Go embed 静态页面、integration tests（httptest）。

---

## 0) Files（锁定）

**Modify:**
- `internal/gateway/ui_page.html`
- `tests/integration/ui_demo_friendly_test.go`

---

## Task 1: TDD — 扩展 UI 门禁（RED）

**Files:**
- Modify: `tests/integration/ui_demo_friendly_test.go`

- [ ] **Step 1: 扩展断言（先红）**

在 `TestUI_IncludesDemoFriendlyBlocks` 中追加：

```go
if !strings.Contains(html, "battle_resolved") {
  t.Fatalf("ui should reference event type %q for highlighting", "battle_resolved")
}
if !strings.Contains(html, "btn_demo_battle") {
  t.Fatalf("ui should include demo button id %q", "btn_demo_battle")
}
```

- [ ] **Step 2: 运行确认失败（RED）**
```bash
go test ./tests/integration -run TestUI_IncludesDemoFriendlyBlocks -v
```

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/ui_demo_friendly_test.go
git commit -m "test(ui): gate battle demo button and highlight"
```

---

## Task 2: UI 高亮 battle_resolved（GREEN）

**Files:**
- Modify: `internal/gateway/ui_page.html`

- [ ] **Step 1: 扩展 EVENT_STYLES**

在 `EVENT_STYLES` 增加：

```js
battle_resolved: { label: 'battle', color: '#7c2d12', bg: '#ffedd5', border: '#fed7aa' },
```

（颜色可微调，但 label 必须稳定，便于识别。）

- [ ] **Step 2: 全量测试**
```bash
go test ./...
```

- [ ] **Step 3: Commit（高亮扩展）**
```bash
git add internal/gateway/ui_page.html
git commit -m "feat(ui): highlight battle_resolved"
```

---

## Task 3: demo 按钮（触发会战）（GREEN）

**Files:**
- Modify: `internal/gateway/ui_page.html`

- [ ] **Step 1: intents row 增加按钮**

在现有 demo 按钮附近加入：

```html
<button id="btn_demo_battle" class="btn-sm">触发会战</button>
```

- [ ] **Step 2: 绑定 onclick**

在 demo 绑定区域加入：

```js
$('btn_demo_battle').onclick = () => submitDemoGoal('进攻：发动会战');
```

- [ ] **Step 3: 全量测试**
```bash
go test ./...
```

- [ ] **Step 4: Commit（demo 按钮）**
```bash
git add internal/gateway/ui_page.html
git commit -m "feat(ui): add battle demo button"
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
1) 输入新的 world_id（例如 `w_battle_demo_...`）
2) 点击 `触发会战`
3) 观察：
   - SSE 中出现 `battle_resolved` 且高亮 badge
   - 回放入口对应事件链接高亮 badge

