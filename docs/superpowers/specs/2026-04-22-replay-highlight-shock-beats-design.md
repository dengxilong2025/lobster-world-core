# replay/highlight：冲击期脚本化回放（beats 更一致）Design

**目标**：当回放目标事件位于某个 shock（冲击）生命周期内（或本身就是 shock 事件）时，`/api/v0/replay/highlight` 输出更稳定的“纪录片分镜”beats：优先按 shock 生命周期（warning → started →（betrayal）→ ended）组织，减少随机邻居事件带来的结构漂移。

**范围**：
- 仅修改 `internal/gateway/routes_replay.go` 中 `buildReplayBeats`
- 增加必要的测试（优先 integration 测试）
- 不修改事件 schema、不修改 API 返回结构（仍为 `{duration_sec, beats[]}`，beat 仍为 `{t, caption}`）

**非目标**：
- 不做 EventStore 新索引（不加按 meta.shock_key 查询接口）
- 不引入复杂的“全链路推理”，只用 event log 里的 `Type`/`Meta.shock_key`/`Trace` 做归纳

---

## 1) 背景与问题

当前 `buildReplayBeats` 的结构：
- opener（t=0）：target narrative
- t=2/4/6/7：world summary stage + 近期 + 1条风险/建议 + 1条看点
- 中段：从 target.Trace 或 prev/next 提取 1~4 条“因为/进展/转折”
- 收尾：固定“下一步：关注冲击/背叛/迁徙窗口”

问题在 shock 场景：
- target 若不是 shock_* 本身，而是 betrayal 或别的关联事件，中段 beats 会随邻居事件变化而漂移
- 玩家想看到“冲击到底发生了什么/从何开始/何时结束”，但 beats 不够稳定

---

## 2) 方案（推荐：轻量抽取 shock 生命周期）

### 2.1 判定：何时启用 shock 脚本化模式

当满足任一条件，启用 shock 模式：
1) `target.Type` 是 `shock_warning|shock_started|shock_ended`
2) `target.Meta["shock_key"]` 存在且为非空字符串
3) `target.Trace` 中某条 cause event 的 `Meta["shock_key"]` 存在（通过 `es.GetByID` 回溯 1 层）

得到 `shockKey` 后，进入抽取流程。

### 2.2 抽取：如何拿到该 shockKey 的 lifecycle 事件

为了不改 store 接口，采用“最近事件扫描”：
- `es.Query(WorldID, SinceTs=0, Limit=2000)`（limit 可配置为常量）
- 过滤 `Meta["shock_key"] == shockKey` 且 `Type` 属于：
  - `shock_warning`
  - `shock_started`
  - `betrayal`（可选，若 scheduler 配了 ActorsPool）
  - `shock_ended`
- 取每个类型的“最靠近 target.Ts 的那一条”（或按 tick 区间：`warning <= started <= ended` 的第一组）

### 2.3 输出：beats 的稳定结构（4~6 个）

shock 模式下，beats 固定为：
1) t=0：`target.Narrative`（保持不变）
2) t=2：`冲击预警：...`（若存在 shock_warning）
3) t=6：`冲击开始：...`（若存在 shock_started）
4) t=12：`关系翻转：...`（若存在 betrayal，caption 可为 betrayal.Narrative）
5) t=18：`冲击结束：...`（若存在 shock_ended）
6) 仍保留 world summary 的 “世界阶段/近期/风险或建议/看点” 其中 2 条（阶段+近期优先），插入到 t=3/4 附近，确保结构可读但不过载。

兜底规则：
- 缺哪一项就跳过；但仍保证 beats 数≥3（opener + stage + ending）
- `ending` 保持现有 `t=28` 固定句式

---

## 3) 测试门禁（优先 integration）

新增 integration test（或扩展现有）：
- 启用 ShockConfig（类似 `TestReplayExport_ReturnsStableNDJSONSorted` 的配置）
- 创建 world，触发 intent，sleep 足够时间让 warning/started/ended 发生
- 选择一个 `shock_started` 事件作为 target，调用 `/api/v0/replay/highlight`
- 断言 beats 中包含：
  - opener（t=0）
  - 至少一个 caption 含 `冲击预警：` 或包含 `shock_warning` narrative
  - 至少一个 caption 含 `冲击开始：` 或包含 `shock_started` narrative
  - 至少一个 caption 含 `冲击结束：` 或包含 `shock_ended` narrative
  - 收尾 `下一步：关注冲击/背叛/迁徙窗口`

并确保输出 beats 排序稳定（已有排序逻辑）。

---

## 4) 风险与权衡

- 通过扫描最近 2000 条事件抽取 lifecycle：实现简单，但在极端长世界日志下性能一般；MVP 可接受
- 若后续 highlight 频繁使用且数据量增长，再升级为 store 索引（下一阶段再做）

