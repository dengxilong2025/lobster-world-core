# v0.3-M4：贸易深化（繁荣 + 纠纷）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增两类贸易剧情事件 `market_boom` 与 `trade_dispute`（由 intent 关键词触发，追加到 `action_completed` 之后），并增强 spectator/home 的可执行建议（点名可触发关键词与预期事件类型）。提供集成测试门禁确保 export 可回放、trace/delta 方向可验证、决定论不被破坏。

**Architecture:** 扩展 `internal/sim/intent_story_rules.go` 的关键词规则，保持“每个 intent 最多追加 1 条剧本事件”的约束，并更新优先级（betrayal/war > dispute/boom > diplomacy positive > trade_agreement）。gateway 的 `world_summary.go` 仅更新建议文案（不改 API 结构）。测试以 httptest 驱动 intents→export→home 进行黑盒验证。

**Tech Stack:** Go（sim/gateway/tests），integration tests（net/http/httptest），NDJSON export。

---

## 0) Files（锁定）

**Modify:**
- `internal/sim/intent_story_rules.go`（新增 market_boom/trade_dispute + 调整优先级）
- `internal/gateway/world_summary.go`（贸易建议强化：market_boom/trade_dispute）

**Create:**
- `tests/integration/intent_story_rules_trade_deepen_test.go`（集成门禁）

---

## Task 1: TDD — 新增集成门禁（RED）

**Files:**
- Create: `tests/integration/intent_story_rules_trade_deepen_test.go`

- [ ] **Step 1: 写 failing test（先红）**

