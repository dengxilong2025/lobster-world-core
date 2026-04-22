# spectator/home 行动建议增强 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改 API 的前提下，让 `/api/v0/spectator/home` 返回的 `world.summary` 中“建议：”更可执行：包含可触发关键词，并明确预期事件类型（`trade_agreement / alliance_formed / treaty_signed`）。

**Architecture:** 仅修改 `internal/gateway/world_summary.go` 的建议生成逻辑（主建议按风险优先级、次建议按 stage/recent 引导），并用 `internal/gateway/world_summary_test.go` 做门禁。保持输出结构不变（`stage + summary[] + state{...}`），且不引入随机/时间（决定论）。

**Tech Stack:** Go（gateway/sim），标准 `testing` 包。

---

## 0) Files 结构与改动范围（先锁定）

**修改：**
- `internal/gateway/world_summary.go`：增强建议生成逻辑（两条建议、关键词与预期事件）
- `internal/gateway/world_summary_test.go`：新增/加强门禁用例（饥荒/战乱）
- `docs/roadmap.md`：阶段 3 “spectator 首页…摘要” 进度对齐（可选但推荐）

---

## Task 1: 写 failing tests（RED → 确认失败）

> 注意：按 TDD 铁律，先让测试失败，确认确实在测“新行为”。

**Files:**
- Modify: `internal/gateway/world_summary_test.go`

- [ ] **Step 1: 新增饥荒场景门禁（必须包含贸易关键词 + trade_agreement）**

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

- [ ] **Step 2: 新增战乱场景门禁（必须包含外交关键词 + alliance_formed/treaty_signed）**

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
go test ./... -run TestDeriveWorldSummary_ActionHint_ -v
```

Expected:
- FAIL（提示 summary 中不包含关键词或预期事件字符串）

- [ ] **Step 4: Commit（仅测试）**

```bash
git add internal/gateway/world_summary_test.go
git commit -m "test(spectator): require actionable hints mention story events"
```

---

## Task 2: 最小实现让测试转绿（GREEN）

**Files:**
- Modify: `internal/gateway/world_summary.go`

- [ ] **Step 1: 实现“主建议（风险优先级）”**

在 `deriveWorldSummary` 中新增/替换 action hints 逻辑：
- `Food<=20`：建议包含 `贸易/集市/交换/商路` 且提到 `trade_agreement`
- `Conflict>=60`：建议包含 `停战/谈判/条约/结盟` 且提到 `treaty_signed / alliance_formed`
- `Trust<=25`：建议包含 `结盟/联盟/合作/互助` 且提到 `alliance_formed`（可兼容 `trade_agreement`）
- `Order<=20`：建议包含 `整顿/裁决/执法/议会`（预期事件不强制，但文案需明确“观察 order 回升”）

- [ ] **Step 2: 实现“次建议（stage/recent 引导）”**

规则：
- 若 `stage==战乱` 或 `recent` 含 `背叛/翻脸`：给一条备选外交建议（并提到 `alliance_formed/treaty_signed`）
- 若 `stage==饥荒`：给一条备选补给建议（狩猎/补给），避免与主建议完全重复
- 其它 stage：次建议可为空（保持简洁）

- [ ] **Step 3: 保持原门禁不回归**

要求：
- 至少存在一条 `建议：` 且包含 `提交`
- `建议：` 不与 `看点：` 文案完全重复
- `summary` 去重逻辑仍然有效

- [ ] **Step 4: 运行测试确认通过（GREEN）**

Run:
```bash
go test ./... -run TestDeriveWorldSummary_ActionHint_ -v
go test ./...
```

Expected:
- 全绿

- [ ] **Step 5: Commit（实现）**

```bash
git add internal/gateway/world_summary.go
git commit -m "feat(spectator): enrich home action hints with story keywords/events"
```

---

## Task 3: 文档同步与交付打包

**Files:**
- Modify: `docs/roadmap.md`

- [ ] **Step 1: 更新 roadmap（阶段 3）**

将“spectator 首页…摘要（更强解说引导）”拆分或标记：
- 行动建议优先：已完成

- [ ] **Step 2: 复跑全量测试**
```bash
go test ./...
```

- [ ] **Step 3: Commit（文档同步）**
```bash
git add docs/roadmap.md
git commit -m "docs: note spectator home actionable hints enhancement"
```

- [ ] **Step 4: 打补丁包（最小提交集）**
```bash
git format-patch -3 -o /workspace/patches_spectator_action_hints
cd /workspace && zip -qr patches_spectator_action_hints.zip patches_spectator_action_hints
```

