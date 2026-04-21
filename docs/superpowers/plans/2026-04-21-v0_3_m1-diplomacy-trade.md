# v0.3-M1（外交/贸易剧本）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 2 条最小“意图→事件剧本”（外交/结盟 + 贸易/经济），在不破坏决定论与既有 API 行为的前提下，让 spectator/entity 与 replay/highlight 有更强叙事素材。

**Architecture:** 写侧（sim）在 `action_completed` 之后追加 0~1 条 world 事件（`alliance_formed/treaty_signed` 或 `trade_agreement`），事件 actors 为确定性挑选的两国，delta 直接作用于世界状态，并带 trace 回溯到意图执行链条。读侧（spectator projection）无需改动即可自动产出关系网变化与原因。

**Tech Stack:** Go（现有 sim/gateway/tests），HTTP 集成测试（httptest），NDJSON export。

---

## 0) Files 结构与改动范围（先锁定）

**新增：**
- `internal/sim/intent_story_rules.go`：外交/贸易剧本规则 + 决定论 actor 选择
- `tests/integration/intent_story_rules_test.go`：集成测试门禁（外交/贸易事件 + trace/delta/顺序）

**修改：**
- `internal/sim/world.go`：在 `action_completed` 后追加剧本事件并 ApplyDelta
- `docs/roadmap.md`：阶段 3 的“更丰富规则/剧本”进度（小步更新）

---

## Task 1: 写 failing 集成测试（RED → 确认失败）

**Files:**
- Create: `tests/integration/intent_story_rules_test.go`

- [ ] **Step 1: 写 failing test（外交 goal 会产出 alliance/treaty；贸易 goal 会产出 trade）**

```go
package integration

import (
  "bufio"
  "bytes"
  "encoding/json"
  "io"
  "net/http"
  "net/http/httptest"
  "testing"
  "time"

  "lobster-world-core/internal/events/spec"
  "lobster-world-core/internal/gateway"
)

type exportEvent struct {
  spec.Event
  ExportSchemaVersion int `json:"export_schema_version"`
}

func readExport(t *testing.T, baseURL, worldID string) []exportEvent {
  t.Helper()
  resp, err := http.Get(baseURL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
  if err != nil {
    t.Fatalf("GET export: %v", err)
  }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK {
    b, _ := io.ReadAll(resp.Body)
    t.Fatalf("export status=%d body=%q", resp.StatusCode, string(b))
  }
  sc := bufio.NewScanner(resp.Body)
  out := []exportEvent{}
  for sc.Scan() {
    line := sc.Bytes()
    if len(bytes.TrimSpace(line)) == 0 {
      continue
    }
    var ev exportEvent
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
  if err := sc.Err(); err != nil {
    t.Fatalf("scan: %v", err)
  }
  return out
}

func findByType(evs []exportEvent, typ string) *exportEvent {
  for i := range evs {
    if evs[i].Type == typ {
      return &evs[i]
    }
  }
  return nil
}

func findActionCompleted(evs []exportEvent) *exportEvent {
  for i := range evs {
    if evs[i].Type == "action_completed" {
      return &evs[i]
    }
  }
  return nil
}

func hasTraceCause(e exportEvent, causeEventID string) bool {
  for _, tl := range e.Trace {
    if tl.CauseEventID == causeEventID {
      return true
    }
  }
  return false
}

func num(m map[string]any, k string) (int64, bool) {
  v, ok := m[k]
  if !ok || v == nil {
    return 0, false
  }
  switch x := v.(type) {
  case float64:
    return int64(x), true
  case int64:
    return x, true
  case int:
    return int64(x), true
  default:
    return 0, false
  }
}

func TestIntentStoryRules_Diplomacy_EmitsAllianceFormed(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{
    TickInterval: 10 * time.Millisecond,
    Seed:         123,
  })
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_story_diplomacy"
  body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "发起结盟"})
  resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
  if err != nil {
    t.Fatalf("POST /intents: %v", err)
  }
  resp.Body.Close()
  time.Sleep(120 * time.Millisecond)

  evs := readExport(t, s.URL, worldID)
  ac := findActionCompleted(evs)
  if ac == nil {
    t.Fatalf("expected action_completed in export")
  }
  al := findByType(evs, "alliance_formed")
  if al == nil {
    t.Fatalf("expected alliance_formed in export")
  }
  if al.Ts <= ac.Ts {
    t.Fatalf("expected alliance_formed after action_completed (ts), got ac.ts=%d al.ts=%d", ac.Ts, al.Ts)
  }
  if len(al.Actors) != 2 || al.Actors[0] == al.Actors[1] {
    t.Fatalf("expected 2 distinct actors, got %#v", al.Actors)
  }
  if _, ok := num(al.Delta, "trust"); !ok {
    t.Fatalf("expected delta.trust in alliance_formed, got %#v", al.Delta)
  }
  if !hasTraceCause(*al, ac.EventID) {
    t.Fatalf("expected alliance_formed trace links to action_completed %q", ac.EventID)
  }
}

func TestIntentStoryRules_Trade_EmitsTradeAgreement(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{
    TickInterval: 10 * time.Millisecond,
    Seed:         123,
  })
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_story_trade"
  body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "组织集市交换物资"})
  resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
  if err != nil {
    t.Fatalf("POST /intents: %v", err)
  }
  resp.Body.Close()
  time.Sleep(120 * time.Millisecond)

  evs := readExport(t, s.URL, worldID)
  ac := findActionCompleted(evs)
  if ac == nil {
    t.Fatalf("expected action_completed in export")
  }
  tr := findByType(evs, "trade_agreement")
  if tr == nil {
    t.Fatalf("expected trade_agreement in export")
  }
  if tr.Ts <= ac.Ts {
    t.Fatalf("expected trade_agreement after action_completed (ts), got ac.ts=%d tr.ts=%d", ac.Ts, tr.Ts)
  }
  if len(tr.Actors) != 2 || tr.Actors[0] == tr.Actors[1] {
    t.Fatalf("expected 2 distinct actors, got %#v", tr.Actors)
  }
  if _, ok := num(tr.Delta, "food"); !ok {
    t.Fatalf("expected delta.food in trade_agreement, got %#v", tr.Delta)
  }
  if !hasTraceCause(*tr, ac.EventID) {
    t.Fatalf("expected trade_agreement trace links to action_completed %q", ac.EventID)
  }
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**

Run:
```bash
go test ./... -run TestIntentStoryRules_ -v
```

Expected:
- FAIL（找不到 `alliance_formed` / `trade_agreement` 事件）

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/intent_story_rules_test.go
git commit -m "test(v0.3-m1): require diplomacy/trade story events"
```

