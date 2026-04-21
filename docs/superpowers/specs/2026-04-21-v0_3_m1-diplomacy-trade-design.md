# v0.3-M1（最小切片）：外交/贸易剧本 + 可解释回放（Design）

**目标**：在不改变“事件溯源 + 投影 + 仿真”主线、不破坏决定论与现有 API 语义的前提下，新增 **2 条**可感知的“意图→事件剧本规则”（外交/结盟 + 贸易/经济），让 `/spectator` 与 `replay/highlight` 更有节目感，同时保持 `go test ./...` 全绿、文档与 WBS 同步更新。

**非目标（本轮不做）**
- 不引入新 API（仍走 `/api/v0/intents`、SSE、`replay/*`、`spectator/*`）
- 不引入新存储（EventStore 仍为 canonical log；Projection 仍可丢弃重建）
- 不做“冲击节目单式脚本化回放”（shock 相关另起里程碑）
- 不引入复杂规则引擎（仍是最小关键词规则 + 可解释 trace）

---

## 1) 背景与现状

当前写侧意图执行在 `internal/sim/world.go`：
- `intent_accepted` 立即写入并发布
- 每 tick 执行 1 条 intent：`action_started` → `action_completed`
- `action_completed` 通过 `intentDelta(goal)` 产生 v0 的确定性 delta，并写入 trace 解释

当前读侧关系视图在 `internal/projections/spectator/projection.go`：
- world scope、actors>=2 的事件类型（`alliance_formed`、`trade_agreement`、`treaty_signed`、`betrayal`、`war_started`、`battle_resolved`）会影响 ally/enemy 关系与原因（relation_reasons）

因此，本轮新增剧本最“省力高收益”的方向是：
> 让某些 intent 在 `action_completed` 之后额外产生 **world scope 的外交/贸易事件**（actors=两国），从而自动增强 spectator/entity 与 replay 的叙事素材。

---

## 2) 设计原则（防偏离）

1. **决定论**：不得引入 wall clock（`time.Now`）参与核心选择；所有 actor 选择与事件序列必须可由 world seed/tick/intent_id 推导。
2. **不破坏既有测试假设**：现有集成测试 `TestIntents_AreProcessedByTickSimWithMonotonicTick` 读取前三条事件并断言顺序为 accepted/started/completed。  
   → 本轮新增事件必须发生在 `action_completed` 之后（同 tick 内亦可）。
3. **可解释性**：新增事件必须带 trace，至少能回溯到 `action_completed` 与 `intent_accepted`（如存在）。
4. **文档同步**：WBS/roadmap/qa_gates 或相关 docs 需在合并时同步更新（避免“计划-现实漂移”）。

---

## 3) 新增剧本（规则集）

> 只新增 2 条“强感知规则”：外交/结盟、贸易/经济；其余 goal 仍走现有 `intentDelta` 默认路径。

### 3.1 外交/结盟剧本

**触发关键词（goal 包含其一即可）**
- 结盟、联盟、条约、停战、谈判

**产出事件（在 action_completed 之后追加）**
- `alliance_formed`（当 goal 含“结盟/联盟”）
- `treaty_signed`（当 goal 含“条约/停战/谈判”）

**actors**
- 必须为 2 个不同的“国家/阵营” ID（例如 `nation_a`、`nation_b`），用于驱动 spectator/entity 的关系网。

**delta（示例，可在实现中微调但必须测试锁定）**
- `trust +8`
- `order +2`
- `conflict -3`

**narrative（示例）**
- `外交突破：nation_a 与 nation_b 达成同盟（目标：<goal>）`

**trace（至少 2 条）**
- `CauseEventID = action_completed.event_id`，Note：`外交推进：从意图执行结果导出`
- `CauseEventID = intent_accepted.event_id`（如果可得），Note：`目标来源：<goal>`

### 3.2 贸易/经济剧本

**触发关键词（goal 包含其一即可）**
- 贸易、集市、交换、商路

