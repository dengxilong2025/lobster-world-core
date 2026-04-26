# v0.3-M4：贸易深化（繁荣 + 纠纷）Design（2026-04-26）

## Goal

在不增加新 API、保持决定论与可回放的前提下，补齐贸易线的“更可感知剧情事件”，让玩家除了外交（背叛/宣战）之外，也能通过贸易选择推动叙事分叉：

1) `market_boom`：贸易繁荣（资源变多、信任上升、冲突缓和）  
2) `trade_dispute`：贸易纠纷（信任下滑、冲突升高、秩序受损）

并确保：
- `/api/v0/replay/export` 可稳定验证事件出现（门禁）
- `/api/v0/spectator/home` 的建议更明确提到可触发关键词与预期事件类型

---

## Non-goals

- 不引入商品/货币系统、不做价格曲线
- 不做多回合贸易战/制裁链条（后续里程碑再做）
- 不改导出格式、不引入新存储

---

## 现状（关键约束）

- sim 写侧已支持在 `action_completed` 之后追加“剧本事件”（当前已覆盖：`alliance_formed/treaty_signed/trade_agreement/betrayal/war_started`）。
- 决定论要求：actors 必须由 `world seed/tick/intent_id` 推导，不使用 wall clock。
- 为避免测试脆弱：每个 intent 最多追加 1 条剧本事件（保持 v0.3 约束）。

---

## 设计

### 1) 新增 intent → story 规则

在 `internal/sim/intent_story_rules.go` 的 `intentStorySpec(goal string)` 中新增两条规则（关键词触发）：

#### 1.1 market_boom（贸易繁荣）

**触发关键词（goal 包含任一）**
- `繁荣`、`互市`、`市场`、`开放贸易`

**产出事件**
- `Type: "market_boom"`
- `Scope: "world"`
- `Actors: [nation_x, nation_y]`（2 个不同 actors，沿用既有 pool 与 pick2DistinctForIntent）

**Delta（建议值，门禁将锁定）**
- `food: +8`
- `knowledge: +2`
- `trust: +2`
- `conflict: -1`

**Narrative（示例）**
- `贸易繁荣：市场兴旺（目标：<goal>）`

**Trace**
- 回溯到 `action_completed`（必带）
- 如可得再回溯 `intent_accepted`

#### 1.2 trade_dispute（贸易纠纷）

**触发关键词（goal 包含任一）**
- `封锁`、`禁运`、`加税`、`关税`

**产出事件**
- `Type: "trade_dispute"`
- `Scope: "world"`
- `Actors: [nation_x, nation_y]`

**Delta（建议值，门禁将锁定）**
- `food: -3`
- `trust: -6`
- `conflict: +4`
- `order: -1`

**Narrative（示例）**
- `贸易纠纷：封锁与反制（目标：<goal>）`

**Trace**
- 同上

---

### 2) 规则优先级（保持“每 intent 最多 1 条额外事件”）

更新 story 优先级为：

`betrayal / war_started`（戏剧性最高）  
→ `trade_dispute / market_boom`（贸易分叉）  
→ `alliance_formed / treaty_signed`（外交正向）  
→ `trade_agreement`（基础贸易）  
→ default（不追加）

---

### 3) spectator/home 的可执行建议增强（最小）

在 `internal/gateway/world_summary.go` 的建议文案中补充两点：

1) 当 `Food <= 20`（或饥荒阶段）：
   - 现有建议已提示 `trade_agreement`，本轮再加一句：
   - `也可尝试“繁荣/互市/开放贸易”（market_boom），或在冲突语境下尝试“封锁/关税”（trade_dispute）观察走向`

2) 当冲突偏高但尚未到战争（例如 `Conflict >= 40 && < 60`）：
   - 给一条“贸易施压/封锁”建议，并点名 `trade_dispute`

目标：玩家看到建议即可写出触发 intent，并能预期会出现哪些事件类型。

---

## 测试与验收

### 1) 集成测试门禁（新增）

新增测试覆盖：
1) 提交 `goal="开放贸易：市场繁荣"` 后，export 中必须出现 `"type":"market_boom"`
2) 提交 `goal="封锁：加税关税"` 后，export 中必须出现 `"type":"trade_dispute"`
3) 两类事件都必须满足：
   - `actors` 长度为 2 且不同
   - `trace` 至少包含 `action_completed.event_id`
   - `delta` 方向正确（boom：food>0；dispute：conflict>0 且 trust<0）

### 2) 回归

- `go test ./...` 全绿
- 现有 staging smoke（`scripts/smoke_staging.sh`）继续通过