---

## Task 2: 实现“决定论 actor 选择 + 剧本事件生成”（GREEN）

**Files:**
- Create: `internal/sim/intent_story_rules.go`
- Modify: `internal/sim/world.go`

- [ ] **Step 1: 新增 intent_story_rules.go**

创建文件 `internal/sim/intent_story_rules.go`：

```go
package sim

import (
  "fmt"
  "hash/fnv"
  "math/rand"
  "strings"

  "lobster-world-core/internal/events/spec"
)

var defaultStoryActorPool = []string{"nation_a", "nation_b", "nation_c", "nation_d", "nation_e", "nation_f"}

func seedForIntent(worldSeed int64, tick int64, intentID string) int64 {
  h := fnv.New64a()
  _, _ = h.Write([]byte(fmt.Sprintf("%d|%d|%s", worldSeed, tick, intentID)))
  return int64(h.Sum64())
}

func pick2DistinctForIntent(worldSeed int64, tick int64, intentID string, pool []string) (string, string, bool) {
  if len(pool) < 2 {
    return "", "", false
  }
  r := rand.New(rand.NewSource(seedForIntent(worldSeed, tick, intentID)))
  i := r.Intn(len(pool))
  j := r.Intn(len(pool) - 1)
  if j >= i {
    j++
  }
  return pool[i], pool[j], true
}

type storyEventSpec struct {
  typ       string
  narrative string
  delta     map[string]any
}

func intentStorySpec(goal string) (storyEventSpec, bool) {
  g := strings.TrimSpace(goal)
  if g == "" {
    return storyEventSpec{}, false
  }
  // Precedence: diplomacy > trade (avoids multiple extra events per intent in v0.3-M1).
  if strings.Contains(g, "结盟") || strings.Contains(g, "联盟") {
    return storyEventSpec{
      typ: "alliance_formed",
      narrative: "外交突破：达成同盟（目标：" + g + "）",
      delta: map[string]any{"trust": int64(8), "order": int64(2), "conflict": int64(-3)},
    }, true
  }
  if strings.Contains(g, "条约") || strings.Contains(g, "停战") || strings.Contains(g, "谈判") {
    return storyEventSpec{
      typ: "treaty_signed",
      narrative: "外交突破：签署条约（目标：" + g + "）",
      delta: map[string]any{"trust": int64(6), "order": int64(2), "conflict": int64(-2)},
    }, true
  }
  if strings.Contains(g, "贸易") || strings.Contains(g, "集市") || strings.Contains(g, "交换") || strings.Contains(g, "商路") {
    return storyEventSpec{
      typ: "trade_agreement",
      narrative: "贸易达成：开通商路（目标：" + g + "）",
      delta: map[string]any{"food": int64(5), "trust": int64(3), "knowledge": int64(1), "conflict": int64(-1)},
    }, true
  }
  return storyEventSpec{}, false
}

func buildStoryWorldEvent(worldID string, tick int64, worldSeed int64, intentID string, goal string, acceptedEventID string, actionCompletedEventID string) (spec.Event, bool) {
  sp, ok := intentStorySpec(goal)
  if !ok {
    return spec.Event{}, false
  }
  a, b, ok := pick2DistinctForIntent(worldSeed, tick, intentID, defaultStoryActorPool)
  if !ok {
    return spec.Event{}, false
  }
  ev := spec.Event{
    SchemaVersion: 1,
    EventID:       "", // filled by caller via w.newEventLocked
    Ts:            0,  // filled by caller
    WorldID:       worldID,
    Scope:         "world",
    Type:          sp.typ,
    Actors:        []string{a, b},
    Narrative:     sp.narrative,
    Tick:          tick,
    Delta:         sp.delta,
    Trace: []spec.TraceLink{
      {CauseEventID: actionCompletedEventID, Note: "从意图执行结果导出剧本事件"},
    },
  }
  if acceptedEventID != "" {
    ev.Trace = append(ev.Trace, spec.TraceLink{CauseEventID: acceptedEventID, Note: "回溯意图来源：" + strings.TrimSpace(goal)})
  }
  return ev, true
}
```

