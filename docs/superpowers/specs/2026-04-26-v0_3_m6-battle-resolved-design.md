# v0.3-M6：战争后续（battle_resolved）Design（2026-04-26）

## Goal

补齐战争线的“后续动作”，让玩家在触发 `war_started` 之后还能通过进攻/突袭等意图继续推进剧情，并生成可回放、可门禁的事件：

- 新增 1 个剧情事件：`battle_resolved`

并确保：
- `/api/v0/replay/export` 可稳定看到该事件（门禁）
- `/api/v0/spectator/home` 建议中点名可触发关键词与预期事件类型
- 决定论不被破坏（actors 仍由 seed/tick/intent_id 推导）

---

## Non-goals

- 不做真正的“战争状态机”（持久战争、战线、兵力等）
- 不做多回合战斗结算或自动触发链
- 不引入新 API 或新存储

---

## 现状与约束

- 写侧（sim）已支持在 `action_completed` 之后追加 0~1 条 world scope 剧本事件（v0.3 已实现多个事件类型）。
- spectator projection 已对 `battle_resolved` 做关系/热度的利用（现有代码已考虑该类型），因此新增事件会立刻增强观战叙事。
- 规则层目前是关键词触发，不引入额外状态更容易保持稳定与测试可控。

---

## 设计

### 1) 新增 intent → story 规则：battle_resolved

在 `internal/sim/intent_story_rules.go` 的 `intentStorySpec(goal string)` 中新增：

**触发关键词（goal 包含任一）**
- `进攻`、`突袭`、`战斗`、`会战`

**产出事件**
- `Type: "battle_resolved"`
- `Scope: "world"`
- `Actors: [nation_x, nation_y]`（2 个不同 actors，沿用既有 pool + pick2DistinctForIntent）

**Delta（建议值，门禁将锁定）**
- `conflict: +6`
- `order: -2`
- `trust: -2`
- `food: -2`

（解释：战斗会短期消耗资源并推高冲突，同时侵蚀秩序与信任；数值保持温和，避免过快跑飞。）

**Narrative（示例）**
- `战斗结算：一场会战尘埃落定（目标：<goal>）`

**Trace**
- 回溯到 `action_completed`（必带）
- 如可得再回溯 `intent_accepted`

---

### 2) 优先级

保持“每个 intent 最多追加 1 条剧本事件”，并把 `battle_resolved` 放在战争线中：

`betrayal / war_started`  
→ `battle_resolved`  
→ `trade_dispute / market_boom`  
→ `alliance_formed / treaty_signed`  
→ `trade_agreement`

理由：
- 若用户 goal 同时包含“宣战+进攻”，优先 `war_started`（开战）
- 若用户明确写“进攻/会战”，则落在 `battle_resolved`

---

### 3) spectator/home 可执行建议增强（最小）

在 `internal/gateway/world_summary.go` 的冲突/战乱语境建议中补一句：
- 可触发关键词：`进攻/突袭/战斗/会战`
- 预期事件：`battle_resolved`

让玩家在 war_started 之后能得到下一步“可执行动作”提示。

---

## 测试与验收

### 1) 集成测试门禁（新增）

新增测试覆盖：
1) 提交 `goal="进攻：发动会战"` 后，export 中必须出现 `"type":"battle_resolved"`
2) 事件必须满足：
   - `actors` 长度为 2 且不同
   - `trace` 至少包含某个 `action_completed.event_id`
   - `delta.conflict > 0`
3) home hints（轻量门禁）：
   - `/api/v0/spectator/home` 文本应包含 `battle_resolved`（或至少包含关键词提示）

### 2) 回归

- `go test ./...` 全绿
- staging smoke 继续通过

