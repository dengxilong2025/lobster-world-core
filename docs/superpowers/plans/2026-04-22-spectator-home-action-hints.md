# spectator/home 行动建议增强 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 强化 spectator/home 的 `world.summary` 行动建议：输出主/次两条可执行建议，包含可触发关键词与预期事件类型（trade_agreement / alliance_formed / treaty_signed），不改 API。

**Architecture:** 只修改 `internal/gateway/world_summary.go` 的建议生成：基于 state 风险优先级生成主建议，基于 stage+recent 生成次建议；保持决定论（仅依赖输入 st+recent），并通过单元测试锁定关键词与事件字符串。

**Tech Stack:** Go（gateway），单元测试（`internal/gateway/world_summary_test.go`）。

---

## 0) 文件结构与改动范围（锁定）

**修改：**
- `internal/gateway/world_summary.go`
- `internal/gateway/world_summary_test.go`
- `docs/roadmap.md`

**参考：**
- `docs/superpowers/specs/2026-04-22-spectator-home-action-hints-design.md`

---

## Task 1: 写 failing tests（RED）

**Files:**
- Modify: `internal/gateway/world_summary_test.go`

- [ ] **Step 1: 增加饥荒建议门禁（必须包含贸易关键词 + trade_agreement）**

在 `world_summary_test.go` 末尾新增：

```go
func TestDeriveWorldSummary_ActionHint_FamineMentionsTradeAndExpectedEvent(t *testing.T) {
	t.Parallel()

	ws := deriveWorldSummary(sim.Status{
		Tick:  10,
		State: sim.WorldState{Food: 15, Population: 120, Order: 50, Trust: 50, Knowledge: 10, Conflict: 10},
	}, []string{"粮仓见底"})

	found := false
	for _, b := range ws.Summary {
		if !strings.HasPrefix(b, "建议：") {
			continue
		}
		// keyword + expected event type
		if (strings.Contains(b, "贸易") || strings.Contains(b, "集市") || strings.Contains(b, "交换") || strings.Contains(b, "商路")) &&
			strings.Contains(b, "trade_agreement") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected famine hint to mention trade keywords and trade_agreement, got summary=%v", ws.Summary)
	}
}
```

- [ ] **Step 2: 增加战乱建议门禁（必须包含外交关键词 + alliance_formed/treaty_signed）**

继续新增：

```go
func TestDeriveWorldSummary_ActionHint_WarMentionsDiplomacyAndExpectedEvent(t *testing.T) {
	t.Parallel()

	ws := deriveWorldSummary(sim.Status{
		Tick:  10,
		State: sim.WorldState{Food: 50, Population: 120, Order: 50, Trust: 40, Knowledge: 10, Conflict: 65},
	}, []string{"边境冲突升级"})

	found := false
	for _, b := range ws.Summary {
		if !strings.HasPrefix(b, "建议：") {
			continue
		}
		if (strings.Contains(b, "停战") || strings.Contains(b, "谈判") || strings.Contains(b, "条约") || strings.Contains(b, "结盟")) &&
			(strings.Contains(b, "alliance_formed") || strings.Contains(b, "treaty_signed")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected war hint to mention diplomacy keywords and alliance_formed/treaty_signed, got summary=%v", ws.Summary)
	}
}
```

- [ ] **Step 3: 运行测试确认失败（RED）**

Run:
```bash
go test ./internal/gateway -run TestDeriveWorldSummary_ActionHint_ -v
```

Expected:
- FAIL（当前建议不一定包含 `trade_agreement` / `alliance_formed|treaty_signed` 字符串）

- [ ] **Step 4: Commit（仅测试）**
```bash
git add internal/gateway/world_summary_test.go
git commit -m "test(spectator): require actionable hints mention expected story events"
```

---

## Task 2: 最小实现让测试转绿（GREEN）

**Files:**
- Modify: `internal/gateway/world_summary.go`

- [ ] **Step 1: 将“建议”拆成主/次两条，并在饥荒/战乱主建议中补充预期事件类型**

在 `deriveWorldSummary` 中：
1) 把 `actionHint(stage)` 改为 `primaryHint(st, stage, recent)` + `secondaryHint(st, stage, recent)`（两个 string）
2) 主建议按优先级：food→conflict→trust→order→default
3) 当走到贸易/外交剧本建议时，文案中必须包含：
   - 贸易：`trade_agreement`
   - 外交：`alliance_formed` 或 `treaty_signed`

示例实现（保持简洁，不要过度抽象）：

```go
primary := "建议：提交一个“探索/贸易/合作”意图，推动世界叙事进入下一节点"
if st.State.Food <= 20 {
	primary = "建议（优先补给）：提交“贸易/集市/交换/商路”意图，观察是否触发 trade_agreement 并提升 food/trust"
} else if st.State.Conflict >= 60 {
	primary = "建议（优先降冲突）：提交“停战/谈判/条约/结盟”意图，观察 treaty_signed / alliance_formed 是否出现并降低 conflict"
} else if st.State.Trust <= 25 {
	primary = "建议（优先修复信任）：提交“结盟/联盟/合作/互助”意图，观察 alliance_formed（或 trade_agreement）是否出现并抬升 trust"
} else if st.State.Order <= 20 {
	primary = "建议（优先稳秩序）：提交“整顿/裁决/执法/议会”意图，观察 order 是否回升并避免连锁崩溃"
}

secondary := ""
joined := strings.Join(recent, "；")
if stage == "战乱" || strings.Contains(joined, "背叛") || strings.Contains(joined, "翻脸") {
	secondary = "建议（备选外交）：提交“结盟/谈判/条约”意图，观察 alliance_formed / treaty_signed 是否出现并改变关系走向"
} else if stage == "饥荒" {
	secondary = "建议（备选补给）：提交“补给/狩猎”意图，观察 food 回升并避免秩序塌陷"
}
if secondary != "" {
	bullets = append(bullets, primary, secondary)
} else {
	bullets = append(bullets, primary)
}
```

4) 保持去重逻辑有效（已有 dedupe）；并保留“避免 hook 与 hint 完全重复”的保护。

- [ ] **Step 2: 运行测试确认通过（GREEN）**

Run:
```bash
go test ./internal/gateway -run TestDeriveWorldSummary_ActionHint_ -v
go test ./...
```

Expected:
- PASS

- [ ] **Step 3: Commit**
```bash
git add internal/gateway/world_summary.go
git commit -m "feat(spectator): add trade/diplomacy-focused actionable hints"
```

---

## Task 3: 文档同步（roadmap）

**Files:**
- Modify: `docs/roadmap.md`

- [ ] **Step 1: 更新阶段 3 的 spectator 摘要项**
把“spectator 首页‘世界阶段/状态摘要’（更强的解说引导）”拆解/标记：行动建议优先已完成。

- [ ] **Step 2: 复跑全量测试**
```bash
go test ./...
```

- [ ] **Step 3: Commit**
```bash
git add docs/roadmap.md
git commit -m "docs: mark spectator action-hints enhancement complete"
```

---

## Task 4: 交付补丁包（便于审阅/合并）

- [ ] **Step 1: 生成最小提交集 patch zip**
Run:
```bash
git format-patch -3 -o /workspace/patches_spectator_action_hints
cd /workspace && zip -qr patches_spectator_action_hints.zip patches_spectator_action_hints
```

- [ ] **Step 2: 输出复盘摘要**
包含：问题→方案→门禁→变更点→下一步。

