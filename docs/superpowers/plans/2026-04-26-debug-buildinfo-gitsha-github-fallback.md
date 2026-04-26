# A2.2：GitHub API 兜底 debug/build 的 git_sha（Render）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 当 `/api/v0/debug/build` 的 `git_sha` 计算结果为 `"unknown"` 时，使用 GitHub 公共 API 获取 `<repo>/<branch>` 的最新 commit（取前 7 位）作为最终兜底；并带进程内 5 分钟缓存 + 2 秒超时，避免限流与慢请求。

**Architecture:** 在 gateway 包新增一个小型 `githubCommitResolver`（http client + TTL cache），由 `buildInfoSnapshot()` 在必要时调用。仓库与分支优先读取 `RENDER_GIT_REPO_SLUG`、`RENDER_GIT_BRANCH`，缺失时 fallback 到固定默认值。请求失败不影响接口：继续返回 `"unknown"`。

**Tech Stack:** Go（`net/http`、`encoding/json`、`time`、`sync`）、integration tests（httptest）。

---

## 0) Files（锁定）

**Create:**
- `internal/gateway/github_commit.go`（GitHub API 获取最新 commit + TTL cache）

**Modify:**
- `internal/gateway/buildinfo.go`（当 git_sha 为 unknown 时触发 resolver）
- `tests/integration/debug_buildinfo_test.go`（新增一条“不因 GitHub 不可用而失败”的测试门禁）

---

## Task 1: TDD — 增加可注入 resolver 的测试门禁（RED）

> 目标：我们需要在测试里“可控地模拟 GitHub API 成功/失败”，因此实现上必须支持注入 baseURL/client 或替换 resolver 的 fetch 函数。

**Files:**
- Modify: `tests/integration/debug_buildinfo_test.go`

- [ ] **Step 1: 增加一个测试：当 git_sha 为 unknown 且 resolver 返回 sha 时，应输出该 sha**

在测试文件中新增（示例，按最终注入方式调整）：

```go
func TestDebugBuild_GitHubFallback_WhenUnknown(t *testing.T) {
  t.Parallel()

  // 1) Start fake GitHub API server.
  gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // GitHub: /repos/<slug>/commits/<branch>
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{"sha":"0123456789abcdef0123456789abcdef01234567"}`))
  }))
  t.Cleanup(gh.Close)

  // 2) Force unknown path: clear buildGitSHA and ensure no buildvcs in unit env.
  // We rely on the implementation's "force unknown then fallback" mode via injected resolver.

  app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
  // Inject resolver baseURL = gh.URL (implementation required).
  app.OverrideGitHubAPIBaseURLForTest(gh.URL) // placeholder API, implement it.

  s := httptest.NewServer(app.Handler)
  t.Cleanup(s.Close)
  t.Cleanup(func() { app.Stop() })

  resp, err := http.Get(s.URL + "/api/v0/debug/build")
  if err != nil { t.Fatalf("GET: %v", err) }
  defer resp.Body.Close()

  var out struct {
    OK    bool                   `json:"ok"`
    Build map[string]interface{} `json:"build"`
  }
  _ = json.NewDecoder(resp.Body).Decode(&out)
  if out.Build["git_sha"] != "0123456" {
    t.Fatalf("expected sha7 from github fallback, got=%v", out.Build["git_sha"])
  }
}
```

- [ ] **Step 2: 运行确认失败（RED）**
```bash
go test ./tests/integration -run TestDebugBuild_GitHubFallback_WhenUnknown -v
```
Expected: FAIL（还没有注入点与 fallback 实现）

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/debug_buildinfo_test.go
git commit -m "test(debug): gate github fallback for git_sha"
```

---

## Task 2: 实现 GitHub resolver（GREEN）

**Files:**
- Create: `internal/gateway/github_commit.go`

- [ ] **Step 1: 新增 resolver（带 TTL cache）**

Create `internal/gateway/github_commit.go`：