**产出事件（在 action_completed 之后追加）**
- `trade_agreement`

**delta（示例）**
- `food +5`
- `trust +3`
- `knowledge +1`
- `conflict -1`

**narrative（示例）**
- `贸易达成：nation_a 与 nation_b 开通商路（目标：<goal>）`

**trace（同上）**

---

## 4) actor 选择（决定论策略）

为保证确定性与可复现，本轮采用固定 actor pool，并用 seed/tick/intent_id 做稳定选择：

- 固定 pool（v0）：`[nation_a, nation_b, nation_c, nation_d, nation_e, nation_f]`
- 选择函数要求：
  - 同一 world、同一 tick、同一 intent_id → 选出的两国固定不变
  - 选出的两国必须不同
  - 不依赖 wall clock

实现建议（不限定唯一实现）：
- 复用现有 `shock.go` 的 `seedFor(worldSeed, epochStart)` 思路，新增一个 `pick2DistinctForIntent(worldSeed, tick, intentID, pool)`  
  或者复用 `pick2Distinct(worldSeed, epochStart, pool)`，把 `epochStart` 替换为一个稳定整数（例如 `tick + hash(intentID)`）。

---

## 5) 写侧实现点（代码边界）

> 约束：不改 API，只在 sim 写侧增加“额外事件”。

建议改动位置：
- `internal/sim/intent_rules.go`
  - 扩展 `intentDelta(goal)`：让外交/贸易的 delta 更符合剧本（并更新 `explainIntentRule` 文案）
  - 新增 `intentExtraWorldEvents(goal, ctx...)`：返回要追加的外交/贸易事件类型与 delta/narrative/trace 所需信息
- `internal/sim/world.go`
  - 在 `action_completed` appendAndPublish 成功后，按规则追加 0~1 条（或 0~2 条）world 事件（外交/贸易），并对世界状态应用 delta
  - 保证事件顺序：accepted → started → completed → extra_events（本轮新增）

---

## 6) 测试与验收（必须可复测）

### 6.1 新增集成测试（TDD）

新增：`tests/integration/intent_story_rules_test.go`（文件名可微调）

覆盖：
1) 提交外交 goal（如“发起结盟谈判”）后：
   - SSE 或 replay/export 中能看到 `alliance_formed` 或 `treaty_signed`
   - 该事件 `actors` 长度为 2 且不同
   - 该事件 `delta` 生效（可通过 `GET /api/v0/world/status` 观察信任/秩序/冲突变化，或通过导出事件 delta 断言）
2) 提交贸易 goal（如“组织集市交换物资”）后：
   - 能看到 `trade_agreement`
   - `delta` 与 trace 存在并可 Validate
3) 决定论门禁（最小）：
   - 同 seed、同输入序列 → export 输出稳定（已有 determinism 测试；本轮新增事件不得引入非确定性）

### 6.2 现有门禁必须继续通过
- `go test ./...` 全绿
- 既有 replay/export/highlight、spectator、SSE 等测试不回归

---

## 7) 文档同步（本轮交付的一部分）

实现合并时同步更新：
- `docs/roadmap.md`：阶段 3 里“更丰富规则/更稳定冲击回放”的进度（只更新本轮相关项）
- `docs/wbs_v0_1.md` 或 `docs/version_plan.md`：如需要，记录 v0.3-M1 的“新增 2 个剧本规则”已经开始落地
- （可选）`docs/retrospectives/YYYY-MM-DD_P3.md`：以 3~5 行记录本轮改动、风险与下一步

---

## 8) 风险清单与缓解

1) **事件顺序破坏现有测试**  
缓解：新增事件必须发生在 `action_completed` 之后；并以集成测试锁定。

2) **引入非决定论**  
缓解：actor 选择只依赖 seed/tick/intent_id；不使用 wall clock；并用 determinism 测试兜底。

3) **文档漂移**  
缓解：合并时必须更新 WBS/roadmap 相关段落，保持“计划-现实对齐”。

