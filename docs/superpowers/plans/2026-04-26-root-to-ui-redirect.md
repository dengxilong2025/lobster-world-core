# Root（/）重定向到 /ui Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 staging 根路径 `/` 访问 404：让 `GET /` 返回 302 跳转到 `/ui`，并保留 query string。

**Architecture:** 在 gateway 路由层新增根路径处理：对 `GET /` 做 `http.Redirect(..., "/ui"+query, http.StatusFound)`。不改现有 `/ui`、`/healthz`、`/api/v0/*`。加入集成测试锁定行为。

**Tech Stack:** Go（net/http）、现有 gateway router、go test 集成测试。

---

## 0) Files（锁定）

**Modify:**
- `internal/gateway/routes_ui.go`（或现有注册 `/ui` 的路由文件：新增 `/` handler）
- `tests/integration/ui_smoke_test.go`（新增 root redirect case）

---

## Task 1: TDD — 写失败的集成测试（RED）

- [ ] **Step 1: 增加测试用例**

在 `tests/integration/ui_smoke_test.go` 增加测试（示例，按现有 helper 调整）：

```go
func TestUI_RootRedirectsToUI(t *testing.T) {
    baseURL := mustStartTestServer(t)

    // 1) no query
    resp, err := http.Get(baseURL + "/")
    require.NoError(t, err)
    defer resp.Body.Close()
    require.Equal(t, http.StatusFound, resp.StatusCode)
    require.Equal(t, "/ui", resp.Header.Get("Location"))

    // 2) preserve query string
    req, err := http.NewRequest(http.MethodGet, baseURL+"/?world_id=w1&goal=hi", nil)
    require.NoError(t, err)
    resp2, err := http.DefaultClient.Do(req)
    require.NoError(t, err)
    defer resp2.Body.Close()
    require.Equal(t, http.StatusFound, resp2.StatusCode)
    require.Equal(t, "/ui?world_id=w1&goal=hi", resp2.Header.Get("Location"))
}
```

> 注：如果当前测试 client 会自动跟随 302，需要把 client 配成不跟随 redirect（查看现有测试写法）。

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run TestUI_RootRedirectsToUI -v
```
Expected: FAIL（当前 `/` 还是 404 或未设置 Location）。

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/ui_smoke_test.go
git commit -m "test(ui): gate root redirect to /ui"
```

---

## Task 2: 实现根路径重定向（GREEN）

- [ ] **Step 1: 增加 `/` handler**

在注册路由处新增：

```go
// GET / -> /ui (keep query)
router.GET("/", func(w http.ResponseWriter, r *http.Request) {
    target := "/ui"
    if r.URL.RawQuery != "" {
        target = target + "?" + r.URL.RawQuery
    }
    http.Redirect(w, r, target, http.StatusFound)
})
```

- [ ] **Step 2: 运行测试转绿**
```bash
go test ./tests/integration -run TestUI_RootRedirectsToUI -v
go test ./...
```
Expected: PASS

- [ ] **Step 3: Commit（实现）**
```bash
git add internal/gateway/routes_ui.go
git commit -m "fix(ui): redirect / to /ui"
```

---

## Task 3: 部署冒烟（Render）

- [ ] **Step 1: push main 触发 Render 自动部署**
```bash
git push origin main
```

- [ ] **Step 2: staging 验收**
```bash
curl -sS -I "https://lobster-world-core.onrender.com/" | head -n 10
curl -sS -I "https://lobster-world-core.onrender.com/ui" | head -n 10
```
Expected:
- `/` 返回 302，`Location: /ui...`
- `/ui` 返回 200

