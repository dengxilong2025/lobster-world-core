# v0.3-M3：/ui 演示友好增强（事件高亮 + build/metrics 摘要）Design（2026-04-26）

## Goal

提升演示与外测的“开箱即看懂”体验：打开 `/ui` 就能直观看到当前部署版本、运行状态摘要，并且在事件流/回放入口中高亮关键剧情事件（尤其是 v0.3 的 `betrayal/war_started`）。

本里程碑只做 **UI 层轻增强**，不新增/不修改业务 API。

---

## Non-goals

- 不引入前端框架/打包流程（仍使用 `internal/gateway/ui_page.html` 单文件）
- 不做复杂图表（保持轻量、可在 staging 快速加载）
- 不做定时轮询（默认只在页面加载时拉取一次；可提供“刷新”按钮）

---

## 依赖与现状

现有端点：
- `GET /api/v0/debug/build`：已能返回 `git_sha`、`uptime_sec` 等
- `GET /api/v0/debug/metrics`：已包含 `metrics.summary.busy/queue/tick`
现有 UI：
- `internal/gateway/ui_page.html`：包含 intents 提交、SSE 事件流、replay/highlight 链接

---

## 设计

### 1) UI 新增“版本 & 排障摘要”区域

在页面顶部（world_id/goal 一行的下方）新增一块信息区：

1) **Build 信息**
   - 数据源：`GET /api/v0/debug/build`
   - 展示字段：`git_sha`、`uptime_sec`（可补 `start_time`）
   - 目的：演示时一眼确认“线上跑的是哪次 commit”以及“是否刚重启”

2) **Metrics 摘要**
   - 数据源：`GET /api/v0/debug/metrics`
   - 展示字段：`metrics.summary.busy`、`metrics.summary.queue`、`metrics.summary.tick`
   - 目的：当出现 503/卡顿/性能抖动时，不用离开 UI 手动 curl

3) **刷新按钮**
   - `刷新 build/metrics`：手动重新拉取上述两个端点
   - 默认页面加载时自动拉取一次（best-effort，失败不阻塞 UI）

失败容错：
- 请求失败时显示 `（不可用：<error>）`，不影响 intents、SSE、回放功能。

---

### 2) 事件高亮（SSE / hot_events / 回放入口）

对以下事件类型加显著的标签样式（颜色 + 粗体）：
- **危险/戏剧性**：`betrayal`、`war_started`
- **外交正向**：`alliance_formed`、`treaty_signed`
- **贸易正向**：`trade_agreement`

展示位置：
1) SSE 事件流（`#events` 的每一行）：
   - 若能 parse 成 JSON 且有 `type`，则将显示文本做“前缀标签”，例如：
     - `[betrayal] 关系裂变：...`
2) 回放入口列表（`#replays`）：
   - link 文本前缀同样加 `[type]` + 对应颜色
3) 可选：世界摘要（`world.summary`）中若包含 `betrayal/war_started` 字符串，给该 li 加一个轻量强调样式（不强制）

---

### 3) “一键触发”演示按钮

在 intents 区域增加两个小按钮（不改变原有输入框逻辑）：
- `触发背叛`：自动 `goal="背叛：翻脸"`
- `触发宣战`：自动 `goal="宣战：开战"`

行为：
- 使用当前 `world_id`，调用现有 `postIntent(worldId, goal)`，成功后刷新 home
- 失败显示在 status（复用现有 setStatus）

---

## 验收标准

1) 打开 `/ui`：
   - 页面能显示 `git_sha`（来自 debug/build）
   - 页面能显示 `busy/queue/tick` 三条摘要（来自 debug/metrics）
2) 触发背叛/宣战：
   - SSE 事件流中出现对应事件行，且被高亮
   - 回放入口列表中对应事件的链接被高亮
3) 所有既有 UI 集成测试（如有）继续通过