- [ ] **Step 2: 在 world.go 中调用（仅在 action_completed 成功后追加）**

在 `internal/sim/world.go` 的 `action_completed` 持久化成功后追加（保持事件顺序不变）：

```go
if err := w.appendAndPublish(done); err != nil {
  log.Printf("sim: failed to persist action_completed world=%s err=%v", w.worldID, err)
  return
}

if extra, ok := buildStoryWorldEvent(w.worldID, w.tick, w.seed, qi.ID, qi.Intent.Goal, qi.AcceptedEventID, done.EventID); ok {
  // Fill canonical fields deterministically using existing world helpers.
  ev := w.newEventLocked(extra.Type, extra.Actors, extra.Narrative)
  ev.Delta = extra.Delta
  ev.Trace = extra.Trace
  w.state.ApplyDelta(ev.Delta)
  if err := w.appendAndPublish(ev); err != nil {
    log.Printf("sim: failed to persist story event world=%s type=%s err=%v", w.worldID, ev.Type, err)
  }
}
```

- [ ] **Step 3: 运行测试确认通过（GREEN）**

Run:
```bash
go test ./... -run TestIntentStoryRules_ -v
go test ./...
```

Expected:
- 全绿

- [ ] **Step 4: Commit**
```bash
git add internal/sim/intent_story_rules.go internal/sim/world.go
git commit -m "feat(v0.3-m1): emit diplomacy/trade story events after intent execution"
```

---

## Task 3: 文档同步（防漂移）

**Files:**
- Modify: `docs/roadmap.md`

- [ ] **Step 1: 更新 roadmap 阶段 3**
在“阶段 3（世界活起来）”中，把“更丰富规则/剧本”项更新为：已新增外交/贸易两条最小剧本（并保留后续扩展项）。

- [ ] **Step 2: 运行全量测试**
```bash
go test ./...
```

- [ ] **Step 3: Commit**
```bash
git add docs/roadmap.md
git commit -m "docs: note v0.3-M1 diplomacy/trade story rules"
```

