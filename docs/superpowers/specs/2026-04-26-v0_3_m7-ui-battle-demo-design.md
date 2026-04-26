# v0.3-M7：/ui 战争演示增强（battle_resolved 高亮 + demo 按钮）Design（2026-04-26）

## 背景

v0.3-M6 已新增战争后续事件：
- `battle_resolved`

当前 `/ui` 已能高亮并一键触发：
- 外交：`betrayal` / `war_started`
- 贸易：`market_boom` / `trade_dispute`

为了让战争线也“一键出效果”，本里程碑补齐 `battle_resolved` 的 UI 高亮与 demo 按钮。

---

## Goal

1) `/ui` 对 `battle_resolved` 进行高亮（SSE 事件流 + 回放入口 + 可选 world.summary）
2) `/ui` 增加 demo 按钮：
   - `触发会战`：提交 `goal="进攻：发动会战"`（触发 `battle_resolved`）

---

## Non-goals

- 不新增/修改后端 API
- 不引入前端框架/打包流程
- 不做 UI 大改版（只做增量按钮 + 映射扩展）

---

## 设计

### 1) 事件高亮扩展

在 `internal/gateway/ui_page.html` 的 `EVENT_STYLES` 增加：
- `battle_resolved`：label=`battle`，红褐/暗橙色系（与 `war_started` 区分开，但同属战争线）

复用现有逻辑：
- SSE：`formatEventHTML` 命中 type 即显示 badge
- replays：`addReplayLink(..., obj.type)` 命中 type 即显示 badge
- world.summary：行文本包含该 type 时添加 `summary-highlight`

### 2) demo 按钮

在 intents row 增加：
- `btn_demo_battle` → `submitDemoGoal('进攻：发动会战')`

行为与已有 demo 按钮一致：
- 使用当前 world_id
- 成功后刷新 home
- 失败显示在 status

---

## 测试与验收

### 1) 集成测试门禁（轻量）

扩展 UI 测试，确保 `/ui` HTML 中包含：
- `battle_resolved`
- `btn_demo_battle`

### 2) staging 手动验收

打开：
- `https://lobster-world-core.onrender.com/ui`

步骤：
1) 输入新的 world_id（例如 `w_battle_demo_...`）
2) 点击 `触发会战`
3) 观察：
   - SSE 中出现 `battle_resolved` 且高亮
   - 回放入口中对应事件链接高亮