```go
package integration

import (
  "bufio"
  "bytes"
  "encoding/json"
  "io"
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"
  "time"

  "lobster-world-core/internal/events/spec"
  "lobster-world-core/internal/gateway"
)

type exportEventTD struct {
  spec.Event
  ExportSchemaVersion int `json:"export_schema_version"`
}

func readExportTD(t *testing.T, baseURL, worldID string) []exportEventTD {
  t.Helper()
  resp, err := http.Get(baseURL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
  if err != nil { t.Fatalf("GET export: %v", err) }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK {
    b, _ := io.ReadAll(resp.Body)
    t.Fatalf("export status=%d body=%q", resp.StatusCode, string(b))
  }
  sc := bufio.NewScanner(resp.Body)
  out := []exportEventTD{}
  for sc.Scan() {
    line := sc.Bytes()
    if len(bytes.TrimSpace(line)) == 0 { continue }
    var ev exportEventTD
    if err := json.Unmarshal(line, &ev); err != nil {
      t.Fatalf("unmarshal: %v line=%q", err, string(line))
    }
    if err := ev.Event.Validate(); err != nil {
      t.Fatalf("invalid event: %v ev=%#v", err, ev.Event)
    }
    if ev.ExportSchemaVersion <= 0 {
      t.Fatalf("expected export_schema_version>0")
    }
    out = append(out, ev)
  }
  if err := sc.Err(); err != nil { t.Fatalf("scan: %v", err) }
  return out
}

func findByTypeTD(evs []exportEventTD, typ string) *exportEventTD {
  for i := range evs {
    if evs[i].Type == typ { return &evs[i] }
  }
  return nil
}

func findActionCompletedIDsTD(evs []exportEventTD) map[string]struct{} {
  ids := map[string]struct{}{}
  for i := range evs {
    if evs[i].Type == "action_completed" {
      ids[evs[i].EventID] = struct{}{}
    }
  }
  return ids
}

func hasTraceCauseInSetTD(e exportEventTD, ids map[string]struct{}) bool {
  for _, tl := range e.Trace {
    if _, ok := ids[tl.CauseEventID]; ok { return true }
  }
  return false
}

func numTD(m map[string]any, k string) (int64, bool) {
  v, ok := m[k]
  if !ok || v == nil { return 0, false }
  switch x := v.(type) {
  case int64:
    return x, true
  case float64:
    return int64(x), true
  default:
    return 0, false
  }
}

func TestStoryRules_TradeDeepen_BoomAndDispute(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{
    TickInterval: 5 * time.Millisecond,
    Seed:         789,
  })
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_story_trade_deepen"
  postIntent := func(goal string) {
    b, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": goal})
    r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(b))
    if err != nil { t.Fatalf("POST /intents: %v", err) }
    _ = r.Body.Close()
    if r.StatusCode != http.StatusOK {
      t.Fatalf("intent status=%d goal=%q", r.StatusCode, goal)
    }
  }

  postIntent("开放贸易：市场繁荣")
  postIntent("封锁：加税关税")
  time.Sleep(400 * time.Millisecond)

  evs := readExportTD(t, s.URL, worldID)
  acIDs := findActionCompletedIDsTD(evs)
  if len(acIDs) == 0 { t.Fatalf("missing action_completed in export") }

  boom := findByTypeTD(evs, "market_boom")
  if boom == nil { t.Fatalf("missing market_boom in export") }
  if len(boom.Actors) != 2 || boom.Actors[0] == boom.Actors[1] { t.Fatalf("market_boom actors invalid: %v", boom.Actors) }
  if !hasTraceCauseInSetTD(*boom, acIDs) { t.Fatalf("market_boom should trace some action_completed") }
  if v, ok := numTD(boom.Delta, "food"); !ok || v <= 0 { t.Fatalf("market_boom expected food>0 delta=%v", boom.Delta) }

  dis := findByTypeTD(evs, "trade_dispute")
  if dis == nil { t.Fatalf("missing trade_dispute in export") }
  if len(dis.Actors) != 2 || dis.Actors[0] == dis.Actors[1] { t.Fatalf("trade_dispute actors invalid: %v", dis.Actors) }
  if !hasTraceCauseInSetTD(*dis, acIDs) { t.Fatalf("trade_dispute should trace some action_completed") }
  if v, ok := numTD(dis.Delta, "conflict"); !ok || v <= 0 { t.Fatalf("trade_dispute expected conflict>0 delta=%v", dis.Delta) }
  if v, ok := numTD(dis.Delta, "trust"); !ok || v >= 0 { t.Fatalf("trade_dispute expected trust<0 delta=%v", dis.Delta) }

  // Home hints should mention at least one of the expected trade deepen event types.
  hr, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=" + worldID)
  if err != nil { t.Fatalf("GET home: %v", err) }
  defer hr.Body.Close()
  hb, _ := io.ReadAll(hr.Body)
  hs := string(hb)
  if !strings.Contains(hs, "建议：") { t.Fatalf("expected hints in home, got=%s", hs) }
  if !(strings.Contains(hs, "market_boom") || strings.Contains(hs, "trade_dispute")) {
    t.Fatalf("expected home hints mention market_boom/trade_dispute, got=%s", hs)
  }
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run TestStoryRules_TradeDeepen_BoomAndDispute -v
```

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/intent_story_rules_trade_deepen_test.go
git commit -m "test(story): gate market_boom and trade_dispute"
```

---

## Task 2: 扩展 story 规则（GREEN）

**Files:**
- Modify: `internal/sim/intent_story_rules.go`

- [ ] **Step 1: 新增 trade_dispute / market_boom 关键词规则（并调整优先级）**

在 `intentStorySpec(goal string)` 中加入（位置：betrayal/war_started 之后，alliance/treaty 之前）：

```go
if strings.Contains(g, "封锁") || strings.Contains(g, "禁运") || strings.Contains(g, "加税") || strings.Contains(g, "关税") {
  return storyEventSpec{
    typ: "trade_dispute",
    narrative: "贸易纠纷：封锁与反制（目标：" + g + "）",
    delta: map[string]any{"food": int64(-3), "trust": int64(-6), "conflict": int64(4), "order": int64(-1)},
  }, true
}
if strings.Contains(g, "繁荣") || strings.Contains(g, "互市") || strings.Contains(g, "市场") || strings.Contains(g, "开放贸易") {
  return storyEventSpec{
    typ: "market_boom",
    narrative: "贸易繁荣：市场兴旺（目标：" + g + "）",
    delta: map[string]any{"food": int64(8), "knowledge": int64(2), "trust": int64(2), "conflict": int64(-1)},
  }, true
}
```

- [ ] **Step 2: 跑测试转绿**
```bash
go test ./tests/integration -run TestStoryRules_TradeDeepen_BoomAndDispute -v
go test ./...
```

- [ ] **Step 3: Commit（规则实现）**
```bash
git add internal/sim/intent_story_rules.go
git commit -m "feat(story): add market_boom and trade_dispute"
```

---

## Task 3: spectator/home 文案增强（GREEN）

**Files:**
- Modify: `internal/gateway/world_summary.go`

- [ ] **Step 1: 在低食物/饥荒提示中补充 market_boom/trade_dispute**

修改 `primaryHint()` 的 food 分支（或追加一条 secondary hint）使其包含：
- `market_boom`（繁荣/互市/开放贸易）
- `trade_dispute`（封锁/关税/禁运/加税）

建议文案（示例，具体可略微调整，但必须包含事件类型字符串）：
> `建议：优先补给——提交“贸易/集市/交换/商路”意图（trade_agreement）；也可尝试“繁荣/互市/开放贸易”（market_boom），或在施压语境下尝试“封锁/关税/禁运/加税”（trade_dispute）观察走向`

- [ ] **Step 2: 在 conflict 中段（40~59）增加一条“贸易施压”提示（可选但推荐）**

若实现不想改 stage 判定，可只在 `Conflict>=40 && <60` 时追加 secondary hint：
- `建议：贸易施压——提交“封锁/关税/禁运/加税”意图，观察 trade_dispute 是否出现并推高 conflict`

- [ ] **Step 3: 全量测试**
```bash
go test ./...
```

- [ ] **Step 4: Commit（文案增强）**
```bash
git add internal/gateway/world_summary.go
git commit -m "feat(spectator): mention market_boom/trade_dispute in home hints"
```

---

## Task 4: 部署与验收（Render）

- [ ] **Step 1: push main**
```bash
git push origin main
```

- [ ] **Step 2: staging 快速验收（脚本 + 手动）**
```bash
bash scripts/smoke_staging.sh
URL=https://lobster-world-core.onrender.com
WORLD_ID=v03m4_$(date +%H%M%S)
curl -sS -H "Content-Type: application/json" -X POST "$URL/api/v0/intents" --data "{\"world_id\":\"$WORLD_ID\",\"goal\":\"开放贸易：市场繁荣\"}"
curl -sS -H "Content-Type: application/json" -X POST "$URL/api/v0/intents" --data "{\"world_id\":\"$WORLD_ID\",\"goal\":\"封锁：加税关税\"}"
sleep 12
curl -sS "$URL/api/v0/replay/export?world_id=$WORLD_ID&limit=5000" | grep -E '\"type\":\"(market_boom|trade_dispute)\"' | head -n 10
```
Expected: export 中出现 `market_boom` 与 `trade_dispute`。