```go
package gateway

import (
  "encoding/json"
  "fmt"
  "net/http"
  "strings"
  "sync"
  "time"
)

type githubCommitResolver struct {
  mu sync.Mutex
  cache map[string]cachedSHA

  ttl time.Duration
  client *http.Client
  baseURL string
}

type cachedSHA struct {
  sha7 string
  exp time.Time
}

func newGitHubCommitResolver() *githubCommitResolver {
  return &githubCommitResolver{
    cache: map[string]cachedSHA{},
    ttl: 5 * time.Minute,
    client: &http.Client{Timeout: 2 * time.Second},
    baseURL: "https://api.github.com",
  }
}

func (r *githubCommitResolver) LatestSHA7(repoSlug, branch string) (string, error) {
  repoSlug = strings.TrimSpace(repoSlug)
  branch = strings.TrimSpace(branch)
  if repoSlug == "" || branch == "" {
    return "", fmt.Errorf("empty repo or branch")
  }
  key := repoSlug + ":" + branch

  now := time.Now()
  r.mu.Lock()
  if v, ok := r.cache[key]; ok && now.Before(v.exp) && v.sha7 != "" {
    r.mu.Unlock()
    return v.sha7, nil
  }
  r.mu.Unlock()

  url := fmt.Sprintf("%s/repos/%s/commits/%s", strings.TrimRight(r.baseURL, "/"), repoSlug, branch)
  req, _ := http.NewRequest(http.MethodGet, url, nil)
  req.Header.Set("Accept", "application/vnd.github+json")
  req.Header.Set("User-Agent", "lobster-world-core")
  resp, err := r.client.Do(req)
  if err != nil {
    return "", err
  }
  defer resp.Body.Close()
  if resp.StatusCode != 200 {
    return "", fmt.Errorf("github status=%d", resp.StatusCode)
  }
  var body struct{ SHA string `json:"sha"` }
  if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
    return "", err
  }
  sha := strings.TrimSpace(body.SHA)
  if len(sha) < 7 {
    return "", fmt.Errorf("bad sha")
  }
  sha7 := sha[:7]

  r.mu.Lock()
  r.cache[key] = cachedSHA{sha7: sha7, exp: now.Add(r.ttl)}
  r.mu.Unlock()
  return sha7, nil
}
```

- [ ] **Step 2: 为测试提供注入点**

在 `githubCommitResolver` 上暴露仅测试用的 setter：
```go
func (r *githubCommitResolver) setBaseURLForTest(u string) { r.baseURL = u }
```
（或通过 Options 注入 resolver；以最少侵入为准。）

- [ ] **Step 3: Commit（resolver）**
```bash
git add internal/gateway/github_commit.go
git commit -m "feat(debug): add github sha resolver with ttl cache"
```

---

## Task 3: 接入 buildInfoSnapshot（GREEN）

**Files:**
- Modify: `internal/gateway/buildinfo.go`

- [ ] **Step 1: 增加 package-level resolver（懒加载）**

在 `buildinfo.go` 增加：
```go
var ghResolver = newGitHubCommitResolver()
```

并在 `buildInfoSnapshot()` 的兜底逻辑中：
- 若 `git_sha == "unknown"`（或缺失）：
  - repo := `os.Getenv("RENDER_GIT_REPO_SLUG")`（缺失则默认 `dengxilong2025/lobster-world-core`）
  - branch := `os.Getenv("RENDER_GIT_BRANCH")`（缺失则默认 `main`）
  - `sha7, err := ghResolver.LatestSHA7(repo, branch)`
  - 若成功：覆盖 `git_sha=sha7`

- [ ] **Step 2: 跑测试转绿**
```bash
go test ./tests/integration -run TestDebugBuild_GitHubFallback_WhenUnknown -v
go test ./...
```

- [ ] **Step 3: Commit（接入）**
```bash
git add internal/gateway/buildinfo.go
git commit -m "fix(debug): github fallback for git_sha when unknown"
```

---

## Task 4: 部署与线上验收（Render）

- [ ] **Step 1: push main 触发 Render 自动部署**
```bash
git push origin main
```

- [ ] **Step 2: staging 验收**
```bash
curl -sS "https://lobster-world-core.onrender.com/api/v0/debug/build" | head -c 500; echo
```
Expected: `build.git_sha` 为真实 7 位 SHA，而非 `"unknown"`（缓存刷新可能最多延迟 5 分钟）。

