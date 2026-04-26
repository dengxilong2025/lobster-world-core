# v0.3-M2：外交深化（背叛 + 宣战）Design（2026-04-26）

## Goal

在不增加新 API、保持决定论与可回放的前提下，新增两类“更有戏”的外交事件，让演示与外测更可感知：

1) `betrayal`：背叛/翻脸引发的关系裂变  
2) `war_started`：宣战/开战引发的冲突升级

并确保：
- `/api/v0/replay/export` 可稳定验证事件出现（可门禁）
- `/api/v0/spectator/home` 的摘要/建议能更明确提到这些事件类型（可感知）

## Non-goals

- 不引入复杂外交状态机（同盟/条约的生命周期后续里程碑做）
- 不做战斗结算（`battle_resolved`）与多轮战争进程
- 不改事件存储模型，不改导出格式

---

## 现状（关键约束）

1) 写侧（sim）在 `action_completed` 之后可追加 0~1 条 world scope “剧本事件”（v0.3-M1 已有 `alliance_formed/treaty_signed/trade_agreement`）。
2) spectator projection 已对 world scope、actors>=2 的多类事件（含 `betrayal`、`war_started`）有关系网增益逻辑（因此新增事件能立刻在 spectator 侧体现）。
3) 决定论：actors 必须可由 world seed/tick/intent_id 稳定推导；不能使用 wall clock。

---

## 设计

### 1) 新增两条 intent→story 规则

在 `internal/sim/intent_story_rules.go` 的关键词规则中新增：

#### 1.1 betrayal

**触发关键词（goal 包含任一）**
- `背叛`、`翻脸`（可后续扩充：`倒戈`、`叛变`）

**产出事件**
- `Type: "betrayal"`
- `Scope: "world"`
- `Actors: [nation_x, nation_y]`（2 个不同 actors，沿用既有 pool 与 pick2DistinctForIntent）

**Delta（建议值，门禁将锁定）**
- `trust: -10`
- `conflict: +8`
- `order: -2`

**Narrative（示例）**
- `关系裂变：背叛发生（目标：<goal>）`

**Trace**
- 回溯到 `action_completed`（必带）
- 如可得再回溯 `intent_accepted`

#### 1.2 war_started

**触发关键词（goal 包含任一）**
- `宣战`、`开战`

**产出事件**
- `Type: "war_started"`
- `Scope: "world"`
- `Actors: [nation_x, nation_y]`

**Delta（建议值，门禁将锁定）**
- `conflict: +10`
- `order: -3`
- `trust: -4`

**Narrative（示例）**
- `战端开启：宣战（目标：<goal>）`

**Trace**
- 同上

---

### 2) 规则优先级（避免一次 intent 产出多条额外事件）

保持 v0.3-M1 的“每个 intent 最多追加 1 条剧本事件”的约束，并更新优先级为：

`betrayal / war_started`（更强戏剧性）  
→ `alliance_formed / treaty_signed`（外交正向）  
→ `trade_agreement`（贸易）  
→ default（不追加）

---

### 3) spectator/home 的可感知增强（最小）

在 `internal/gateway/world_summary.go` 的建议/看点文案中补一句更明确的提示（不改 API 结构）：

- 当近期叙事包含 `背叛/翻脸` 或冲突高企时，建议中明确提到：
  - 目标关键词：`背叛/翻脸/宣战/开战`
  - 预期事件：`betrayal / war_started`

目标是让玩家“看到建议就能写出能触发的 intent，并知道会出现什么事件类型”。

---

## 测试与验收

### 1) 集成测试门禁（新增/扩展）

扩展现有集成测试（优先复用 `tests/integration/intent_story_rules_test.go` 或新增一个相邻测试）覆盖：

1) 提交 `goal="背叛：翻脸"` 后，export 中必须出现 `"type":"betrayal"`  
2) 提交 `goal="宣战：开战"` 后，export 中必须出现 `"type":"war_started"`  
3) 两类事件均满足：
   - `actors` 长度为 2 且不同
   - `trace` 至少包含 `action_completed.event_id`
   - `delta` 包含并满足预期方向（betrayal：trust<0、conflict>0；war_started：conflict>0）

### 2) home 文案可见性门禁（轻量）

对 `deriveWorldSummary(...)` 增加/更新单测：
- 在“战乱/背叛语境”下的 summary 中必须出现 `betrayal` 或 `war_started` 字样（或至少一个），以保证可感知。

---

## 风险与缓解

1) **事件增多导致测试脆弱**：门禁只断言“事件类型出现 + trace + delta 方向”，不强绑定 narrative 全文。
2) **决定论破坏**：继续沿用 `pick2DistinctForIntent(worldSeed, tick, intentID, pool)`，禁止引入 wall clock。

