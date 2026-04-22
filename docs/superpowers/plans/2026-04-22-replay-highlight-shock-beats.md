# replay/highlight 冲击期脚本化回放（beats 更一致）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 当 highlight 的目标事件处于某个 shock 生命周期内（或本身是 shock_* 事件）时，输出更稳定的 beats 结构：优先按 `shock_warning → shock_started →（betrayal）→ shock_ended` 组织，并保留少量世界阶段信息与固定收尾。

**Architecture:** 在 `buildReplayBeats` 内检测 `shock_key`（来自 target.Meta 或 1 层 trace cause），如果命中则扫描最近 N 条事件并抽取同 shock_key 的 lifecycle 事件，按固定时间轴生成 beats；否则保持现有逻辑不变。测试优先用 integration：启动 app + ShockConfig，抓一个 shock_started 事件当 target 调用 `/replay/highlight` 并断言固定结构 beats 存在。

**Tech Stack:** Go（gateway + sim），`testing`、`net/http/httptest`。

---

## 0) Files 结构与改动范围（先锁定）

**修改：**
- `internal/gateway/routes_replay.go`：增强 `buildReplayBeats`（shock 模式）
- `tests/integration/replay_highlight_shock_beats_test.go`：新增集成测试门禁
- （可选）`docs/roadmap.md`：阶段 3 中 “冲击期脚本化回放” 标记完成

---

## Task 1: 写 failing integration test（RED）

**Files:**
- Create: `tests/integration/replay_highlight_shock_beats_test.go`

- [ ] **Step 1: 新增测试：shock_started 目标应产出 lifecycle beats**

