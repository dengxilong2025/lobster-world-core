# A2：/api/v0/debug/build（版本可见性）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `GET /api/v0/debug/build` 端点，返回 build info（vcs.revision/vcs.time/vcs.modified + go/module）以及 start_time/uptime_sec，便于确认线上部署版本与是否重启。

**Architecture:** 在 gateway 包内增加进程启动时间 `startTime`（package-level，只初始化一次）；debug handler 使用 `runtime/debug.ReadBuildInfo()` 解析 settings；返回稳定 JSON。补一个 integration test 做字段存在性门禁。

**Tech Stack:** Go（`net/http`、`runtime/debug`、`time`）、httptest 集成测试。

---

## 0) Files（锁定）

**Modify:**
- `internal/gateway/routes_debug.go`（新增 `/api/v0/debug/build` handler）

**Create:**
- `internal/gateway/buildinfo.go`（封装 buildinfo 读取与 start/uptime 组装）
- `tests/integration/debug_buildinfo_test.go`（门禁测试）

---

## Task 1: TDD — 写 failing 集成测试（RED）

**Files:**
- Create: `tests/integration/debug_buildinfo_test.go`

- [ ] **Step 1: 写测试（先红）**

```go
package integration

import (
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "testing"
  "time"

  "lobster-world-core/internal/gateway"
)

func TestDebugBuild_ReturnsBuildInfo(t *testing.T) {
  t.Parallel()

  app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  resp, err := http.Get(s.URL + "/api/v0/debug/build")
  if err != nil {
    t.Fatalf("GET debug/build: %v", err)
  }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK {
    t.Fatalf("expected 200, got %d", resp.StatusCode)
  }

  var out struct {
    OK    bool                   `json:"ok"`
    Build map[string]interface{} `json:"build"`
  }
  if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
    t.Fatalf("decode: %v", err)
  }
  if !out.OK {
    t.Fatalf("expected ok=true")
  }
  if out.Build == nil {
    t.Fatalf("expected build object")
  }
  // Required fields
  if _, ok := out.Build["start_time"]; !ok {
    t.Fatalf("missing start_time")
  }
  if _, ok := out.Build["uptime_sec"]; !ok {
    t.Fatalf("missing uptime_sec")
  }
  // Optional but useful (may be empty depending on build flags)
  if v, ok := out.Build["git_sha"]; ok {
    if s, _ := v.(string); ok && s == "" {
      t.Fatalf("git_sha present but empty")
    }
  }
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run TestDebugBuild_ReturnsBuildInfo -v
```
Expected: FAIL（404 或路由不存在）

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/debug_buildinfo_test.go
git commit -m "test(debug): gate /debug/build endpoint"
```

---

## Task 2: 实现 buildinfo 组装（GREEN）

**Files:**
- Create: `internal/gateway/buildinfo.go`
- Modify: `internal/gateway/routes_debug.go`

- [ ] **Step 1: 新增 buildinfo helper**

Create `internal/gateway/buildinfo.go`：
```go
package gateway

import (
  "runtime/debug"
  "strings"
  "time"
)

var startTime = time.Now()

func buildInfoSnapshot() map[string]any {
  out := map[string]any{
    "start_time": startTime.UTC().Format(time.RFC3339),
    "uptime_sec": int64(time.Since(startTime).Seconds()),
  }

  // Go build metadata (best-effort).
  if bi, ok := debug.ReadBuildInfo(); ok && bi != nil {
    out["go_version"] = bi.GoVersion
    if bi.Main.Path != "" {
      // Prefer last path segment for readability.
      parts := strings.Split(bi.Main.Path, "/")
      out["module"] = parts[len(parts)-1]
    }
    if bi.Main.Version != "" {
      out["module_version"] = bi.Main.Version
    }
    for _, s := range bi.Settings {
      switch s.Key {
      case "vcs.revision":
        out["git_sha"] = s.Value
      case "vcs.time":
        out["vcs_time"] = s.Value
      case "vcs.modified":
        out["vcs_modified"] = (s.Value == "true")
      }
    }
  }
  return out
}
```

- [ ] **Step 2: 注册路由**

在 `internal/gateway/routes_debug.go` 内增加：
```go
mux.HandleFunc("GET /api/v0/debug/build", func(w http.ResponseWriter, r *http.Request) {
  writeJSON(w, http.StatusOK, map[string]any{
    "ok":    true,
    "build": buildInfoSnapshot(),
  })
})
```

- [ ] **Step 3: 跑测试转绿**
```bash
go test ./tests/integration -run TestDebugBuild_ReturnsBuildInfo -v
go test ./...
```
Expected: PASS

- [ ] **Step 4: Commit（实现）**
```bash
git add internal/gateway/buildinfo.go internal/gateway/routes_debug.go
git commit -m "feat(debug): add /debug/build buildinfo endpoint"
```

---

## Task 3: 部署验证（Render）

- [ ] **Step 1: push main 触发 Render 自动部署**
```bash
git push origin main
```

- [ ] **Step 2: staging 验收**
```bash
curl -sS "https://lobster-world-core.onrender.com/api/v0/debug/build" | head -c 500; echo
```
Expected: JSON，包含 `start_time`/`uptime_sec`，若 buildvcs 可用则含 `git_sha`。

