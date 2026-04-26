# v0.3-M2：外交深化（背叛 + 宣战）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增两类“更有戏”的外交事件 `betrayal` 与 `war_started`（由 intent 关键词触发，追加到 `action_completed` 之后），并增强 spectator/home 建议文案的可执行性；提供集成测试门禁确保 export 可回放、trace/delta 可验证、决定论不被破坏。

**Architecture:** 扩展 `internal/sim/intent_story_rules.go` 的关键词规则（优先级：betrayal/war_started > alliance/treaty > trade）；在 sim 写侧（`world.go`）保持“每个 intent 最多追加 1 条剧本事件”的约束；gateway 的 `world_summary.go` 仅更新文案提示（不改 API 结构）。测试用 httptest 驱动 `/api/v0/intents` + `/spectator/home` + `/replay/export` 黑盒验证。

**Tech Stack:** Go（sim/gateway/tests），integration tests（net/http/httptest），NDJSON export。

---

## 0) Files（锁定）

**Modify:**
- `internal/sim/intent_story_rules.go`（新增 betrayal/war_started + 调整优先级）
- `internal/gateway/world_summary.go`（建议文案包含 betrayal/war_started）

**Create:**
- `tests/integration/intent_story_rules_betrayal_war_test.go`（集成门禁）

---

## Task 1: TDD — 新增集成门禁（RED）

**Files:**
- Create: `tests/integration/intent_story_rules_betrayal_war_test.go`

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

type exportEventBW struct {
  spec.Event
  ExportSchemaVersion int `json:"export_schema_version"`
}