```go
package integration

import (
  "bufio"
  "bytes"
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "testing"
  "time"

  "lobster-world-core/internal/gateway"
  "lobster-world-core/internal/sim"
)

func TestReplayHighlight_ShockLifecycleBeatsAreStable(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{
    TickInterval: 10 * time.Millisecond,
    Seed:         123,
    Shock: &sim.ShockConfig{
      EpochTicks:    6,
      WarningOffset: 1,
      DurationTicks: 2,
      CooldownTicks: 6,
      Candidates: []sim.ShockCandidate{
        {Key: "riftwinter", Weight: 1, WarningNarrative: "天象异常：裂冬指数上升", StartedNarrative: "冲击开始：裂冬纪元降临", EndedNarrative: "冲击结束：裂冬余波仍在", ActorsPool: []string{"nation_a", "nation_b", "nation_c"}},
      },
    },
  })
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  worldID := "w_highlight_shock"
  body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
  r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
  if err != nil {
    t.Fatalf("POST /intents: %v", err)
  }
  r.Body.Close()

  // Wait: warning/start/end should happen in small-tick schedule.
  time.Sleep(250 * time.Millisecond)

  // Export and pick a shock_started event id.
  resp, err := http.Get(s.URL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
  if err != nil {
    t.Fatalf("GET export: %v", err)
  }
  defer resp.Body.Close()

  shockStartedID := ""
  sc := bufio.NewScanner(resp.Body)
  for sc.Scan() {
    line := sc.Bytes()
    if len(bytes.TrimSpace(line)) == 0 {
      continue
    }
    var meta map[string]any
    _ = json.Unmarshal(line, &meta)
    if meta["type"] == "shock_started" {
      if id, ok := meta["event_id"].(string); ok && id != "" {
        shockStartedID = id
        break
      }
    }
  }
  if shockStartedID == "" {
    t.Fatalf("expected to find shock_started in export")
  }

  // Call highlight.
  hr, err := http.Get(s.URL + "/api/v0/replay/highlight?world_id=" + worldID + "&event_id=" + shockStartedID)
  if err != nil {
    t.Fatalf("GET highlight: %v", err)
  }
  defer hr.Body.Close()
  if hr.StatusCode != http.StatusOK {
    t.Fatalf("expected 200, got %d", hr.StatusCode)
  }
  var out struct {
    Beats []struct {
      T       int    `json:"t"`
      Caption string `json:"caption"`
    } `json:"beats"`
  }
  if err := json.NewDecoder(hr.Body).Decode(&out); err != nil {
    t.Fatalf("decode: %v", err)
  }

  hasOpener := false
  hasWarn := false
  hasStart := false
  hasEnd := false
  hasEnding := false
  for _, b := range out.Beats {
    if b.T == 0 && b.Caption != "" {
      hasOpener = true
    }
    if bytes.Contains([]byte(b.Caption), []byte("冲击预警")) || bytes.Contains([]byte(b.Caption), []byte("天象异常")) {
      hasWarn = true
    }
    if bytes.Contains([]byte(b.Caption), []byte("冲击开始")) || bytes.Contains([]byte(b.Caption), []byte("裂冬纪元")) {
      hasStart = true
    }
    if bytes.Contains([]byte(b.Caption), []byte("冲击结束")) || bytes.Contains([]byte(b.Caption), []byte("裂冬余波")) {
      hasEnd = true
    }
    if bytes.Contains([]byte(b.Caption), []byte("下一步：关注冲击/背叛/迁徙窗口")) {
      hasEnding = true
    }
  }
  if !hasOpener || !hasStart || !hasEnd || !hasEnding {
    t.Fatalf("expected opener/start/end/ending beats, got=%#v", out.Beats)
  }
  // warning might be missed if scheduler config changes; keep it soft but preferred.
  _ = hasWarn
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**

Run:
```bash
go test ./... -run TestReplayHighlight_ShockLifecycleBeatsAreStable -v
```

Expected:
- FAIL（当前 beats 未必包含“冲击预警/开始/结束”的固定结构）

- [ ] **Step 3: Commit（仅测试）**

```bash
git add tests/integration/replay_highlight_shock_beats_test.go
git commit -m "test(replay): require stable shock lifecycle beats in highlight"
```

---

## Task 2: 最小实现 shock 模式 beats（GREEN）

**Files:**
- Modify: `internal/gateway/routes_replay.go`

- [ ] **Step 1: 新增 shockKey 提取函数（局部 helper）**

在 `routes_replay.go` 中添加 helper（保持文件内私有）：
- `func shockKeyFromEvent(e spec.Event) (string, bool)`：从 `e.Meta["shock_key"]` 取字符串
- `func resolveShockKeyFromTrace(es store.EventStore, worldID string, e spec.Event) (string, bool)`：回溯 1 层 trace cause event 获取 shock_key

- [ ] **Step 2: 新增 lifecycle 抽取函数（扫描最近 N 条事件）**

新增 helper：
- `func findShockLifecycle(worldID string, es store.EventStore, shockKey string, targetTs int64) (warn, start, betrayal, end *spec.Event)`

策略：
- `es.Query({WorldID: worldID, SinceTs: 0, Limit: 2000})`
- 过滤 `Meta["shock_key"] == shockKey`
- 对每种 type 选取“最接近 targetTs 的那条”（或按 ts 最接近的那条；简单即可）

- [ ] **Step 3: 在 buildReplayBeats 内启用 shock 模式 beats**

逻辑：
- 若 `target` 或 trace resolve 命中 shockKey：构建 beats：
  - t=0 opener（target）
  - t=2 warn（若有）
  - t=6 started（若有）
  - t=12 betrayal（若有）
  - t=18 ended（若有）
  - t=28 ending（固定句）
- 同时保留 world summary 的 `世界阶段` + `近期` 两条（放在 t=3/t=4），不再塞入过多随机 bullets（保持结构稳定）
- 最后仍按现有排序稳定输出

- [ ] **Step 4: 运行测试确认通过（GREEN）**

Run:
```bash
go test ./... -run TestReplayHighlight_ShockLifecycleBeatsAreStable -v
go test ./...
```

Expected:
- 全绿

- [ ] **Step 5: Commit（实现）**

```bash
git add internal/gateway/routes_replay.go
git commit -m "feat(replay): script highlight beats around shock lifecycle"
```

---

## Task 3: 文档同步与交付

**Files:**
- Modify: `docs/roadmap.md`（可选）

- [ ] **Step 1: 更新 roadmap（阶段 3）**

将：
- `更稳定的“冲击期脚本化回放”（beats 结构更一致）`
标记为完成（或拆分为子项并标记 shock 模式完成）。

- [ ] **Step 2: Commit（文档）**
```bash
git add docs/roadmap.md
git commit -m "docs: mark shock scripted replay beats as done"
```

- [ ] **Step 3: 打补丁包**
```bash
git format-patch -3 -o /workspace/patches_replay_shock_beats
cd /workspace && zip -qr patches_replay_shock_beats.zip patches_replay_shock_beats
```

