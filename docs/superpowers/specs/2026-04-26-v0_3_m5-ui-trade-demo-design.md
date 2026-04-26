# v0.3-M5：/ui 贸易演示增强（高亮 + demo 按钮）Design（2026-04-26）

## 背景

v0.3-M4 已新增贸易深化事件：
- `market_boom`
- `trade_dispute`

v0.3-M3 的 `/ui` 已支持：
1) 关键事件高亮（betrayal/war_started/…）
2) demo 按钮（触发背叛/宣战）

为了让贸易线也能“一键出效果”，本里程碑补齐 UI 层的高亮与 demo 按钮。

---

## Goal

1) `/ui` 对 `market_boom` / `trade_dispute` 进行高亮（SSE 事件流 + 回放入口 + 可选 world.summary）
2) `/ui` 增加两个 demo 按钮：
   - `触发繁荣`：提交 `goal="开放贸易：市场繁荣"`（触发 `market_boom`）
   - `触发封锁`：提交 `goal="封锁：加税关税"`（触发 `trade_dispute`）

---

## Non-goals

- 不新增/修改后端 API
- 不引入前端框架/打包流程
- 不做 UI 重排（只在现有 intents row 里新增两个按钮）

---

## 设计

### 1) 事件高亮扩展

在 `internal/gateway/ui_page.html` 的 `EVENT_STYLES` 增加两项（示例配色）：
- `market_boom`：label=`boom`，绿/青色系
- `trade_dispute`：label=`dispute`，紫/灰色系

然后复用现有高亮逻辑：
- SSE：`formatEventHTML(obj, raw)` 命中 type 即显示 badge
- replays：`addReplayLink(worldId, eventId, title, obj.type)` 命中 type 即显示 badge
- world.summary：如果行文本包含该 type，则 `summary-highlight`

### 2) demo 按钮

在 intents 区域加入：
- `btn_demo_boom` → `submitDemoGoal('开放贸易：市场繁荣')`
- `btn_demo_dispute` → `submitDemoGoal('封锁：加税关税')`

按钮行为与既有 demo 按钮一致：
- 使用当前 `world_id`
- 成功后刷新 home
- 失败显示在 status

---

## 测试与验收

### 1) 集成测试门禁（轻量）

扩展/新增一个 UI 测试，确保 `/ui` HTML 中包含：
- `market_boom`
- `trade_dispute`
- `btn_demo_boom` / `btn_demo_dispute`（或对应 goal 字符串）

### 2) 手动验收（staging）

打开：
- `https://lobster-world-core.onrender.com/ui`

步骤：
1) 设置 world_id（建议新值，例如 `w_trade_demo_...`）
2) 点击 `触发繁荣`、`触发封锁`
3) 观察：
   - SSE 中出现 `market_boom`、`trade_dispute` 且高亮
   - 回放入口列表中对应事件链接高亮