func readExportBW(t *testing.T, baseURL, worldID string) []exportEventBW {
  t.Helper()
  resp, err := http.Get(baseURL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
  if err != nil { t.Fatalf("GET export: %v", err) }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK {
    b, _ := io.ReadAll(resp.Body)
    t.Fatalf("export status=%d body=%q", resp.StatusCode, string(b))
  }
  sc := bufio.NewScanner(resp.Body)
  out := []exportEventBW{}
  for sc.Scan() {
    line := sc.Bytes()
    if len(bytes.TrimSpace(line)) == 0 { continue }
    var ev exportEventBW
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

func findByTypeBW(evs []exportEventBW, typ string) *exportEventBW {
  for i := range evs {
    if evs[i].Type == typ { return &evs[i] }
  }
  return nil
}

func findActionCompletedBW(evs []exportEventBW) *exportEventBW {
  for i := range evs {
    if evs[i].Type == "action_completed" { return &evs[i] }
  }
  return nil
}

func hasTraceCauseBW(e exportEventBW, causeEventID string) bool {
  for _, tl := range e.Trace {
    if tl.CauseEventID == causeEventID { return true }
  }
  return false
}

func numBW(m map[string]any, k string) (int64, bool) {
  v, ok := m[k]
  if !ok || v == nil { return 0, false }
  switch x := v.(type) {
  case int64:
    return x, true
  case float64: // JSON numbers sometimes come as float64 in loose decoding
    return int64(x), true
  default:
    return 0, false
  }
}

func TestStoryRules_BetrayalAndWarStarted(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{
    TickInterval: 5 * time.Millisecond,
    Seed:         456,
  })
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_story_bw"

  postIntent := func(goal string) {
    b, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": goal})
    r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(b))
    if err != nil { t.Fatalf("POST /intents: %v", err) }
    _ = r.Body.Close()
    if r.StatusCode != http.StatusOK {
      t.Fatalf("intent status=%d goal=%q", r.StatusCode, goal)
    }
  }

  // Trigger betrayal and war.
  postIntent("背叛：翻脸")
  postIntent("宣战：开战")
  time.Sleep(350 * time.Millisecond)

  evs := readExportBW(t, s.URL, worldID)
  ac := findActionCompletedBW(evs)
  if ac == nil { t.Fatalf("missing action_completed in export") }

  be := findByTypeBW(evs, "betrayal")
  if be == nil { t.Fatalf("missing betrayal in export") }
  if len(be.Actors) != 2 || be.Actors[0] == be.Actors[1] { t.Fatalf("betrayal actors invalid: %v", be.Actors) }
  if !hasTraceCauseBW(*be, ac.EventID) { t.Fatalf("betrayal should trace action_completed %q", ac.EventID) }
  if v, ok := numBW(be.Delta, "trust"); !ok || v >= 0 { t.Fatalf("betrayal expected trust<0 delta=%v", be.Delta) }
  if v, ok := numBW(be.Delta, "conflict"); !ok || v <= 0 { t.Fatalf("betrayal expected conflict>0 delta=%v", be.Delta) }

  we := findByTypeBW(evs, "war_started")
  if we == nil { t.Fatalf("missing war_started in export") }
  if len(we.Actors) != 2 || we.Actors[0] == we.Actors[1] { t.Fatalf("war_started actors invalid: %v", we.Actors) }
  if !hasTraceCauseBW(*we, ac.EventID) { t.Fatalf("war_started should trace action_completed %q", ac.EventID) }
  if v, ok := numBW(we.Delta, "conflict"); !ok || v <= 0 { t.Fatalf("war_started expected conflict>0 delta=%v", we.Delta) }

  // Home should be actionable and mention at least one expected event type under conflict/betrayal context.
  hr, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=" + worldID)
  if err != nil { t.Fatalf("GET home: %v", err) }
  defer hr.Body.Close()
  hb, _ := io.ReadAll(hr.Body)
  hs := string(hb)
  if !strings.Contains(hs, "建议：") { t.Fatalf("expected hints in home, got=%s", hs) }
  if !(strings.Contains(hs, "betrayal") || strings.Contains(hs, "war_started")) {
    t.Fatalf("expected home hints mention betrayal/war_started, got=%s", hs)
  }
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run TestStoryRules_BetrayalAndWarStarted -v
```

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/intent_story_rules_betrayal_war_test.go
git commit -m "test(story): gate betrayal and war_started events"
```

---

## Task 2: 扩展 story 规则（GREEN）

**Files:**
- Modify: `internal/sim/intent_story_rules.go`

- [ ] **Step 1: 新增 betrayal/war_started 关键词规则（并调整优先级）**

在 `intentStorySpec(goal string)` 中最前面加入：
```go
if strings.Contains(g, "背叛") || strings.Contains(g, "翻脸") {
  return storyEventSpec{
    typ: "betrayal",
    narrative: "关系裂变：背叛发生（目标：" + g + "）",
    delta: map[string]any{"trust": int64(-10), "conflict": int64(8), "order": int64(-2)},
  }, true
}
if strings.Contains(g, "宣战") || strings.Contains(g, "开战") {
  return storyEventSpec{
    typ: "war_started",
    narrative: "战端开启：宣战（目标：" + g + "）",
    delta: map[string]any{"conflict": int64(10), "order": int64(-3), "trust": int64(-4)},
  }, true
}
```

并确保它们优先于 alliance/treaty/trade（函数顶部即可）。

- [ ] **Step 2: 跑测试转绿**
```bash
go test ./tests/integration -run TestStoryRules_BetrayalAndWarStarted -v
go test ./...
```

- [ ] **Step 3: Commit（规则实现）**
```bash
git add internal/sim/intent_story_rules.go
git commit -m "feat(story): add betrayal and war_started story rules"
```

---

## Task 3: spectator/home 文案增强（GREEN）

**Files:**
- Modify: `internal/gateway/world_summary.go`

- [ ] **Step 1: 在“战乱/背叛语境”下的建议里提到 betrayal/war_started**

建议做法：在 `secondaryHint()` 的“战乱/背叛/翻脸”分支，将文案从仅提 alliance/treaty 扩展为：
- 仍建议 “结盟/谈判/条约”
- 但补一句 “也可尝试背叛/宣战（betrayal/war_started）观察关系裂变与冲突升级”

确保包含字符串 `betrayal` 或 `war_started`（满足门禁）。

- [ ] **Step 2: 全量测试**
```bash
go test ./...
```

- [ ] **Step 3: Commit（文案增强）**
```bash
git add internal/gateway/world_summary.go
git commit -m "feat(spectator): mention betrayal/war_started in home hints"
```

---

## Task 4: 部署与验收（Render）

- [ ] **Step 1: push main**
```bash
git push origin main
```

- [ ] **Step 2: staging 快速验收（脚本）**
```bash
bash scripts/smoke_staging.sh
```
Expected: ALL OK

