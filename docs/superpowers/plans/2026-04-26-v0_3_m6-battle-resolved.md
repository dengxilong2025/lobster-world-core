# v0.3-M6：战争后续（battle_resolved）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增剧情事件 `battle_resolved`（由 intent 关键词触发，在 `action_completed` 之后追加 0~1 条剧本事件），并让 spectator/home 提供可执行提示（关键词 + 预期事件类型）。提供集成测试门禁：export 必出现该事件，actors/trace/delta 方向正确。

**Architecture:** 扩展 `internal/sim/intent_story_rules.go` 的关键词规则与优先级（war_started/betrayal > battle_resolved > trade deepen > diplomacy+ > trade_agreement），继续使用决定论 actor 选择 `pick2DistinctForIntent`，保持“每 intent 最多 1 条额外事件”约束。gateway 的 `world_summary.go` 仅补文案提示（不改 API）。测试通过 httptest 驱动 intents→export→home 黑盒验证。

**Tech Stack:** Go（sim/gateway/tests），integration tests（net/http/httptest），NDJSON export。

---

## 0) Files（锁定）

**Modify:**
- `internal/sim/intent_story_rules.go`（新增 battle_resolved + 调整优先级）
- `internal/gateway/world_summary.go`（战乱/冲突语境提示 battle_resolved）

**Create:**
- `tests/integration/intent_story_rules_battle_resolved_test.go`（集成门禁）

---

## Task 1: TDD — 新增集成门禁（RED）

**Files:**
- Create: `tests/integration/intent_story_rules_battle_resolved_test.go`

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

type exportEventBR struct {
  spec.Event
  ExportSchemaVersion int `json:"export_schema_version"`
}

func readExportBR(t *testing.T, baseURL, worldID string) []exportEventBR {
  t.Helper()
  resp, err := http.Get(baseURL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
  if err != nil { t.Fatalf("GET export: %v", err) }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK {
    b, _ := io.ReadAll(resp.Body)
    t.Fatalf("export status=%d body=%q", resp.StatusCode, string(b))
  }
  sc := bufio.NewScanner(resp.Body)
  out := []exportEventBR{}
  for sc.Scan() {
    line := sc.Bytes()
    if len(bytes.TrimSpace(line)) == 0 { continue }
    var ev exportEventBR
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

func findByTypeBR(evs []exportEventBR, typ string) *exportEventBR {
  for i := range evs {
    if evs[i].Type == typ { return &evs[i] }
  }
  return nil
}

func findActionCompletedIDsBR(evs []exportEventBR) map[string]struct{} {
  ids := map[string]struct{}{}
  for i := range evs {
    if evs[i].Type == "action_completed" {
      ids[evs[i].EventID] = struct{}{}
    }
  }
  return ids
}

func hasTraceCauseInSetBR(e exportEventBR, ids map[string]struct{}) bool {
  for _, tl := range e.Trace {
    if _, ok := ids[tl.CauseEventID]; ok { return true }
  }
  return false
}

func numBR(m map[string]any, k string) (int64, bool) {
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

func TestStoryRules_BattleResolved(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{
    TickInterval: 5 * time.Millisecond,
    Seed:         2468,
  })
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_story_battle_resolved"

  b, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "进攻：发动会战"})
  r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(b))
  if err != nil { t.Fatalf("POST /intents: %v", err) }
  _ = r.Body.Close()
  if r.StatusCode != http.StatusOK { t.Fatalf("intent status=%d", r.StatusCode) }

  time.Sleep(300 * time.Millisecond)

  evs := readExportBR(t, s.URL, worldID)
  acIDs := findActionCompletedIDsBR(evs)
  if len(acIDs) == 0 { t.Fatalf("missing action_completed in export") }

  br := findByTypeBR(evs, "battle_resolved")
  if br == nil { t.Fatalf("missing battle_resolved in export") }
  if len(br.Actors) != 2 || br.Actors[0] == br.Actors[1] { t.Fatalf("battle_resolved actors invalid: %v", br.Actors) }
  if !hasTraceCauseInSetBR(*br, acIDs) { t.Fatalf("battle_resolved should trace some action_completed") }
  if v, ok := numBR(br.Delta, "conflict"); !ok || v <= 0 { t.Fatalf("battle_resolved expected conflict>0 delta=%v", br.Delta) }

  hr, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=" + worldID)
  if err != nil { t.Fatalf("GET home: %v", err) }
  defer hr.Body.Close()
  hb, _ := io.ReadAll(hr.Body)
  hs := string(hb)
  if !strings.Contains(hs, "建议：") { t.Fatalf("expected hints in home, got=%s", hs) }
  if !strings.Contains(hs, "battle_resolved") {
    t.Fatalf("expected home hints mention battle_resolved, got=%s", hs)
  }
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run TestStoryRules_BattleResolved -v
```

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/intent_story_rules_battle_resolved_test.go
git commit -m "test(story): gate battle_resolved"
```

---

## Task 2: 实现 battle_resolved 规则（GREEN）

**Files:**
- Modify: `internal/sim/intent_story_rules.go`

- [ ] **Step 1: 新增关键词规则（并插入到正确优先级位置）**

在 `intentStorySpec(goal string)` 中加入（位置：war_started/betrayal 之后、trade_deepen 之前）：

```go
if strings.Contains(g, "进攻") || strings.Contains(g, "突袭") || strings.Contains(g, "战斗") || strings.Contains(g, "会战") {
  return storyEventSpec{
    typ:       "battle_resolved",
    narrative: "战斗结算：一场会战尘埃落定（目标：" + g + "）",
    delta:     map[string]any{"conflict": int64(6), "order": int64(-2), "trust": int64(-2), "food": int64(-2)},
  }, true
}
```

- [ ] **Step 2: 跑测试转绿**
```bash
go test ./tests/integration -run TestStoryRules_BattleResolved -v
go test ./...
```

- [ ] **Step 3: Commit（规则实现）**
```bash
git add internal/sim/intent_story_rules.go
git commit -m "feat(story): add battle_resolved"
```

---

## Task 3: spectator/home 文案增强（GREEN）

**Files:**
- Modify: `internal/gateway/world_summary.go`

- [ ] **Step 1: 在战乱/冲突语境下补充 battle_resolved 提示**

建议在 conflict 分支（或 stage==战乱 / recent 含背叛）现有建议上追加一句（必须包含字符串 `battle_resolved`）：
- `也可尝试“进攻/突袭/战斗/会战”（battle_resolved），观察冲突升级与后续连锁事件`

- [ ] **Step 2: 全量测试**
```bash
go test ./...
```

- [ ] **Step 3: Commit（home hints）**
```bash
git add internal/gateway/world_summary.go
git commit -m "feat(spectator): mention battle_resolved in home hints"
```

---

## Task 4: 部署与验收（Render）

- [ ] **Step 1: push main**
```bash
git push origin main
```

- [ ] **Step 2: staging 快速验收（手动）**
```bash
URL=https://lobster-world-core.onrender.com
WORLD_ID=v03m6_$(date +%H%M%S)
curl -sS -H "Content-Type: application/json" -X POST "$URL/api/v0/intents" --data "{\"world_id\":\"$WORLD_ID\",\"goal\":\"进攻：发动会战\"}"
sleep 12
curl -sS "$URL/api/v0/replay/export?world_id=$WORLD_ID&limit=5000" | grep -E '\"type\":\"battle_resolved\"' | head -n 5
curl -sS "$URL/api/v0/spectator/home?world_id=$WORLD_ID" | head -c 600; echo
```
Expected: export 中出现 `battle_resolved`；home hints 提到 `battle_resolved`。

